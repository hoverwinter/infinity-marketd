package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

type LimitIndexImportOptions struct {
	IndexCode       string
	Since           string
	Until           string
	DryRun          bool
	Store           LimitReviewWriter
	ClientOptions   tdx.QuoteClientOptions
	FetchBars       func(context.Context, tdx.HQBarsRequest, tdx.QuoteClientOptions) ([]tdx.HQBar, error)
	FetchSecurities func(context.Context, string, tdx.QuoteClientOptions) ([]tdx.Security, error)
	Now             func() time.Time
}

func tdxLimitIndex(code string) (string, string, error) {
	switch code {
	case "prev_limit_up_perf":
		return "880863", "昨日涨停", nil
	case "prev_ladder_perf":
		return "880812", "昨日连板", nil
	case "prev_limit_down_perf":
		return "880751", "昨日跌停", nil
	default:
		return "", "", fmt.Errorf("index %q has no verified TDX mapping; non-ST must not be substituted by all-limit-up", code)
	}
}

func ImportTDXLimitIndex(ctx context.Context, opts LimitIndexImportOptions) (NormalizedReviewImportSummary, error) {
	const dataset = "a_share_limit_performance_index_bars_1d"
	symbol, name, err := tdxLimitIndex(opts.IndexCode)
	if err != nil {
		return NormalizedReviewImportSummary{}, err
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	since, err := parseReviewDate(opts.Since, loc)
	if err != nil {
		return NormalizedReviewImportSummary{}, err
	}
	until, err := parseReviewDate(opts.Until, loc)
	if err != nil {
		return NormalizedReviewImportSummary{}, err
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if since.After(until) || until.After(opts.Now().In(loc)) {
		return NormalizedReviewImportSummary{}, fmt.Errorf("invalid index date range")
	}
	if opts.FetchBars == nil {
		opts.FetchBars = tdx.FetchHQIndexBars
	}
	if opts.FetchSecurities == nil {
		opts.FetchSecurities = tdx.FetchSecurityList
	}
	params, _ := json.Marshal(map[string]any{"index_code": opts.IndexCode, "market": "sh", "symbol": symbol, "verified_name": name, "since": opts.Since, "until": opts.Until, "provider": "tdx", "volume_unit": "hand"})
	result, err := RunOnlineJob(ctx, OnlineJob[model.LimitPerformanceIndexBar]{Dataset: dataset, TargetTable: dataset, TaskType: "tdx_limit_index_import", InputFormat: "tdx.hq.index_bars", Asset: opts.IndexCode, Params: string(params), DryRun: opts.DryRun, Ops: opts.Store, Now: opts.Now,
		Produce: func(ctx context.Context, runID string) ([]model.LimitPerformanceIndexBar, uint64, []model.QualityIssue, error) {
			securities, err := opts.FetchSecurities(ctx, "sh", opts.ClientOptions)
			if err != nil {
				return nil, 0, nil, err
			}
			verified := false
			for _, s := range securities {
				if s.Symbol == symbol {
					if s.Name != name {
						return nil, 0, nil, fmt.Errorf("TDX %s identity changed: got %q, want %q", symbol, s.Name, name)
					}
					verified = true
					break
				}
			}
			if !verified {
				return nil, 0, nil, fmt.Errorf("TDX %s identity not found", symbol)
			}
			seen := map[string]bool{}
			rows := []model.LimitPerformanceIndexBar{}
			var earliest time.Time
			complete := false
			for start := 0; start < 64000; start += 800 {
				bars, err := opts.FetchBars(ctx, tdx.HQBarsRequest{Market: "sh", Symbol: symbol, Category: tdx.HQKLineDayAlt, Start: start, Count: 800}, opts.ClientOptions)
				if err != nil {
					return nil, 0, nil, err
				}
				if len(bars) == 0 {
					complete = true
					break
				}
				if len(bars) > 800 {
					return nil, 0, nil, fmt.Errorf("TDX index page exceeds requested count")
				}
				pageEarliest := time.Time{}
				previousEarliest := earliest
				for _, bar := range bars {
					if bar.Market != "sh" || bar.Symbol != symbol || bar.Category != tdx.HQKLineDayAlt {
						return nil, 0, nil, fmt.Errorf("TDX index response identity mismatch")
					}
					date, err := time.ParseInLocation("2006-01-02 15:04", bar.DateTime, loc)
					if err != nil {
						return nil, 0, nil, err
					}
					date = dateOnly(date, loc)
					if !previousEarliest.IsZero() && !date.Before(previousEarliest) {
						return nil, 0, nil, fmt.Errorf("TDX index pages are not strictly moving backwards")
					}
					key := date.Format("2006-01-02")
					if seen[key] {
						return nil, 0, nil, fmt.Errorf("TDX repeated index date %s", key)
					}
					seen[key] = true
					if pageEarliest.IsZero() || date.Before(pageEarliest) {
						pageEarliest = date
					}
					if earliest.IsZero() || date.Before(earliest) {
						earliest = date
					}
					if date.Before(since) || date.After(until) {
						continue
					}
					if key == opts.Now().In(loc).Format("2006-01-02") && opts.Now().In(loc).Before(date.Add(15*time.Hour+5*time.Minute)) {
						return nil, 0, nil, fmt.Errorf("index day has not closed")
					}
					if !finiteNumbers(bar.Open, bar.High, bar.Low, bar.Close, bar.Volume, bar.Amount) || bar.Low <= 0 || bar.High < bar.Open || bar.High < bar.Close || bar.Low > bar.Open || bar.Low > bar.Close || bar.Amount < 0 || bar.Volume < 0 || bar.Volume >= math.MaxUint64 {
						return nil, 0, nil, fmt.Errorf("invalid index OHLCV on %s", key)
					}
					volume := uint64(math.Round(bar.Volume))
					amount := bar.Amount
					rows = append(rows, model.LimitPerformanceIndexBar{TradeDate: date, IndexCode: opts.IndexCode, Open: bar.Open, High: bar.High, Low: bar.Low, Close: bar.Close, Volume: &volume, Amount: &amount})
				}
				if !pageEarliest.After(since) || len(bars) < 800 {
					complete = true
					break
				}
			}
			if !complete {
				return nil, 0, nil, fmt.Errorf("TDX index pagination limit reached")
			}
			if len(rows) == 0 {
				return nil, 0, nil, fmt.Errorf("no index bars in requested range")
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].TradeDate.Before(rows[j].TradeDate) })
			issues := []model.QualityIssue{}
			if earliest.After(since) {
				issues = append(issues, model.QualityIssue{RunID: runID, Dataset: dataset, Severity: "warning", IssueType: "index_history_starts_late", Market: "sh", Symbol: symbol, LogicalKey: opts.IndexCode, ObservedAt: opts.Now(), Message: "provider history begins at " + earliest.Format("2006-01-02") + "; earlier coverage is not proven"})
			}
			return rows, 0, issues, nil
		},
		Write: func(ctx context.Context, rows []model.LimitPerformanceIndexBar) error {
			return opts.Store.InsertLimitPerformanceIndexBars(ctx, rows)
		},
		Bounds: performanceWatermarks,
	})
	return NormalizedReviewImportSummary{RunID: result.RunID, Dataset: result.Dataset, TargetTable: result.TargetTable, RowsWritten: result.RowsWritten, RowsSkipped: result.RowsSkipped, Issues: result.Issues, DryRun: result.DryRun}, err
}
