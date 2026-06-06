package quotesvc

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

// memStore is an in-memory StateStore for tests.
type memStore struct {
	mu      sync.Mutex
	runs    map[string]model.QuoteServiceRun
	batches map[string]map[uint32]model.QuoteServiceBatch
}

func newMemStore() *memStore {
	return &memStore{runs: map[string]model.QuoteServiceRun{}, batches: map[string]map[uint32]model.QuoteServiceBatch{}}
}

func (m *memStore) SaveQuoteServiceRun(ctx context.Context, run model.QuoteServiceRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs[run.RunID] = run
	return nil
}

func (m *memStore) SaveQuoteServiceBatch(ctx context.Context, b model.QuoteServiceBatch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.batches[b.RunID] == nil {
		m.batches[b.RunID] = map[uint32]model.QuoteServiceBatch{}
	}
	m.batches[b.RunID][b.BatchNo] = b
	return nil
}

func (m *memStore) LoadQuoteServiceRun(ctx context.Context, runID string) (model.QuoteServiceRun, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[runID]
	return run, ok, nil
}

func (m *memStore) LoadQuoteServiceBatches(ctx context.Context, runID string) ([]model.QuoteServiceBatch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []model.QuoteServiceBatch
	for _, b := range m.batches[runID] {
		out = append(out, b)
	}
	return out, nil
}

// scriptConn drives Fetch through a shared closure.
type scriptConn struct {
	fetch func(reqs []tdx.QuoteRequest) ([]tdx.Quote, error)
}

func (c *scriptConn) Fetch(reqs []tdx.QuoteRequest) ([]tdx.Quote, error) { return c.fetch(reqs) }
func (c *scriptConn) Heartbeat() error                                   { return nil }
func (c *scriptConn) Close() error                                       { return nil }

func noopSleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// newTestExecutor builds an executor over a single server whose Fetch is fetch.
func newTestExecutor(t *testing.T, store StateStore, cfg ExecutorConfig, fetch func([]tdx.QuoteRequest) ([]tdx.Quote, error)) *Executor {
	t.Helper()
	servers := []string{"s1"}
	dialer := func(ctx context.Context, server string) (Conn, error) {
		return &scriptConn{fetch: fetch}, nil
	}
	pools := NewPools(servers, dialer, PoolConfig{MaxConns: 8}, nil)
	e := NewExecutor(pools, store, cfg, servers, nil, noopSleep)
	e.randFloat = func() float64 { return 0 } // deterministic backoff
	return e
}

func okFetch(reqs []tdx.QuoteRequest) ([]tdx.Quote, error) {
	out := make([]tdx.Quote, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, tdx.Quote{Market: r.Market, Symbol: r.Symbol})
	}
	return out, nil
}

func plan(symbols ...string) SweepPlan {
	p, _ := PlanSweep(context.Background(), PlanOptions{Requests: reqs("sh", symbols...), BatchSize: 1}, nil)
	return p
}

func TestExecutorAllSucceed(t *testing.T) {
	store := newMemStore()
	e := newTestExecutor(t, store, ExecutorConfig{Concurrency: 2}, okFetch)
	run, err := e.Run(context.Background(), RunOptions{RunID: "r1", Plan: plan("600000", "600001", "600002")})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if run.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", run.Status)
	}
	if run.SucceededBatches != 3 || run.FailedBatches != 0 {
		t.Fatalf("unexpected counters: %+v", run)
	}
	if run.RowsFetched != 3 {
		t.Fatalf("expected 3 rows, got %d", run.RowsFetched)
	}
}

func TestExecutorRetriesRecoverable(t *testing.T) {
	store := newMemStore()
	var calls int32
	fetch := func(reqs []tdx.QuoteRequest) ([]tdx.Quote, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			return nil, errors.New("connection reset by peer") // transport, recoverable
		}
		return okFetch(reqs)
	}
	e := newTestExecutor(t, store, ExecutorConfig{Concurrency: 1, RetryBudget: 2}, fetch)
	run, err := e.Run(context.Background(), RunOptions{RunID: "r1", Plan: plan("600000")})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if run.Status != StatusCompleted || run.SucceededBatches != 1 {
		t.Fatalf("expected success after retry, got %+v", run)
	}
	b := store.batches["r1"][1]
	if b.Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", b.Attempts)
	}
}

func TestExecutorPreservesDecodeFailure(t *testing.T) {
	store := newMemStore()
	var calls int32
	fetch := func(reqs []tdx.QuoteRequest) ([]tdx.Quote, error) {
		atomic.AddInt32(&calls, 1)
		return nil, errors.New("decode TDX HQ quote response from s1: bad payload")
	}
	e := newTestExecutor(t, store, ExecutorConfig{Concurrency: 1, RetryBudget: 3}, fetch)
	run, err := e.Run(context.Background(), RunOptions{RunID: "r1", Plan: plan("600000")})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if run.Status != StatusWithFailures || run.FailedBatches != 1 {
		t.Fatalf("expected failure, got %+v", run)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("decode failure must not be retried, got %d calls", got)
	}
	b := store.batches["r1"][1]
	if b.FailureKind != string(FailureDecode) {
		t.Fatalf("expected decode failure kind, got %q", b.FailureKind)
	}
}

func TestExecutorFailureIsolation(t *testing.T) {
	store := newMemStore()
	fetch := func(reqs []tdx.QuoteRequest) ([]tdx.Quote, error) {
		if reqs[0].Symbol == "600001" {
			return nil, errors.New("boom transport")
		}
		return okFetch(reqs)
	}
	e := newTestExecutor(t, store, ExecutorConfig{Concurrency: 1, RetryBudget: 0}, fetch)
	run, err := e.Run(context.Background(), RunOptions{RunID: "r1", Plan: plan("600000", "600001", "600002")})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if run.Status != StatusWithFailures {
		t.Fatalf("expected completed_with_failures, got %s", run.Status)
	}
	if run.SucceededBatches != 2 || run.FailedBatches != 1 {
		t.Fatalf("expected 2 succeeded / 1 failed, got %+v", run)
	}
}

func TestExecutorConcurrencyLimit(t *testing.T) {
	store := newMemStore()
	const concurrency = 3
	var active, maxActive int32
	arrive := make(chan struct{}, 100)
	release := make(chan struct{})
	fetch := func(reqs []tdx.QuoteRequest) ([]tdx.Quote, error) {
		n := atomic.AddInt32(&active, 1)
		for {
			m := atomic.LoadInt32(&maxActive)
			if n <= m || atomic.CompareAndSwapInt32(&maxActive, m, n) {
				break
			}
		}
		arrive <- struct{}{}
		<-release
		atomic.AddInt32(&active, -1)
		return okFetch(reqs)
	}
	e := newTestExecutor(t, store, ExecutorConfig{Concurrency: concurrency}, fetch)
	go func() {
		for i := 0; i < concurrency; i++ {
			<-arrive
		}
		close(release)
	}()
	run, err := e.Run(context.Background(), RunOptions{RunID: "r1", Plan: plan("a", "b", "c", "d", "e", "f")})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if run.SucceededBatches != 6 {
		t.Fatalf("expected all 6 succeeded, got %+v", run)
	}
	if got := atomic.LoadInt32(&maxActive); got != concurrency {
		t.Fatalf("expected max concurrency %d, observed %d", concurrency, got)
	}
}

func TestExecutorResumeSkipsSucceeded(t *testing.T) {
	store := newMemStore()
	p := plan("600000", "600001")

	// First run: batch 1 succeeds, batch 2 fails.
	fail2 := func(reqs []tdx.QuoteRequest) ([]tdx.Quote, error) {
		if reqs[0].Symbol == "600001" {
			return nil, errors.New("transient transport")
		}
		return okFetch(reqs)
	}
	e1 := newTestExecutor(t, store, ExecutorConfig{Concurrency: 1, RetryBudget: 0}, fail2)
	if _, err := e1.Run(context.Background(), RunOptions{RunID: "r1", Plan: p}); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// Resume: count fetches; only batch 2 should run.
	var fetched []string
	var mu sync.Mutex
	resumeFetch := func(reqs []tdx.QuoteRequest) ([]tdx.Quote, error) {
		mu.Lock()
		fetched = append(fetched, reqs[0].Symbol)
		mu.Unlock()
		return okFetch(reqs)
	}
	e2 := newTestExecutor(t, store, ExecutorConfig{Concurrency: 1, RetryBudget: 0}, resumeFetch)
	run, err := e2.Run(context.Background(), RunOptions{RunID: "r1", Plan: p, Resume: true})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if run.Status != StatusCompleted {
		t.Fatalf("expected completed after resume, got %s", run.Status)
	}
	if len(fetched) != 1 || fetched[0] != "600001" {
		t.Fatalf("resume should only fetch batch 2, got %v", fetched)
	}
	if run.SucceededBatches != 2 {
		t.Fatalf("expected 2 succeeded total, got %d", run.SucceededBatches)
	}
}

func TestExecutorRejectsIncompatibleResume(t *testing.T) {
	store := newMemStore()
	p := plan("600000", "600001")
	e := newTestExecutor(t, store, ExecutorConfig{Concurrency: 1}, okFetch)
	if _, err := e.Run(context.Background(), RunOptions{RunID: "r1", Plan: p}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	// Resume with a different batch size -> incompatible.
	p2, _ := PlanSweep(context.Background(), PlanOptions{Requests: reqs("sh", "600000", "600001"), BatchSize: 2}, nil)
	_, err := e.Run(context.Background(), RunOptions{RunID: "r1", Plan: p2, Resume: true})
	if err == nil {
		t.Fatalf("expected incompatible resume to be rejected")
	}
}
