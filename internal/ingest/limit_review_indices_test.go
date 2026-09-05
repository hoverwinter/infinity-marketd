package ingest

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

func reviewIndexOptions() LimitIndexImportOptions {
	return LimitIndexImportOptions{IndexCode: "prev_ladder_perf", Since: "2016-01-01", Until: "2026-09-04", DryRun: true,
		Now: func() time.Time { return time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC) },
		FetchSecurities: func(context.Context, string, tdx.QuoteClientOptions) ([]tdx.Security, error) {
			return []tdx.Security{{Symbol: "880812", Name: "昨日连板"}}, nil
		},
	}
}

func reviewIndexBar(date string) tdx.HQBar {
	return tdx.HQBar{Market: "sh", Symbol: "880812", Category: tdx.HQKLineDayAlt, DateTime: date + " 15:00", Open: 100, High: 102, Low: 99, Close: 101, Volume: 100, Amount: 10000}
}

func TestLimitIndexPaginationAndCoverage(t *testing.T) {
	for _, dry := range []bool{true, false} {
		t.Run(fmt.Sprint(dry), func(t *testing.T) {
			opts := reviewIndexOptions()
			store := &reviewMemoryStore{}
			opts.DryRun, opts.Store = dry, store
			calls := 0
			opts.FetchBars = func(_ context.Context, q tdx.HQBarsRequest, _ tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
				if q.Start != calls*800 || q.Count != 800 || q.Symbol != "880812" {
					t.Fatalf("%+v", q)
				}
				calls++
				if calls == 2 {
					return []tdx.HQBar{reviewIndexBar("2020-01-01")}, nil
				}
				end := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
				rows := []tdx.HQBar{}
				for i := 799; i >= 0; i-- {
					rows = append(rows, reviewIndexBar(end.AddDate(0, 0, -i).Format("2006-01-02")))
				}
				return rows, nil
			}
			out, err := ImportTDXLimitIndex(context.Background(), opts)
			if err != nil || calls != 2 || out.RowsWritten != 801 || len(out.Issues) != 1 || out.Issues[0].IssueType != "index_history_starts_late" {
				t.Fatalf("%+v %v calls=%d", out, err, calls)
			}
			if dry && len(store.runs) != 0 {
				t.Fatal("dry run wrote ops")
			}
			if !dry && (len(store.runs) != 1 || store.runs[0].Status != "degraded" || store.runs[0].RowsWritten != 801) {
				t.Fatal(store.runs)
			}
		})
	}
}

func TestLimitIndexRejectsUnverifiedAndMalformed(t *testing.T) {
	for _, kind := range []string{"non-st", "wrong-name", "wrong-symbol", "duplicate", "bad-price", "no-coverage", "unclosed"} {
		t.Run(kind, func(t *testing.T) {
			opts := reviewIndexOptions()
			opts.FetchBars = func(context.Context, tdx.HQBarsRequest, tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
				bar := reviewIndexBar("2026-09-04")
				switch kind {
				case "wrong-symbol":
					bar.Symbol = "880864"
				case "duplicate":
					return []tdx.HQBar{bar, bar}, nil
				case "bad-price":
					bar.High = 90
				case "no-coverage":
					return nil, nil
				}
				return []tdx.HQBar{bar}, nil
			}
			if kind == "non-st" {
				opts.IndexCode = "prev_non_st_limit_up_perf"
			}
			if kind == "wrong-name" {
				opts.FetchSecurities = func(context.Context, string, tdx.QuoteClientOptions) ([]tdx.Security, error) {
					return []tdx.Security{{Symbol: "880812", Name: "昨日振荡"}}, nil
				}
			}
			if kind == "unclosed" {
				opts.Now = func() time.Time { return time.Date(2026, 9, 4, 1, 0, 0, 0, time.UTC) }
			}
			if _, err := ImportTDXLimitIndex(context.Background(), opts); err == nil {
				t.Fatal("accepted invalid index input")
			}
		})
	}
	if code, name, err := tdxLimitIndex("prev_limit_down_perf"); err != nil || code != "880751" || !strings.Contains(name, "跌停") {
		t.Fatalf("%s %s %v", code, name, err)
	}
}
