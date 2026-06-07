package ingest

import (
	"context"
	"fmt"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

// OnlineOps is the narrow ops-plane write surface the online ingest runner needs.
// *clickhouse.Store satisfies it. Kept as an interface so the runner lifecycle is
// testable without a live ClickHouse connection.
type OnlineOps interface {
	InsertQualityIssues(ctx context.Context, issues []model.QualityIssue) error
	InsertWatermark(ctx context.Context, wm model.Watermark) error
	InsertTaskRun(ctx context.Context, run model.TaskRun) error
}

// OnlineJob is one explicit online-provider-to-ClickHouse import job. T is the
// product's normalized row type. The runner owns lifecycle and ops recording;
// the adapter owns fetch, normalize, dedup, logical key, write target, and
// watermark bounds. The runner never inspects provider packet structs.
type OnlineJob[T any] struct {
	Dataset     string
	TargetTable string
	TaskType    string
	InputFormat string
	Asset       string // watermark asset key, e.g. "sh:600519"; empty disables watermarking
	Params      string // product-owned JSON params recorded on the task run
	DryRun      bool
	Ops         OnlineOps        // nil for dry-run / when no store is available
	Now         func() time.Time // nil -> time.Now

	// Produce fetches and normalizes rows. It returns rows, a skipped-row count,
	// product quality issues, and a fatal error. All data semantics live here.
	Produce func(ctx context.Context, runID string) ([]T, uint64, []model.QualityIssue, error)
	// Write persists rows; the adapter closes over its Store write method.
	Write func(ctx context.Context, rows []T) error
	// Bounds returns optional watermark min/max for the produced rows.
	Bounds func(rows []T) (*time.Time, *time.Time)
}

// OnlineResult is the runner's product-agnostic outcome.
type OnlineResult struct {
	RunID       string
	Dataset     string
	TargetTable string
	RowsWritten uint64
	RowsSkipped uint64
	Issues      []model.QualityIssue
	DryRun      bool
}

// RunOnlineJob executes one online import: produce rows, then (unless dry-run)
// write rows, quality issues, a watermark, and a task run. On a fatal error it
// records a failed task run when ops are available. Dry-run writes nothing.
func RunOnlineJob[T any](ctx context.Context, job OnlineJob[T]) (OnlineResult, error) {
	now := job.Now
	if now == nil {
		now = time.Now
	}
	if !job.DryRun && job.Ops == nil {
		return OnlineResult{}, fmt.Errorf("ops store is required when dry-run is false")
	}
	runID := newRunID()
	started := now()
	result := OnlineResult{RunID: runID, Dataset: job.Dataset, TargetTable: job.TargetTable, DryRun: job.DryRun}

	rows, skipped, issues, err := job.Produce(ctx, runID)
	result.RowsSkipped = skipped
	result.Issues = issues
	if err != nil {
		job.recordRun(ctx, runID, started, now, 0, skipped, err, "failed")
		return result, err
	}
	result.RowsWritten = uint64(len(rows))
	if job.DryRun {
		return result, nil
	}

	if err := job.Write(ctx, rows); err != nil {
		job.recordRun(ctx, runID, started, now, 0, skipped, err, "failed")
		return result, err
	}
	if len(issues) > 0 {
		if err := job.Ops.InsertQualityIssues(ctx, issues); err != nil {
			job.recordRun(ctx, runID, started, now, result.RowsWritten, skipped, err, "failed")
			return result, err
		}
	}

	status := "success"
	message := "ok"
	if len(issues) > 0 {
		status = "degraded"
		message = fmt.Sprintf("%d quality issue(s)", len(issues))
	}
	if result.RowsWritten == 0 {
		message = "no rows returned"
	}
	if job.Bounds != nil && job.Asset != "" {
		minWM, maxWM := job.Bounds(rows)
		if minWM != nil || maxWM != nil {
			if err := job.Ops.InsertWatermark(ctx, model.Watermark{
				Dataset:      job.Dataset,
				Asset:        job.Asset,
				Status:       status,
				MinWatermark: minWM,
				MaxWatermark: maxWM,
				RowsWritten:  result.RowsWritten,
				Message:      message,
				UpdatedAt:    now(),
			}); err != nil {
				job.recordRun(ctx, runID, started, now, result.RowsWritten, skipped, err, "failed")
				return result, err
			}
		}
	}
	if err := job.recordRun(ctx, runID, started, now, result.RowsWritten, skipped, nil, status); err != nil {
		return result, err
	}
	return result, nil
}

// recordRun writes a task run unless dry-run or ops are unavailable.
func (job OnlineJob[T]) recordRun(ctx context.Context, runID string, started time.Time, now func() time.Time, rowsWritten, rowsSkipped uint64, failure error, status string) error {
	if job.DryRun || job.Ops == nil {
		return nil
	}
	finished := now()
	duration := uint64(finished.Sub(started).Milliseconds())
	errText := ""
	if failure != nil {
		errText = failure.Error()
	}
	return job.Ops.InsertTaskRun(ctx, model.TaskRun{
		RunID:       runID,
		Dataset:     job.Dataset,
		TaskType:    job.TaskType,
		Status:      status,
		TargetTable: job.TargetTable,
		InputFormat: job.InputFormat,
		Params:      job.Params,
		StartedAt:   started,
		FinishedAt:  &finished,
		DurationMS:  &duration,
		RowsWritten: rowsWritten,
		RowsSkipped: rowsSkipped,
		Error:       errText,
		UpdatedAt:   finished,
	})
}
