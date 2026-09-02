package ingest

import (
	"context"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

func TestImportHQDailyBarsDryRun(t *testing.T) {
	summary, err := ImportHQDailyBars(context.Background(), HQDailyImportOptions{
		Market: "sh", Symbol: "600519", Since: "2026-06-05", Until: "2026-06-05", DryRun: true,
		FetchBars: func(_ context.Context, req tdx.HQBarsRequest, _ tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
			if req.Category != tdx.HQKLineDayAlt || req.Start != 0 || req.Count != tdx.MaxHQKLineCount {
				t.Fatalf("req=%+v", req)
			}
			return []tdx.HQBar{
				testHQDailyBar(req.Market, req.Symbol, 2026, 6, 4, 10),
				testHQDailyBar(req.Market, req.Symbol, 2026, 6, 5, 11),
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !summary.DryRun || summary.TargetTable != "a_share_bars_1d" || summary.RowsWritten != 1 || summary.RowsSkipped != 1 || summary.PagesFetched != 1 || summary.RowsFetched != 2 {
		t.Fatalf("summary=%+v", summary)
	}
	if len(summary.Issues) != 0 {
		t.Fatalf("issues=%+v", summary.Issues)
	}
}

func TestImportHQDailyBarsPagesProviderWindow(t *testing.T) {
	var starts []int
	var counts []int
	summary, err := ImportHQDailyBars(context.Background(), HQDailyImportOptions{
		Market: "sh", Symbol: "600519", Count: tdx.MaxHQKLineCount + 1, DryRun: true,
		FetchBars: func(_ context.Context, req tdx.HQBarsRequest, _ tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
			starts = append(starts, req.Start)
			counts = append(counts, req.Count)
			if req.Start == 0 {
				rows := make([]tdx.HQBar, 0, tdx.MaxHQKLineCount)
				for i := 0; i < tdx.MaxHQKLineCount; i++ {
					rows = append(rows, testHQDailyBarAt(req.Market, req.Symbol, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i), 10))
				}
				return rows, nil
			}
			return []tdx.HQBar{testHQDailyBar(req.Market, req.Symbol, 2026, 6, 5, 20)}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Trim(strings.Join(intsToStrings(starts), ","), ",") != "0,800" || strings.Join(intsToStrings(counts), ",") != "800,1" {
		t.Fatalf("starts=%v counts=%v", starts, counts)
	}
	if summary.PagesFetched != 2 || summary.RowsFetched != 801 || summary.RowsWritten != 801 {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestImportHQDailyBarsEmptyResultCreatesIssue(t *testing.T) {
	summary, err := ImportHQDailyBars(context.Background(), HQDailyImportOptions{
		Market: "sh", Symbol: "600519", DryRun: true,
		FetchBars: func(context.Context, tdx.HQBarsRequest, tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
			return nil, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.RowsWritten != 0 || len(summary.Issues) != 1 || summary.Issues[0].IssueType != "zero_valid_rows" {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestImportHQDailyBarsInvalidAndDuplicateRows(t *testing.T) {
	summary, err := ImportHQDailyBars(context.Background(), HQDailyImportOptions{
		Market: "sh", Symbol: "600519", DryRun: true,
		FetchBars: func(_ context.Context, req tdx.HQBarsRequest, _ tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
			return []tdx.HQBar{
				testHQDailyBar(req.Market, req.Symbol, 2026, 6, 5, 10),
				testHQDailyBar(req.Market, req.Symbol, 2026, 6, 5, 10),
				testHQDailyBar(req.Market, req.Symbol, 2026, 6, 6, 11),
				testHQDailyBar(req.Market, req.Symbol, 2026, 6, 6, 12),
				{Market: req.Market, Symbol: req.Symbol, Year: 2026, Month: 6, Day: 7, Open: 10, High: 9, Low: 11, Close: 10, Volume: 1, Amount: 1},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.RowsWritten != 2 || summary.RowsSkipped != 3 || len(summary.Issues) != 3 {
		t.Fatalf("summary=%+v", summary)
	}
	types := []string{summary.Issues[0].IssueType, summary.Issues[1].IssueType, summary.Issues[2].IssueType}
	if strings.Join(types, ",") != "duplicate_logical_key,conflicting_logical_key,invalid_provider_row" {
		t.Fatalf("issue types=%v", types)
	}
}

func TestImportHQDailyBarsRejectsMalformedProviderValues(t *testing.T) {
	summary, err := ImportHQDailyBars(context.Background(), HQDailyImportOptions{
		Market: "sh", Symbol: "600519", DryRun: true,
		FetchBars: func(_ context.Context, req tdx.HQBarsRequest, _ tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
			badDate := testHQDailyBar(req.Market, req.Symbol, 2026, 6, 5, 10)
			badDate.Month = 13
			badNumber := testHQDailyBar(req.Market, req.Symbol, 2026, 6, 6, 11)
			badNumber.Close = math.NaN()
			return []tdx.HQBar{badDate, badNumber}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.RowsWritten != 0 || summary.RowsSkipped != 2 || len(summary.Issues) != 3 {
		t.Fatalf("summary=%+v", summary)
	}
	if summary.Issues[0].IssueType != "invalid_provider_row" || summary.Issues[1].IssueType != "invalid_provider_row" || summary.Issues[2].IssueType != "zero_valid_rows" {
		t.Fatalf("issues=%+v", summary.Issues)
	}
}

func TestImportHQDailyBarsRejectsInvalidBounds(t *testing.T) {
	_, err := ImportHQDailyBars(context.Background(), HQDailyImportOptions{Market: "sh", Symbol: "600519", Since: "2026-06-06", Until: "2026-06-05", DryRun: true})
	if err == nil || !strings.Contains(err.Error(), "--since must be <= --until") {
		t.Fatalf("err=%v", err)
	}
}

func testHQDailyBar(market, symbol string, year, month, day int, close float64) tdx.HQBar {
	return testHQDailyBarAt(market, symbol, time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), close)
}

func testHQDailyBarAt(market, symbol string, date time.Time, close float64) tdx.HQBar {
	return tdx.HQBar{
		Market:   market,
		Symbol:   symbol,
		Category: tdx.HQKLineDayAlt,
		DateTime: date.Format("2006-01-02 15:04"),
		Year:     date.Year(),
		Month:    int(date.Month()),
		Day:      date.Day(),
		Open:     close - 1,
		High:     close + 1,
		Low:      close - 2,
		Close:    close,
		Volume:   100,
		Amount:   1000,
	}
}

func intsToStrings(values []int) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, strconv.Itoa(value))
	}
	return out
}
