package securitymaster

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeSource struct {
	rows []SourceRow
	err  error
}

func (s fakeSource) Fetch(context.Context, []string) ([]SourceRow, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.rows, nil
}

type fakeWriter struct {
	beginCalled    bool
	finishCalled   bool
	securityWrites int
	aliasWrites    int
	historyWrites  int
	finishedStatus string
}

func (w *fakeWriter) BeginRefreshRun(context.Context, RefreshRun) (int64, error) {
	w.beginCalled = true
	return 7, nil
}

func (w *fakeWriter) FinishRefreshRun(_ context.Context, _ int64, run RefreshRun) error {
	w.finishCalled = true
	w.finishedStatus = run.Status
	return nil
}

func (w *fakeWriter) UpsertSecurity(context.Context, Security) error {
	w.securityWrites++
	return nil
}

func (w *fakeWriter) UpsertAliases(_ context.Context, aliases []Alias) (int, error) {
	w.aliasWrites += len(aliases)
	return len(aliases), nil
}

func (w *fakeWriter) UpsertNameHistory(_ context.Context, history []NameHistory) (int, error) {
	w.historyWrites += len(history)
	return len(history), nil
}

func TestRefreshDryRunDoesNotWriteStore(t *testing.T) {
	writer := &fakeWriter{}
	summary, err := Refresh(context.Background(), RefreshOptions{
		SourceName: SourceFile,
		Markets:    []string{"bj"},
		DryRun:     true,
		Source: fakeSource{rows: []SourceRow{{
			Market:      "bj",
			Symbol:      "920001",
			Name:        "北证测试",
			ListingDate: "20260610",
		}}},
		Store: writer,
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Status != RefreshStatusDryRun || summary.RowsUpserted != 1 || summary.HistoryUpserted != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if writer.beginCalled || writer.securityWrites != 0 {
		t.Fatalf("writer was used: %+v", writer)
	}
}

func TestRefreshRecordsSelectedSourceFailure(t *testing.T) {
	writer := &fakeWriter{}
	now := time.Date(2026, 6, 10, 9, 30, 0, 0, time.UTC)
	sourceErr := errors.New("tdx source failed")
	summary, err := Refresh(context.Background(), RefreshOptions{
		SourceName: SourceTDX,
		Markets:    []string{"bj"},
		Source:     fakeSource{err: sourceErr},
		Store:      writer,
		Now:        func() time.Time { return now },
	})
	if !errors.Is(err, sourceErr) {
		t.Fatalf("err = %v", err)
	}
	if summary.Status != RefreshStatusFailed || summary.Error == "" {
		t.Fatalf("summary = %+v", summary)
	}
	if !writer.beginCalled || !writer.finishCalled || writer.finishedStatus != RefreshStatusFailed {
		t.Fatalf("writer = %+v", writer)
	}
}
