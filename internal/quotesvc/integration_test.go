package quotesvc

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

// TestSweepProgressEndToEnd drives a multi-batch sweep through real pools, the
// rate limiter, the executor, and the state store using fake TDX connections,
// asserting retry, decode preservation, failure isolation, and per-batch
// progress recording all hang together.
func TestSweepProgressEndToEnd(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	store := newMemStore()

	var mu sync.Mutex
	attemptsBySymbol := map[string]int{}
	fetch := func(reqs []tdx.QuoteRequest) ([]tdx.Quote, error) {
		sym := reqs[0].Symbol
		mu.Lock()
		attemptsBySymbol[sym]++
		n := attemptsBySymbol[sym]
		mu.Unlock()
		switch sym {
		case "600001": // both servers fail on the first attempt round, then succeeds on retry
			if n <= 2 {
				return nil, errors.New("connection reset")
			}
			return okFetch(reqs)
		case "600003": // permanent decode failure -> preserved, not retried
			return nil, errors.New("decode TDX HQ quote response: bad payload")
		default:
			return okFetch(reqs)
		}
	}

	servers := []string{"s1", "s2"}
	dialer := func(ctx context.Context, server string) (Conn, error) { return &scriptConn{fetch: fetch}, nil }
	pools := NewPools(servers, dialer, PoolConfig{MaxConns: 4, HeartbeatInterval: time.Second}, clk.Now)
	exec := NewExecutor(pools, store, ExecutorConfig{
		Concurrency:      2,
		RetryBudget:      2,
		GlobalRatePerSec: 1000, // permissive; just exercise the limiter path
		PerServerRate:    1000,
		Burst:            10,
	}, servers, clk.Now, noopSleep)
	exec.randFloat = func() float64 { return 0 }

	p := plan("600000", "600001", "600002", "600003", "600004")
	run, err := exec.Run(context.Background(), RunOptions{RunID: "run-e2e", Plan: p})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if run.SucceededBatches != 4 || run.FailedBatches != 1 {
		t.Fatalf("expected 4 succeeded / 1 failed, got %+v", run)
	}
	if run.Status != StatusWithFailures {
		t.Fatalf("expected completed_with_failures, got %s", run.Status)
	}
	if run.RowsFetched != 4 {
		t.Fatalf("expected 4 rows fetched, got %d", run.RowsFetched)
	}

	batches := store.batches["run-e2e"]
	if len(batches) != 5 {
		t.Fatalf("expected 5 batch records, got %d", len(batches))
	}
	// 600001 is the second batch (batch_no 2) and should show a retry.
	if b := batches[2]; b.Status != BatchStatusSucceeded || b.Attempts != 2 {
		t.Fatalf("batch 2 should succeed on retry, got %+v", b)
	}
	// 600003 is batch_no 4: decode failure, single attempt, preserved.
	if b := batches[4]; b.Status != BatchStatusFailed || b.Attempts != 1 || b.FailureKind != string(FailureDecode) {
		t.Fatalf("batch 4 should be a preserved decode failure, got %+v", b)
	}

	// Run record round-trips through the store.
	loaded, ok, err := store.LoadQuoteServiceRun(context.Background(), "run-e2e")
	if err != nil || !ok {
		t.Fatalf("load run: ok=%v err=%v", ok, err)
	}
	if loaded.PlannedBatches != 5 {
		t.Fatalf("expected 5 planned batches, got %d", loaded.PlannedBatches)
	}
}
