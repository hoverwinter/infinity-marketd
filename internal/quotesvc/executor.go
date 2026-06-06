package quotesvc

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

// Run and batch status values persisted to the ops plane.
const (
	StatusRunning        = "running"
	StatusCompleted      = "completed"
	StatusWithFailures   = "completed_with_failures"
	StatusInterrupted    = "interrupted"
	BatchStatusRunning   = "running"
	BatchStatusSucceeded = "succeeded"
	BatchStatusFailed    = "failed"
	BatchStatusSkipped   = "skipped"
)

// StateStore persists durable run/batch state. *clickhouse.Store satisfies it.
type StateStore interface {
	SaveQuoteServiceRun(ctx context.Context, run model.QuoteServiceRun) error
	SaveQuoteServiceBatch(ctx context.Context, batch model.QuoteServiceBatch) error
	LoadQuoteServiceRun(ctx context.Context, runID string) (model.QuoteServiceRun, bool, error)
	LoadQuoteServiceBatches(ctx context.Context, runID string) ([]model.QuoteServiceBatch, error)
}

// ExecutorConfig configures sweep execution policy.
type ExecutorConfig struct {
	Concurrency      int
	RetryBudget      int
	BackoffBase      time.Duration
	BackoffMax       time.Duration
	GlobalRatePerSec float64
	PerServerRate    float64
	Burst            int
}

// Executor runs and resumes quote sweeps over a pool set, applying rate limits,
// retries, failure isolation, and durable progress recording.
type Executor struct {
	pools     *Pools
	store     StateStore
	cfg       ExecutorConfig
	global    *Limiter
	perServer map[string]*Limiter
	now       func() time.Time
	sleep     func(ctx context.Context, d time.Duration) error
	randFloat func() float64

	mu          sync.Mutex
	lastSuccess time.Time
}

// LastSuccessfulQuote returns the time of the most recent successful fetch.
func (e *Executor) LastSuccessfulQuote() (time.Time, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastSuccess, !e.lastSuccess.IsZero()
}

// GlobalLimiterTokens reports the global limiter's available tokens.
func (e *Executor) GlobalLimiterTokens() float64 {
	if e.global == nil {
		return 0
	}
	return e.global.Tokens()
}

// NewExecutor builds an executor. now/sleep may be nil (real clock / real sleep).
func NewExecutor(pools *Pools, store StateStore, cfg ExecutorConfig, servers []string, now func() time.Time, sleep func(context.Context, time.Duration) error) *Executor {
	if now == nil {
		now = time.Now
	}
	if sleep == nil {
		sleep = defaultSleep
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	e := &Executor{
		pools:     pools,
		store:     store,
		cfg:       cfg,
		now:       now,
		sleep:     sleep,
		randFloat: rand.Float64,
		perServer: make(map[string]*Limiter, len(servers)),
	}
	e.global = NewLimiter(cfg.GlobalRatePerSec, cfg.Burst, now, sleep)
	for _, s := range servers {
		e.perServer[s] = NewLimiter(cfg.PerServerRate, cfg.Burst, now, sleep)
	}
	return e
}

// RunOptions describes a sweep execution request.
type RunOptions struct {
	RunID  string // empty -> generated (ignored when Resume is true and required)
	Plan   SweepPlan
	Resume bool
	// Drain, when closed, stops launching new batches (they are recorded as
	// skipped) while in-flight batches finish; used for graceful shutdown.
	Drain <-chan struct{}
}

type batchResult struct {
	batch      Batch
	rows       int
	attempts   int
	kind       FailureKind
	err        error
	startedAt  time.Time
	finishedAt time.Time
}

// Run executes (or resumes) a sweep, recording durable run/batch state and
// returning the final run record. It continues past individual batch failures.
func (e *Executor) Run(ctx context.Context, opts RunOptions) (model.QuoteServiceRun, error) {
	plan := opts.Plan
	if len(plan.Batches) == 0 {
		return model.QuoteServiceRun{}, fmt.Errorf("sweep plan has no batches")
	}

	priorSucceeded := map[uint32]bool{}
	var run model.QuoteServiceRun

	if opts.Resume {
		if opts.RunID == "" {
			return model.QuoteServiceRun{}, fmt.Errorf("resume requires a run id")
		}
		existing, ok, err := e.store.LoadQuoteServiceRun(ctx, opts.RunID)
		if err != nil {
			return model.QuoteServiceRun{}, fmt.Errorf("load run for resume: %w", err)
		}
		if !ok {
			return model.QuoteServiceRun{}, fmt.Errorf("resume run %q not found", opts.RunID)
		}
		if err := checkResumeCompatible(existing, plan); err != nil {
			return model.QuoteServiceRun{}, err
		}
		batches, err := e.store.LoadQuoteServiceBatches(ctx, opts.RunID)
		if err != nil {
			return model.QuoteServiceRun{}, fmt.Errorf("load batches for resume: %w", err)
		}
		for _, b := range batches {
			if b.Status == BatchStatusSucceeded {
				priorSucceeded[b.BatchNo] = true
			}
		}
		run = existing
	} else {
		runID := opts.RunID
		if runID == "" {
			runID = fmt.Sprintf("qs-%d", e.now().UnixNano())
		}
		run = model.QuoteServiceRun{
			RunID:          runID,
			Markets:        plan.Markets,
			SymbolSource:   string(plan.SymbolSource),
			BatchSize:      uint32(plan.BatchSize),
			PlannedSymbols: uint32(len(plan.Requests)),
			PlannedBatches: uint32(len(plan.Batches)),
			StartedAt:      e.now(),
		}
	}

	run.Status = StatusRunning
	run.UpdatedAt = e.now()
	if err := e.store.SaveQuoteServiceRun(ctx, run); err != nil {
		return run, fmt.Errorf("record run start: %w", err)
	}

	var pending []Batch
	succeeded := 0
	for _, b := range plan.Batches {
		if priorSucceeded[uint32(b.No)] {
			succeeded++
			continue
		}
		pending = append(pending, b)
	}

	var (
		mu          sync.Mutex
		failed      int
		skipped     int
		rows        uint64
		interrupted bool
		wg          sync.WaitGroup
	)
	sem := make(chan struct{}, e.cfg.Concurrency)

	skip := func(b Batch) {
		e.recordBatch(run.RunID, b, batchResult{batch: b, kind: FailureCancelled, err: context.Canceled, startedAt: e.now(), finishedAt: e.now()})
		mu.Lock()
		skipped++
		interrupted = true
		mu.Unlock()
	}
	for _, b := range pending {
		if ctx.Err() != nil || drained(opts.Drain) {
			skip(b)
			continue
		}
		sem <- struct{}{}
		// Re-check after possibly waiting on the slot: shutdown may have begun
		// while this batch was queued, in which case it must not launch.
		if ctx.Err() != nil || drained(opts.Drain) {
			<-sem
			skip(b)
			continue
		}
		wg.Add(1)
		go func(b Batch) {
			defer wg.Done()
			defer func() { <-sem }()
			res := e.runBatch(ctx, b)
			e.recordBatch(run.RunID, b, res)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case res.kind == FailureNone:
				succeeded++
				rows += uint64(res.rows)
			case res.kind == FailureCancelled || isCancellation(res.err):
				skipped++
				interrupted = true
			default:
				failed++
			}
		}(b)
	}
	wg.Wait()

	now := e.now()
	run.SucceededBatches = uint32(succeeded)
	run.FailedBatches = uint32(failed)
	run.SkippedBatches = uint32(skipped)
	run.RowsFetched = rows
	run.FinishedAt = &now
	dur := uint64(now.Sub(run.StartedAt) / time.Millisecond)
	run.DurationMS = &dur
	switch {
	case interrupted || ctx.Err() != nil:
		run.Status = StatusInterrupted
	case failed > 0:
		run.Status = StatusWithFailures
	default:
		run.Status = StatusCompleted
	}
	run.UpdatedAt = now
	// Persist final state even if ctx was cancelled, so resume sees real state.
	if err := e.store.SaveQuoteServiceRun(detached(ctx), run); err != nil {
		return run, fmt.Errorf("record run completion: %w", err)
	}
	return run, nil
}

// runBatch executes one batch with retries and backoff. Decode failures are not
// retried so parser regressions are preserved.
func (e *Executor) runBatch(ctx context.Context, b Batch) batchResult {
	res := batchResult{batch: b, startedAt: e.now()}
	for attempt := 0; attempt <= e.cfg.RetryBudget; attempt++ {
		res.attempts = attempt + 1
		if attempt > 0 {
			if err := e.sleep(ctx, e.backoff(attempt)); err != nil {
				res.kind = FailureCancelled
				res.err = err
				res.finishedAt = e.now()
				return res
			}
		}
		quotes, kind, err := e.fetchOnce(ctx, b)
		if err == nil {
			res.rows = len(quotes)
			res.kind = FailureNone
			res.err = nil
			res.finishedAt = e.now()
			return res
		}
		res.kind = kind
		res.err = err
		if !recoverable(kind) {
			break // decode/cancelled -> preserve, do not retry
		}
	}
	res.finishedAt = e.now()
	return res
}

// fetchOnce attempts one quote fetch, falling back across servers. Decode
// failures return immediately (server-independent parser problem).
func (e *Executor) fetchOnce(ctx context.Context, b Batch) ([]tdx.Quote, FailureKind, error) {
	servers := e.pools.order()
	if len(servers) == 0 {
		return nil, FailureServerSelect, errServerSelect
	}
	lastErr := errServerSelect
	lastKind := FailureServerSelect
	for _, server := range servers {
		if err := e.waitLimiters(ctx, server); err != nil {
			return nil, FailureRateLimit, err
		}
		pool := e.pools.pool(server)
		pc, err := pool.Acquire(ctx)
		if err != nil {
			e.pools.markHealthy(server, false)
			lastErr, lastKind = err, classifyFailure(err)
			continue
		}
		quotes, ferr := pc.conn.Fetch(b.Requests)
		if ferr != nil {
			pool.Release(pc, false)
			kind := classifyFailure(ferr)
			if kind == FailureDecode {
				return nil, FailureDecode, ferr
			}
			e.pools.markHealthy(server, false)
			lastErr, lastKind = ferr, kind
			continue
		}
		pool.Release(pc, true)
		e.pools.markHealthy(server, true)
		e.mu.Lock()
		e.lastSuccess = e.now()
		e.mu.Unlock()
		return quotes, FailureNone, nil
	}
	return nil, lastKind, fmt.Errorf("all servers failed: %w", lastErr)
}

// drained reports whether the drain channel has been closed/signalled.
func drained(drain <-chan struct{}) bool {
	if drain == nil {
		return false
	}
	select {
	case <-drain:
		return true
	default:
		return false
	}
}

func (e *Executor) waitLimiters(ctx context.Context, server string) error {
	if e.global != nil {
		if err := e.global.Wait(ctx); err != nil {
			return err
		}
	}
	if lim := e.perServer[server]; lim != nil {
		if err := lim.Wait(ctx); err != nil {
			return err
		}
	}
	return nil
}

// backoff returns an exponential, capped, jittered delay for the given attempt.
func (e *Executor) backoff(attempt int) time.Duration {
	base := e.cfg.BackoffBase
	if base <= 0 {
		base = 200 * time.Millisecond
	}
	d := base << (attempt - 1)
	if e.cfg.BackoffMax > 0 && d > e.cfg.BackoffMax {
		d = e.cfg.BackoffMax
	}
	// full jitter on the upper half: [d/2, d]
	jitter := time.Duration(float64(d) * 0.5 * e.randFloat())
	return d/2 + jitter
}

func (e *Executor) recordBatch(runID string, b Batch, res batchResult) {
	status := BatchStatusSucceeded
	switch {
	case res.kind == FailureNone:
		status = BatchStatusSucceeded
	case res.kind == FailureCancelled || isCancellation(res.err):
		status = BatchStatusSkipped
	default:
		status = BatchStatusFailed
	}
	rec := model.QuoteServiceBatch{
		RunID:       runID,
		BatchNo:     uint32(b.No),
		Status:      status,
		SymbolCount: uint32(len(b.Requests)),
		FirstSymbol: b.FirstSymbol(),
		LastSymbol:  b.LastSymbol(),
		Attempts:    uint32(res.attempts),
		RowsFetched: uint64(res.rows),
		FailureKind: string(res.kind),
		UpdatedAt:   e.now(),
	}
	if !res.startedAt.IsZero() {
		started := res.startedAt
		rec.StartedAt = &started
	}
	if !res.finishedAt.IsZero() {
		finished := res.finishedAt
		rec.FinishedAt = &finished
		if rec.StartedAt != nil {
			dur := uint64(finished.Sub(*rec.StartedAt) / time.Millisecond)
			rec.DurationMS = &dur
		}
	}
	if res.err != nil {
		rec.Error = res.err.Error()
	}
	_ = e.store.SaveQuoteServiceBatch(context.WithoutCancel(context.Background()), rec)
}

func checkResumeCompatible(existing model.QuoteServiceRun, plan SweepPlan) error {
	if existing.SymbolSource != string(plan.SymbolSource) {
		return fmt.Errorf("resume rejected: symbol source changed (%s -> %s)", existing.SymbolSource, plan.SymbolSource)
	}
	if existing.BatchSize != uint32(plan.BatchSize) {
		return fmt.Errorf("resume rejected: batch size changed (%d -> %d)", existing.BatchSize, plan.BatchSize)
	}
	if existing.PlannedBatches != uint32(len(plan.Batches)) {
		return fmt.Errorf("resume rejected: planned batch count changed (%d -> %d)", existing.PlannedBatches, len(plan.Batches))
	}
	if !equalStrings(existing.Markets, plan.Markets) {
		return fmt.Errorf("resume rejected: markets changed (%v -> %v)", existing.Markets, plan.Markets)
	}
	return nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// detached returns a context that carries values but is not cancelled when the
// parent is, so final ops state is still persisted during shutdown.
func detached(ctx context.Context) context.Context {
	return context.WithoutCancel(ctx)
}
