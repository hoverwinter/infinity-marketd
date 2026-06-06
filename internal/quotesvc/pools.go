package quotesvc

import (
	"sync"
	"time"
)

// Pools is a registry of per-server connection pools with round-robin ordering
// and coarse health tracking, so sweeps can fall back across servers.
type Pools struct {
	dial Dialer
	cfg  PoolConfig
	now  func() time.Time

	mu      sync.Mutex
	servers []string
	pools   map[string]*Pool
	healthy map[string]bool
	rr      int
}

// NewPools builds a pool per server. now may be nil (defaults to time.Now).
func NewPools(servers []string, dial Dialer, cfg PoolConfig, now func() time.Time) *Pools {
	if now == nil {
		now = time.Now
	}
	ps := &Pools{
		dial:    dial,
		cfg:     cfg,
		now:     now,
		pools:   make(map[string]*Pool, len(servers)),
		healthy: make(map[string]bool, len(servers)),
	}
	for _, s := range servers {
		if _, ok := ps.pools[s]; ok {
			continue
		}
		ps.servers = append(ps.servers, s)
		ps.pools[s] = NewPool(s, dial, cfg, now)
		ps.healthy[s] = true
	}
	return ps
}

// order returns the configured servers rotated round-robin so successive sweeps
// spread load rather than always hammering the first server.
func (ps *Pools) order() []string {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	n := len(ps.servers)
	if n == 0 {
		return nil
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, ps.servers[(ps.rr+i)%n])
	}
	ps.rr = (ps.rr + 1) % n
	return out
}

func (ps *Pools) pool(server string) *Pool {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.pools[server]
}

func (ps *Pools) markHealthy(server string, ok bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.healthy[server] = ok
}

// HealthyCount reports how many servers are currently marked healthy.
func (ps *Pools) HealthyCount() int {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	n := 0
	for _, ok := range ps.healthy {
		if ok {
			n++
		}
	}
	return n
}

// Stats returns a snapshot of every pool, in configured order.
func (ps *Pools) Stats() []PoolStats {
	ps.mu.Lock()
	servers := append([]string(nil), ps.servers...)
	pools := make([]*Pool, 0, len(servers))
	for _, s := range servers {
		pools = append(pools, ps.pools[s])
	}
	ps.mu.Unlock()
	out := make([]PoolStats, 0, len(pools))
	for _, p := range pools {
		out = append(out, p.Stats())
	}
	return out
}

// Close closes every pool.
func (ps *Pools) Close() {
	ps.mu.Lock()
	pools := make([]*Pool, 0, len(ps.pools))
	for _, p := range ps.pools {
		pools = append(pools, p)
	}
	ps.mu.Unlock()
	for _, p := range pools {
		p.Close()
	}
}
