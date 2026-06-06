package quotesvc

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/config"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

func TestServiceHealthOnStart(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	cfg := config.Default().QuoteService
	cfg.Servers = []string{"s1", "s2"}
	discover := func(ctx context.Context, market string) ([]tdx.QuoteRequest, error) { return nil, nil }
	svc := NewService(cfg, newMemStore(), discover, clk.Now)
	defer svc.Close()

	h := svc.Health()
	if h.State != "running" {
		t.Fatalf("expected running, got %s", h.State)
	}
	if len(h.Servers) != 2 || h.HealthyServers != 2 {
		t.Fatalf("expected 2 healthy servers, got %+v", h)
	}
	if h.LastSuccessfulQuote != nil {
		t.Fatalf("expected no successful quote yet")
	}
}

func TestExecutorGracefulDrain(t *testing.T) {
	store := newMemStore()
	drain := make(chan struct{})
	release := make(chan struct{})
	var started int32
	fetch := func(reqs []tdx.QuoteRequest) ([]tdx.Quote, error) {
		if atomic.AddInt32(&started, 1) == 1 {
			close(drain) // begin graceful shutdown once the first batch is in-flight
			<-release    // hold the first batch in-flight
		}
		return okFetch(reqs)
	}
	e := newTestExecutor(t, store, ExecutorConfig{Concurrency: 1}, fetch)

	var run model.QuoteServiceRun
	var rerr error
	done := make(chan struct{})
	go func() {
		run, rerr = e.Run(context.Background(), RunOptions{RunID: "r1", Plan: plan("a", "b", "c"), Drain: drain})
		close(done)
	}()
	<-drain        // first batch started and shutdown signalled
	close(release) // let the first batch finish
	<-done

	if rerr != nil {
		t.Fatalf("run: %v", rerr)
	}
	if run.Status != StatusInterrupted {
		t.Fatalf("expected interrupted, got %s", run.Status)
	}
	if run.SucceededBatches != 1 || run.SkippedBatches != 2 {
		t.Fatalf("expected 1 succeeded / 2 skipped, got %+v", run)
	}
}

func TestExecutorRecordsLastSuccessfulQuote(t *testing.T) {
	clk := &fakeClock{t: time.Unix(2000, 0)}
	store := newMemStore()
	servers := []string{"s1"}
	dialer := func(ctx context.Context, server string) (Conn, error) { return &scriptConn{fetch: okFetch}, nil }
	pools := NewPools(servers, dialer, PoolConfig{MaxConns: 2}, clk.Now)
	e := NewExecutor(pools, store, ExecutorConfig{Concurrency: 1}, servers, clk.Now, noopSleep)
	if _, err := e.Run(context.Background(), RunOptions{RunID: "r1", Plan: plan("600000")}); err != nil {
		t.Fatalf("run: %v", err)
	}
	ts, ok := e.LastSuccessfulQuote()
	if !ok || !ts.Equal(time.Unix(2000, 0)) {
		t.Fatalf("expected last successful quote at 2000, got %v ok=%v", ts, ok)
	}
}
