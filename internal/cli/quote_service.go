package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/signal"
	"syscall"
	"time"

	chstore "github.com/hoverwinter/infinity-marketd/internal/clickhouse"
	"github.com/hoverwinter/infinity-marketd/internal/config"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/quotesvc"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

// The quote service persists only ops-plane run/batch records: its StateStore
// surface exposes no fact-table writers, so realtime snapshots can never reach
// canonical market fact tables through this path.
var _ quotesvc.StateStore = (*chstore.Store)(nil)

func runQuoteServe(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var symbols symbolFlags
	var markets listFlags
	var limit int
	var resumeID string
	fs := newFlagSet("quote-serve", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.Var(&symbols, "symbol", "explicit symbol to quote; repeat or comma-separate; omit to discover by market")
	fs.Var(&markets, "market", "market to discover; repeat or comma-separate (defaults to config)")
	fs.IntVar(&limit, "limit", 0, "maximum symbols to sweep")
	fs.StringVar(&resumeID, "resume", "", "resume an existing run id, scheduling only non-succeeded batches")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := config.Load(overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cleanup, ok := setupLogging(cfg, stderr)
	if !ok {
		return 1
	}
	defer cleanup()

	requests := make([]tdx.QuoteRequest, 0, len(symbols))
	for _, symbol := range symbols {
		req, perr := tdx.ParseQuoteRequest(symbol)
		if perr != nil {
			fmt.Fprintln(stderr, perr)
			return 2
		}
		requests = append(requests, req)
	}

	store, err := chstore.Open(ctx, cfg.ClickHouse)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer store.Close()

	svc := quotesvc.NewService(cfg.QuoteService, store, nil, nil)
	defer svc.Close()

	plan, err := svc.Plan(ctx, requests, []string(markets), limit)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "service: running, servers=%d, planned_symbols=%d, planned_batches=%d\n", svc.Health().HealthyServers, len(plan.Requests), len(plan.Batches))

	run, err := serveSweep(ctx, svc, quotesvc.RunOptions{
		RunID:  resumeID,
		Plan:   plan,
		Resume: resumeID != "",
	}, cfg.QuoteService.ShutdownDeadline.Duration())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	formatQuoteRun(stdout, run)
	formatQuoteHealth(stdout, svc.Health())
	return 0
}

// serveSweep runs a sweep with graceful shutdown: on SIGINT/SIGTERM it stops
// launching new batches (drain) and lets in-flight batches finish within the
// shutdown deadline before hard-cancelling.
func serveSweep(ctx context.Context, svc *quotesvc.Service, opts quotesvc.RunOptions, deadline time.Duration) (model.QuoteServiceRun, error) {
	sigCtx, stopSig := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stopSig()
	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	drain := make(chan struct{})
	done := make(chan struct{})
	opts.Drain = drain

	go func() {
		select {
		case <-done:
			return
		case <-sigCtx.Done():
		}
		close(drain) // stop launching new batches
		timer := time.NewTimer(deadline)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
			runCancel() // deadline reached; cancel in-flight batches
		}
	}()

	run, err := svc.RunSweep(runCtx, opts)
	close(done)
	return run, err
}

func runQuoteStatus(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var runID string
	var limit int
	var asJSON bool
	fs := newFlagSet("quote-status", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&runID, "run", "", "show batch detail for a specific run id")
	fs.IntVar(&limit, "limit", 20, "maximum runs to list")
	fs.BoolVar(&asJSON, "json", false, "emit JSON instead of text")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := config.Load(overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cleanup, ok := setupLogging(cfg, stderr)
	if !ok {
		return 1
	}
	defer cleanup()
	store, err := chstore.Open(ctx, cfg.ClickHouse)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer store.Close()

	if runID != "" {
		run, found, lerr := store.LoadQuoteServiceRun(ctx, runID)
		if lerr != nil {
			fmt.Fprintln(stderr, lerr)
			return 1
		}
		if !found {
			fmt.Fprintf(stderr, "run %q not found\n", runID)
			return 1
		}
		batches, lerr := store.LoadQuoteServiceBatches(ctx, runID)
		if lerr != nil {
			fmt.Fprintln(stderr, lerr)
			return 1
		}
		if asJSON {
			return emitJSON(stdout, stderr, map[string]any{"run": run, "batches": batches})
		}
		formatQuoteRun(stdout, run)
		for _, b := range batches {
			fmt.Fprintf(stdout, "  batch %d %s symbols=%d attempts=%d rows=%d %s\n", b.BatchNo, b.Status, b.SymbolCount, b.Attempts, b.RowsFetched, b.FailureKind)
		}
		return 0
	}

	runs, err := store.LatestQuoteServiceRuns(ctx, limit)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if asJSON {
		return emitJSON(stdout, stderr, runs)
	}
	if len(runs) == 0 {
		fmt.Fprintln(stdout, "runs: none")
		return 0
	}
	for _, run := range runs {
		formatQuoteRun(stdout, run)
	}
	return 0
}

func formatQuoteRun(out io.Writer, run model.QuoteServiceRun) {
	fmt.Fprintf(out, "run %s %s started=%s batches=%d/%d succeeded=%d failed=%d skipped=%d rows=%d\n",
		run.RunID, run.Status, run.StartedAt.Format("2006-01-02 15:04:05"),
		run.SucceededBatches+run.FailedBatches+run.SkippedBatches, run.PlannedBatches,
		run.SucceededBatches, run.FailedBatches, run.SkippedBatches, run.RowsFetched)
}

func formatQuoteHealth(out io.Writer, h quotesvc.Health) {
	last := "never"
	if h.LastSuccessfulQuote != nil {
		last = h.LastSuccessfulQuote.Format("2006-01-02 15:04:05")
	}
	fmt.Fprintf(out, "health: %s servers=%d/%d last_quote=%s\n", h.State, h.HealthyServers, len(h.Servers), last)
	for _, p := range h.Pools {
		fmt.Fprintf(out, "  server %s open=%d idle=%d heartbeat_failures=%d\n", p.Server, p.Open, p.Idle, p.HeartbeatFailures)
	}
}

func emitJSON(stdout io.Writer, stderr io.Writer, v any) int {
	encoded, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, string(encoded))
	return 0
}
