package quotesvc

import (
	"context"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/config"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

// Service is the long-running realtime quote service: managed connection pools,
// rate-limited sweeps, and durable progress recording. Status is read back from
// the ops plane through the same Store, not through the infinity querier.
type Service struct {
	cfg      config.QuoteServiceConfig
	pools    *Pools
	exec     *Executor
	discover Discoverer
	servers  []string
	started  time.Time
	now      func() time.Time
}

// Health is a point-in-time view of the running service.
type Health struct {
	State               string      `json:"state"`
	Servers             []string    `json:"servers"`
	HealthyServers      int         `json:"healthy_servers"`
	Pools               []PoolStats `json:"pools"`
	GlobalLimiterTokens float64     `json:"global_limiter_tokens"`
	LastSuccessfulQuote *time.Time  `json:"last_successful_quote,omitempty"`
	UptimeSeconds       int64       `json:"uptime_seconds"`
}

// NewService builds a service from configuration. now may be nil. The discoverer
// is optional; when nil, discovery-based sweeps return an error.
func NewService(cfg config.QuoteServiceConfig, store StateStore, discover Discoverer, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	servers := cfg.Servers
	if len(servers) == 0 {
		servers = append([]string(nil), tdx.DefaultHQServers...)
	}
	poolCfg := PoolConfig{
		MaxConns:          cfg.MaxConnsPerServer,
		IdleTimeout:       cfg.IdleTimeout.Duration(),
		MaxConnAge:        cfg.MaxConnAge.Duration(),
		HeartbeatInterval: cfg.HeartbeatInterval.Duration(),
	}
	pools := NewPools(servers, SessionDialer(cfg.DialTimeout.Duration()), poolCfg, now)
	exec := NewExecutor(pools, store, ExecutorConfig{
		Concurrency:      cfg.BatchConcurrency,
		RetryBudget:      cfg.RetryBudget,
		BackoffBase:      cfg.BackoffBase.Duration(),
		BackoffMax:       cfg.BackoffMax.Duration(),
		GlobalRatePerSec: cfg.GlobalRatePerSec,
		PerServerRate:    cfg.PerServerRate,
		Burst:            cfg.Burst,
	}, servers, now, nil)

	if discover == nil {
		discover = NewTDXDiscoverer(tdx.QuoteClientOptions{
			Servers: servers,
			Timeout: cfg.DialTimeout.Duration(),
		})
	}
	return &Service{
		cfg:      cfg,
		pools:    pools,
		exec:     exec,
		discover: discover,
		servers:  servers,
		started:  now(),
		now:      now,
	}
}

// Plan resolves a sweep plan using the service's configured markets/batch size.
func (s *Service) Plan(ctx context.Context, requests []tdx.QuoteRequest, markets []string, limit int) (SweepPlan, error) {
	if len(markets) == 0 {
		markets = s.cfg.Markets
	}
	return PlanSweep(ctx, PlanOptions{
		Markets:   markets,
		Requests:  requests,
		BatchSize: s.cfg.BatchSize,
		Limit:     limit,
	}, s.discover)
}

// RunSweep executes (or resumes) a sweep.
func (s *Service) RunSweep(ctx context.Context, opts RunOptions) (model.QuoteServiceRun, error) {
	return s.exec.Run(ctx, opts)
}

// Health returns the current service health snapshot.
func (s *Service) Health() Health {
	h := Health{
		State:               "running",
		Servers:             s.servers,
		HealthyServers:      s.pools.HealthyCount(),
		Pools:               s.pools.Stats(),
		GlobalLimiterTokens: s.exec.GlobalLimiterTokens(),
		UptimeSeconds:       int64(s.now().Sub(s.started) / time.Second),
	}
	if ts, ok := s.exec.LastSuccessfulQuote(); ok {
		h.LastSuccessfulQuote = &ts
	}
	return h
}

// Close releases all pooled connections.
func (s *Service) Close() {
	s.pools.Close()
}

// NewTDXDiscoverer returns a Discoverer backed by the TDX online security list.
func NewTDXDiscoverer(opts tdx.QuoteClientOptions) Discoverer {
	return func(ctx context.Context, market string) ([]tdx.QuoteRequest, error) {
		securities, err := tdx.FetchSecurityList(ctx, market, opts)
		if err != nil {
			return nil, err
		}
		out := make([]tdx.QuoteRequest, 0, len(securities))
		for _, sec := range securities {
			out = append(out, tdx.QuoteRequest{Market: sec.Market, Symbol: sec.Symbol})
		}
		return out, nil
	}
}
