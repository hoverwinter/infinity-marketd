// Package quotesvc operationalizes TDX realtime quote collection: bounded
// connection pools, rate limiting, batch sweep scheduling, failure recovery,
// and resume. Protocol construction/decoding stays in internal/tdx; this
// package owns lifecycle and policy only and never opens ClickHouse itself.
package quotesvc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

// Conn is the minimal managed TDX HQ connection. *tdx.QuoteSession satisfies it
// via SessionDialer; tests supply fakes.
type Conn interface {
	Fetch(requests []tdx.QuoteRequest) ([]tdx.Quote, error)
	Heartbeat() error
	Close() error
}

// Dialer opens a new connection to a server.
type Dialer func(ctx context.Context, server string) (Conn, error)

// SessionDialer returns a Dialer backed by real tdx.QuoteSession connections.
func SessionDialer(timeout time.Duration) Dialer {
	return func(ctx context.Context, server string) (Conn, error) {
		return tdx.OpenQuoteSession(ctx, server, timeout)
	}
}

// PoolConfig configures connection lifecycle for one server pool.
type PoolConfig struct {
	MaxConns          int           // max open sockets (idle + checked out)
	IdleTimeout       time.Duration // retire idle conns older than this on acquire
	MaxConnAge        time.Duration // retire conns older than this (periodic reconnect)
	HeartbeatInterval time.Duration // heartbeat before reuse if idle longer than this
}

// ErrPoolExhausted is returned when no connection is available within MaxConns.
var ErrPoolExhausted = errors.New("connection pool exhausted")

// ErrPoolClosed is returned when acquiring from a closed pool.
var ErrPoolClosed = errors.New("connection pool closed")

type pooledConn struct {
	conn       Conn
	createdAt  time.Time
	lastUsedAt time.Time
}

// Pool is a bounded set of reusable connections to a single TDX HQ server.
type Pool struct {
	server string
	dial   Dialer
	cfg    PoolConfig
	now    func() time.Time

	mu                sync.Mutex
	idle              []*pooledConn
	open              int // idle + checked out
	heartbeatFailures int64
	closed            bool
}

// PoolStats is a point-in-time snapshot of a pool.
type PoolStats struct {
	Server            string `json:"server"`
	Open              int    `json:"open"`
	Idle              int    `json:"idle"`
	HeartbeatFailures int64  `json:"heartbeat_failures"`
}

// NewPool creates a pool for server. now may be nil (defaults to time.Now).
func NewPool(server string, dial Dialer, cfg PoolConfig, now func() time.Time) *Pool {
	if now == nil {
		now = time.Now
	}
	if cfg.MaxConns <= 0 {
		cfg.MaxConns = 1
	}
	return &Pool{server: server, dial: dial, cfg: cfg, now: now}
}

func (p *Pool) tooOld(pc *pooledConn, now time.Time) bool {
	return p.cfg.MaxConnAge > 0 && now.Sub(pc.createdAt) >= p.cfg.MaxConnAge
}

func (p *Pool) idleExpired(pc *pooledConn, now time.Time) bool {
	return p.cfg.IdleTimeout > 0 && now.Sub(pc.lastUsedAt) >= p.cfg.IdleTimeout
}

// Acquire returns a usable connection, reusing a healthy idle one or dialing a
// new one within MaxConns. Stale connections are retired; if an idle connection
// fails its heartbeat it is closed and another is tried.
func (p *Pool) Acquire(ctx context.Context) (*pooledConn, error) {
	for {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return nil, ErrPoolClosed
		}
		var cand *pooledConn
		for len(p.idle) > 0 {
			pc := p.idle[len(p.idle)-1]
			p.idle = p.idle[:len(p.idle)-1]
			now := p.now()
			if p.tooOld(pc, now) || p.idleExpired(pc, now) {
				pc.conn.Close()
				p.open--
				continue
			}
			cand = pc
			break
		}
		if cand != nil {
			needHeartbeat := p.cfg.HeartbeatInterval > 0 && p.now().Sub(cand.lastUsedAt) >= p.cfg.HeartbeatInterval
			p.mu.Unlock()
			if needHeartbeat {
				if err := cand.conn.Heartbeat(); err != nil {
					p.mu.Lock()
					cand.conn.Close()
					p.open--
					p.heartbeatFailures++
					p.mu.Unlock()
					continue // retire and try the next idle / dial
				}
			}
			return cand, nil
		}
		if p.open >= p.cfg.MaxConns {
			p.mu.Unlock()
			return nil, ErrPoolExhausted
		}
		p.open++ // reserve a slot before the (slow) dial
		p.mu.Unlock()

		conn, err := p.dial(ctx, p.server)
		if err != nil {
			p.mu.Lock()
			p.open--
			p.mu.Unlock()
			return nil, fmt.Errorf("dial %s: %w", p.server, err)
		}
		now := p.now()
		return &pooledConn{conn: conn, createdAt: now, lastUsedAt: now}, nil
	}
}

// Release returns a connection to the pool. When healthy is false (or the pool
// is closed or the connection is too old) it is closed instead of pooled.
func (p *Pool) Release(pc *pooledConn, healthy bool) {
	if pc == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || !healthy || p.tooOld(pc, p.now()) {
		pc.conn.Close()
		p.open--
		return
	}
	pc.lastUsedAt = p.now()
	p.idle = append(p.idle, pc)
}

// Close closes all idle connections and prevents further reuse. Connections
// currently checked out are closed when released.
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true
	for _, pc := range p.idle {
		pc.conn.Close()
		p.open--
	}
	p.idle = nil
}

// Stats returns a snapshot of pool state.
func (p *Pool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return PoolStats{
		Server:            p.server,
		Open:              p.open,
		Idle:              len(p.idle),
		HeartbeatFailures: p.heartbeatFailures,
	}
}
