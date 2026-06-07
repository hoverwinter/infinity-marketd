package ingest

import (
	"context"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

// These regression tests exercise the intraday product logic (date modes,
// normalization, dedup) through the online ingest runner in dry-run mode, so
// they need neither a ClickHouse store nor a live TDX server.

func baseIntradayOpts() IntradayImportOptions {
	return IntradayImportOptions{
		Market:   "sh",
		Symbol:   "600519",
		DryRun:   true,
		Timezone: "Asia/Shanghai",
		Now:      func() time.Time { return time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC) },
	}
}

func TestIntradaySingleDate(t *testing.T) {
	opts := baseIntradayOpts()
	opts.Date = "2026-06-05"
	var gotDate int
	opts.FetchHistoryMinute = func(ctx context.Context, req tdx.HQMinuteRequest, date int, o tdx.QuoteClientOptions) ([]tdx.HQMinutePoint, error) {
		gotDate = date
		return []tdx.HQMinutePoint{
			{Market: req.Market, Symbol: req.Symbol, Time: "09:30", Index: 0, Price: 12.34, Volume: 100},
			{Market: req.Market, Symbol: req.Symbol, Time: "09:31", Index: 1, Price: 12.35, Volume: 50},
		}, nil
	}
	s, err := ImportIntradayPoints(context.Background(), opts)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if gotDate != 20260605 {
		t.Fatalf("expected date 20260605, got %d", gotDate)
	}
	if s.DatesFetched != 1 || s.RowsWritten != 2 || s.RowsSkipped != 0 {
		t.Fatalf("unexpected summary: %+v", s)
	}
}

func TestIntradayDateRange(t *testing.T) {
	opts := baseIntradayOpts()
	opts.Since = "2026-06-04"
	opts.Until = "2026-06-05"
	var dates []int
	opts.FetchHistoryMinute = func(ctx context.Context, req tdx.HQMinuteRequest, date int, o tdx.QuoteClientOptions) ([]tdx.HQMinutePoint, error) {
		dates = append(dates, date)
		return []tdx.HQMinutePoint{{Market: req.Market, Symbol: req.Symbol, Time: "09:30", Index: 0, Price: 1, Volume: 1}}, nil
	}
	s, err := ImportIntradayPoints(context.Background(), opts)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if s.DatesFetched != 2 || s.RowsWritten != 2 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	if len(dates) != 2 || dates[0] != 20260604 || dates[1] != 20260605 {
		t.Fatalf("unexpected dates: %v", dates)
	}
}

func TestIntradayToday(t *testing.T) {
	opts := baseIntradayOpts()
	opts.Today = true
	opts.FetchMinuteTime = func(ctx context.Context, req tdx.HQMinuteRequest, o tdx.QuoteClientOptions) ([]tdx.HQMinutePoint, error) {
		return []tdx.HQMinutePoint{{Market: req.Market, Symbol: req.Symbol, Time: "09:30", Index: 0, Price: 1, Volume: 1}}, nil
	}
	s, err := ImportIntradayPoints(context.Background(), opts)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if s.DatesFetched != 1 || s.RowsWritten != 1 {
		t.Fatalf("unexpected summary: %+v", s)
	}
}

func TestIntradayEmptyResponse(t *testing.T) {
	opts := baseIntradayOpts()
	opts.Date = "2026-06-05"
	opts.FetchHistoryMinute = func(ctx context.Context, req tdx.HQMinuteRequest, date int, o tdx.QuoteClientOptions) ([]tdx.HQMinutePoint, error) {
		return nil, nil
	}
	s, err := ImportIntradayPoints(context.Background(), opts)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if s.DatesFetched != 1 || s.EmptyDates != 1 || s.RowsWritten != 0 {
		t.Fatalf("unexpected summary: %+v", s)
	}
}

func TestIntradayDuplicateIdenticalPoint(t *testing.T) {
	opts := baseIntradayOpts()
	opts.Date = "2026-06-05"
	opts.FetchHistoryMinute = func(ctx context.Context, req tdx.HQMinuteRequest, date int, o tdx.QuoteClientOptions) ([]tdx.HQMinutePoint, error) {
		return []tdx.HQMinutePoint{
			{Market: req.Market, Symbol: req.Symbol, Time: "09:30", Index: 0, Price: 12.34, Volume: 100},
			{Market: req.Market, Symbol: req.Symbol, Time: "09:30", Index: 0, Price: 12.34, Volume: 100},
		}, nil
	}
	s, err := ImportIntradayPoints(context.Background(), opts)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if s.RowsWritten != 1 || s.RowsSkipped != 1 {
		t.Fatalf("expected 1 written / 1 skipped, got %+v", s)
	}
	if len(s.Issues) != 0 {
		t.Fatalf("identical duplicate should not raise a quality issue, got %+v", s.Issues)
	}
}

func TestIntradayDuplicateConflictingPoint(t *testing.T) {
	opts := baseIntradayOpts()
	opts.Date = "2026-06-05"
	opts.FetchHistoryMinute = func(ctx context.Context, req tdx.HQMinuteRequest, date int, o tdx.QuoteClientOptions) ([]tdx.HQMinutePoint, error) {
		return []tdx.HQMinutePoint{
			{Market: req.Market, Symbol: req.Symbol, Time: "09:30", Index: 0, Price: 12.34, Volume: 100},
			{Market: req.Market, Symbol: req.Symbol, Time: "09:30", Index: 0, Price: 99.99, Volume: 100},
		}, nil
	}
	s, err := ImportIntradayPoints(context.Background(), opts)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if s.RowsWritten != 1 || s.RowsSkipped != 1 {
		t.Fatalf("expected 1 written / 1 skipped, got %+v", s)
	}
	if len(s.Issues) != 1 || s.Issues[0].IssueType != "conflicting_logical_key" {
		t.Fatalf("expected conflicting_logical_key issue, got %+v", s.Issues)
	}
}
