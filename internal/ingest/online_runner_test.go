package ingest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

type fakeOps struct {
	taskRuns   []model.TaskRun
	watermarks []model.Watermark
	issues     []model.QualityIssue
}

func (f *fakeOps) InsertQualityIssues(ctx context.Context, issues []model.QualityIssue) error {
	f.issues = append(f.issues, issues...)
	return nil
}
func (f *fakeOps) InsertWatermark(ctx context.Context, wm model.Watermark) error {
	f.watermarks = append(f.watermarks, wm)
	return nil
}
func (f *fakeOps) InsertTaskRun(ctx context.Context, run model.TaskRun) error {
	f.taskRuns = append(f.taskRuns, run)
	return nil
}

type tsRow struct{ t time.Time }

func tsBounds(rows []tsRow) (*time.Time, *time.Time) {
	if len(rows) == 0 {
		return nil, nil
	}
	min, max := rows[0].t, rows[0].t
	for _, r := range rows[1:] {
		if r.t.Before(min) {
			min = r.t
		}
		if r.t.After(max) {
			max = r.t
		}
	}
	return &min, &max
}

func baseJob(ops OnlineOps, written *[]tsRow) OnlineJob[tsRow] {
	return OnlineJob[tsRow]{
		Dataset:     "ds",
		TargetTable: "tbl",
		TaskType:    "test_import",
		InputFormat: "test",
		Asset:       "sh:600519",
		Params:      `{"k":"v"}`,
		Ops:         ops,
		Now:         func() time.Time { return time.Unix(1000, 0) },
		Bounds:      tsBounds,
		Write: func(ctx context.Context, rows []tsRow) error {
			*written = append(*written, rows...)
			return nil
		},
	}
}

func TestRunOnlineJobSuccess(t *testing.T) {
	ops := &fakeOps{}
	var written []tsRow
	job := baseJob(ops, &written)
	job.Produce = func(ctx context.Context, runID string) ([]tsRow, uint64, []model.QualityIssue, error) {
		return []tsRow{{time.Unix(10, 0)}, {time.Unix(20, 0)}}, 0, nil, nil
	}
	res, err := RunOnlineJob(context.Background(), job)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.RowsWritten != 2 || len(written) != 2 {
		t.Fatalf("expected 2 rows written, got res=%d write=%d", res.RowsWritten, len(written))
	}
	if len(ops.taskRuns) != 1 || ops.taskRuns[0].Status != "success" {
		t.Fatalf("expected 1 success task run, got %+v", ops.taskRuns)
	}
	if len(ops.watermarks) != 1 || ops.watermarks[0].Asset != "sh:600519" {
		t.Fatalf("expected watermark, got %+v", ops.watermarks)
	}
}

func TestRunOnlineJobDryRunWritesNothing(t *testing.T) {
	ops := &fakeOps{}
	var written []tsRow
	job := baseJob(ops, &written)
	job.DryRun = true
	job.Produce = func(ctx context.Context, runID string) ([]tsRow, uint64, []model.QualityIssue, error) {
		return []tsRow{{time.Unix(10, 0)}}, 3, []model.QualityIssue{{IssueType: "x"}}, nil
	}
	res, err := RunOnlineJob(context.Background(), job)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.RowsWritten != 1 || res.RowsSkipped != 3 || len(res.Issues) != 1 {
		t.Fatalf("dry-run should still report counts, got %+v", res)
	}
	if len(written) != 0 || len(ops.taskRuns) != 0 || len(ops.watermarks) != 0 || len(ops.issues) != 0 {
		t.Fatalf("dry-run must not write anything: write=%d runs=%d wm=%d issues=%d", len(written), len(ops.taskRuns), len(ops.watermarks), len(ops.issues))
	}
}

func TestRunOnlineJobEmptyRowsNoWatermark(t *testing.T) {
	ops := &fakeOps{}
	var written []tsRow
	job := baseJob(ops, &written)
	job.Produce = func(ctx context.Context, runID string) ([]tsRow, uint64, []model.QualityIssue, error) {
		return nil, 0, nil, nil
	}
	res, err := RunOnlineJob(context.Background(), job)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.RowsWritten != 0 {
		t.Fatalf("expected 0 rows, got %d", res.RowsWritten)
	}
	if len(ops.watermarks) != 0 {
		t.Fatalf("empty rows must not record a watermark, got %+v", ops.watermarks)
	}
	if len(ops.taskRuns) != 1 {
		t.Fatalf("expected 1 task run, got %d", len(ops.taskRuns))
	}
}

func TestRunOnlineJobInsertsQualityIssues(t *testing.T) {
	ops := &fakeOps{}
	var written []tsRow
	job := baseJob(ops, &written)
	job.Produce = func(ctx context.Context, runID string) ([]tsRow, uint64, []model.QualityIssue, error) {
		return []tsRow{{time.Unix(10, 0)}}, 1, []model.QualityIssue{{IssueType: "bad"}}, nil
	}
	if _, err := RunOnlineJob(context.Background(), job); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(ops.issues) != 1 {
		t.Fatalf("expected 1 issue inserted, got %d", len(ops.issues))
	}
	if ops.taskRuns[0].Status != "degraded" {
		t.Fatalf("expected degraded status with issues, got %q", ops.taskRuns[0].Status)
	}
}

func TestRunOnlineJobRecordsProduceFailure(t *testing.T) {
	ops := &fakeOps{}
	var written []tsRow
	job := baseJob(ops, &written)
	job.Produce = func(ctx context.Context, runID string) ([]tsRow, uint64, []model.QualityIssue, error) {
		return nil, 0, nil, errors.New("fetch boom")
	}
	_, err := RunOnlineJob(context.Background(), job)
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(written) != 0 {
		t.Fatalf("no rows should be written on produce failure")
	}
	if len(ops.taskRuns) != 1 || ops.taskRuns[0].Status != "failed" || ops.taskRuns[0].Error == "" {
		t.Fatalf("expected 1 failed task run with error, got %+v", ops.taskRuns)
	}
}

func TestRunOnlineJobRecordsWriteFailure(t *testing.T) {
	ops := &fakeOps{}
	var written []tsRow
	job := baseJob(ops, &written)
	job.Write = func(ctx context.Context, rows []tsRow) error { return errors.New("write boom") }
	job.Produce = func(ctx context.Context, runID string) ([]tsRow, uint64, []model.QualityIssue, error) {
		return []tsRow{{time.Unix(10, 0)}}, 0, nil, nil
	}
	_, err := RunOnlineJob(context.Background(), job)
	if err == nil {
		t.Fatalf("expected write error")
	}
	if len(ops.taskRuns) != 1 || ops.taskRuns[0].Status != "failed" {
		t.Fatalf("expected failed task run, got %+v", ops.taskRuns)
	}
}

func TestRunOnlineJobRequiresOpsWhenNotDryRun(t *testing.T) {
	var written []tsRow
	job := baseJob(nil, &written)
	job.Produce = func(ctx context.Context, runID string) ([]tsRow, uint64, []model.QualityIssue, error) {
		return nil, 0, nil, nil
	}
	if _, err := RunOnlineJob(context.Background(), job); err == nil {
		t.Fatalf("expected error when ops is nil and not dry-run")
	}
}
