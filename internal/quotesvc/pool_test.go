package quotesvc

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

type fakeConn struct {
	id      int
	closed  bool
	hbErr   error
	hbCalls int
}

func (c *fakeConn) Fetch(requests []tdx.QuoteRequest) ([]tdx.Quote, error) {
	out := make([]tdx.Quote, 0, len(requests))
	for _, r := range requests {
		out = append(out, tdx.Quote{Market: r.Market, Symbol: r.Symbol})
	}
	return out, nil
}

func (c *fakeConn) Heartbeat() error { c.hbCalls++; return c.hbErr }

func (c *fakeConn) Close() error { c.closed = true; return nil }

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// dialFactory returns a Dialer that hands out sequential fakeConns and records them.
func dialFactory() (Dialer, *[]*fakeConn, *int) {
	var conns []*fakeConn
	dials := 0
	dialer := func(ctx context.Context, server string) (Conn, error) {
		dials++
		c := &fakeConn{id: dials}
		conns = append(conns, c)
		return c, nil
	}
	return dialer, &conns, &dials
}

func TestPoolReusesHealthyConnection(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	dialer, _, dials := dialFactory()
	p := NewPool("s1", dialer, PoolConfig{MaxConns: 2}, clk.Now)

	pc1, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	p.Release(pc1, true)
	pc2, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire2: %v", err)
	}
	if pc1.conn != pc2.conn {
		t.Fatalf("expected reuse of same connection")
	}
	if *dials != 1 {
		t.Fatalf("expected 1 dial, got %d", *dials)
	}
}

func TestPoolBoundsMaxConns(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	dialer, _, _ := dialFactory()
	p := NewPool("s1", dialer, PoolConfig{MaxConns: 1}, clk.Now)

	if _, err := p.Acquire(context.Background()); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	_, err := p.Acquire(context.Background()) // not released, at limit
	if !errors.Is(err, ErrPoolExhausted) {
		t.Fatalf("expected ErrPoolExhausted, got %v", err)
	}
}

func TestPoolReplacesFailedHeartbeat(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	dialer, conns, dials := dialFactory()
	p := NewPool("s1", dialer, PoolConfig{MaxConns: 2, HeartbeatInterval: 10 * time.Second}, clk.Now)

	pc1, _ := p.Acquire(context.Background())
	p.Release(pc1, true)

	// Make the pooled connection fail its next heartbeat, and age it past the
	// heartbeat interval so a heartbeat is required before reuse.
	(*conns)[0].hbErr = errors.New("dead socket")
	clk.advance(11 * time.Second)

	pc2, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire2: %v", err)
	}
	if pc2.conn == pc1.conn {
		t.Fatalf("expected a replacement connection after heartbeat failure")
	}
	if !(*conns)[0].closed {
		t.Fatalf("expected failed connection to be closed")
	}
	if *dials != 2 {
		t.Fatalf("expected a redial, got %d dials", *dials)
	}
	if got := p.Stats().HeartbeatFailures; got != 1 {
		t.Fatalf("expected 1 heartbeat failure, got %d", got)
	}
}

func TestPoolRetiresIdleExpiredConnection(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	dialer, conns, dials := dialFactory()
	p := NewPool("s1", dialer, PoolConfig{MaxConns: 2, IdleTimeout: 30 * time.Second}, clk.Now)

	pc1, _ := p.Acquire(context.Background())
	p.Release(pc1, true)
	clk.advance(31 * time.Second)

	pc2, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire2: %v", err)
	}
	if pc2.conn == pc1.conn {
		t.Fatalf("expected idle-expired connection to be retired")
	}
	if !(*conns)[0].closed {
		t.Fatalf("expected idle-expired connection to be closed")
	}
	if *dials != 2 {
		t.Fatalf("expected a redial, got %d dials", *dials)
	}
}

func TestPoolMaxAgeReconnect(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	dialer, conns, dials := dialFactory()
	p := NewPool("s1", dialer, PoolConfig{MaxConns: 2, MaxConnAge: time.Minute}, clk.Now)

	pc1, _ := p.Acquire(context.Background())
	clk.advance(61 * time.Second)
	// Releasing a too-old connection must not return it to the idle set.
	p.Release(pc1, true)
	if !(*conns)[0].closed {
		t.Fatalf("expected too-old connection to be closed on release")
	}
	pc2, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire2: %v", err)
	}
	if pc2.conn == pc1.conn {
		t.Fatalf("expected fresh connection after max age")
	}
	if *dials != 2 {
		t.Fatalf("expected a redial, got %d dials", *dials)
	}
}
