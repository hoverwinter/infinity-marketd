package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"time"

	chstore "github.com/hoverwinter/infinity-marketd/internal/clickhouse"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

const hqDailyDataset = "a_share_bars_1d"
const hqDailyInputFormat = "tdx.hq.security_bars.day"
const maxHQDailyImportPages = 64

type FetchHQSecurityBarsFunc func(context.Context, tdx.HQBarsRequest, tdx.QuoteClientOptions) ([]tdx.HQBar, error)

type HQDailyImportOptions struct {
	Market        string
	Symbol        string
	Since         string
	Until         string
	Start         int
	Count         int
	DryRun        bool
	Store         *chstore.Store
	Timezone      string
	ClientOptions tdx.QuoteClientOptions
	FetchBars     FetchHQSecurityBarsFunc
	Now           func() time.Time
}

type HQDailySummary struct {
	RunID        string               `json:"run_id"`
	Dataset      string               `json:"dataset"`
	TargetTable  string               `json:"target_table"`
	Market       string               `json:"market"`
	Symbol       string               `json:"symbol"`
	Since        string               `json:"since,omitempty"`
	Until        string               `json:"until,omitempty"`
	Start        int                  `json:"start"`
	Count        int                  `json:"count"`
	PagesFetched uint64               `json:"pages_fetched"`
	RowsFetched  uint64               `json:"rows_fetched"`
	RowsWritten  uint64               `json:"rows_written"`
	RowsSkipped  uint64               `json:"rows_skipped"`
	Issues       []model.QualityIssue `json:"issues"`
	DryRun       bool                 `json:"dry_run"`
}

func ImportHQDailyBars(ctx context.Context, opts HQDailyImportOptions) (HQDailySummary, error) {
	if opts.Timezone == "" {
		opts.Timezone = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(opts.Timezone)
	if err != nil {
		return HQDailySummary{}, err
	}
	if opts.FetchBars == nil {
		opts.FetchBars = tdx.FetchHQSecurityBars
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	req, err := normalizeHQDailyRequest(opts)
	if err != nil {
		return HQDailySummary{}, err
	}
	since, hasSince, err := parseOnlineDailyDateBound(opts.Since, loc)
	if err != nil {
		return HQDailySummary{}, fmt.Errorf("parse --since: %w", err)
	}
	until, hasUntil, err := parseOnlineDailyDateBound(opts.Until, loc)
	if err != nil {
		return HQDailySummary{}, fmt.Errorf("parse --until: %w", err)
	}
	if hasSince && hasUntil && since.After(until) {
		return HQDailySummary{}, fmt.Errorf("--since must be <= --until")
	}
	if !opts.DryRun && opts.Store == nil {
		return HQDailySummary{}, fmt.Errorf("store is required when dry-run is false")
	}

	summary := HQDailySummary{
		Dataset:     hqDailyDataset,
		TargetTable: hqDailyDataset,
		Market:      req.Market,
		Symbol:      req.Symbol,
		Since:       opts.Since,
		Until:       opts.Until,
		Start:       req.Start,
		Count:       req.Count,
		DryRun:      opts.DryRun,
	}

	produce := func(ctx context.Context, runID string) ([]model.DailyBar, uint64, []model.QualityIssue, error) {
		rows, pagesFetched, rowsFetched, err := fetchHQDailyPages(ctx, req, opts.ClientOptions, opts.FetchBars)
		summary.PagesFetched = pagesFetched
		summary.RowsFetched = rowsFetched
		if err != nil {
			return nil, 0, nil, err
		}
		normalized, skipped, issues := normalizeHQDailyBars(runID, rows, req, loc, since, hasSince, until, hasUntil, opts.Now)
		if len(normalized) == 0 {
			issues = append(issues, zeroRowsIssue(runID, hqDailyDataset, hqDailyInputFormat, req.Market, req.Symbol))
		}
		return normalized, skipped, issues, nil
	}

	params, _ := json.Marshal(map[string]any{
		"market": req.Market,
		"symbol": req.Symbol,
		"since":  opts.Since,
		"until":  opts.Until,
		"start":  req.Start,
		"count":  req.Count,
	})
	var ops OnlineOps
	if opts.Store != nil {
		ops = opts.Store
	}
	result, err := RunOnlineJob(ctx, OnlineJob[model.DailyBar]{
		Dataset:     hqDailyDataset,
		TargetTable: hqDailyDataset,
		TaskType:    "tdx_hq_daily_import",
		InputFormat: hqDailyInputFormat,
		Asset:       fmt.Sprintf("%s:%s", req.Market, req.Symbol),
		Params:      string(params),
		DryRun:      opts.DryRun,
		Ops:         ops,
		Now:         opts.Now,
		Produce:     produce,
		Write: func(ctx context.Context, rows []model.DailyBar) error {
			return opts.Store.InsertDailyBars(ctx, rows)
		},
		Bounds: dailyWatermarks,
	})
	summary.RunID = result.RunID
	summary.RowsWritten = result.RowsWritten
	summary.RowsSkipped = result.RowsSkipped
	summary.Issues = result.Issues
	if err != nil {
		return summary, err
	}
	return summary, nil
}

func normalizeHQDailyRequest(opts HQDailyImportOptions) (tdx.HQBarsRequest, error) {
	count := opts.Count
	if count == 0 {
		count = tdx.MaxHQKLineCount
	}
	if count <= 0 || count > tdx.MaxHQKLineCount*maxHQDailyImportPages {
		return tdx.HQBarsRequest{}, fmt.Errorf("online daily import count must be between 1 and %d", tdx.MaxHQKLineCount*maxHQDailyImportPages)
	}
	pageCount := count
	if pageCount > tdx.MaxHQKLineCount {
		pageCount = tdx.MaxHQKLineCount
	}
	req, err := tdx.ParseHQBarsRequest(tdx.HQKLineDayAlt, opts.Market, opts.Symbol, opts.Start, pageCount)
	if err != nil {
		return tdx.HQBarsRequest{}, err
	}
	req.Count = count
	return req, nil
}

func fetchHQDailyPages(ctx context.Context, req tdx.HQBarsRequest, clientOpts tdx.QuoteClientOptions, fetch FetchHQSecurityBarsFunc) ([]tdx.HQBar, uint64, uint64, error) {
	remaining := req.Count
	start := req.Start
	var out []tdx.HQBar
	var pages uint64
	var rowsFetched uint64
	for remaining > 0 {
		if pages >= maxHQDailyImportPages {
			return nil, pages, rowsFetched, fmt.Errorf("online daily import exceeded max page count %d", maxHQDailyImportPages)
		}
		pageCount := remaining
		if pageCount > tdx.MaxHQKLineCount {
			pageCount = tdx.MaxHQKLineCount
		}
		pageReq, err := tdx.ParseHQBarsRequest(tdx.HQKLineDayAlt, req.Market, req.Symbol, start, pageCount)
		if err != nil {
			return nil, pages, rowsFetched, err
		}
		rows, err := fetch(ctx, pageReq, clientOpts)
		if err != nil {
			return nil, pages, rowsFetched, err
		}
		pages++
		rowsFetched += uint64(len(rows))
		out = append(out, rows...)
		if len(rows) < pageCount {
			break
		}
		start += pageCount
		remaining -= pageCount
	}
	return out, pages, rowsFetched, nil
}

func normalizeHQDailyBars(runID string, rows []tdx.HQBar, req tdx.HQBarsRequest, loc *time.Location, since time.Time, hasSince bool, until time.Time, hasUntil bool, now func() time.Time) ([]model.DailyBar, uint64, []model.QualityIssue) {
	seen := map[string]model.DailyBar{}
	out := make([]model.DailyBar, 0, len(rows))
	var skipped uint64
	var issues []model.QualityIssue
	for _, row := range rows {
		bar, key, err := dailyBarFromHQ(row, req, loc)
		if err != nil {
			skipped++
			issues = append(issues, hqDailyIssue(runID, "invalid_provider_row", req.Market, req.Symbol, key, err.Error(), "warning", now))
			continue
		}
		if hasSince && bar.TradeDate.Before(since) {
			skipped++
			continue
		}
		if hasUntil && bar.TradeDate.After(until) {
			skipped++
			continue
		}
		key = hqDailyKey(bar)
		if prev, ok := seen[key]; ok {
			skipped++
			issueType := "duplicate_logical_key"
			severity := "warning"
			message := "duplicate online daily logical key"
			if !equalDailyBar(prev, bar) {
				issueType = "conflicting_logical_key"
				severity = "error"
				message = "conflicting online daily logical key"
			}
			issues = append(issues, hqDailyIssue(runID, issueType, req.Market, req.Symbol, key, message, severity, now))
			continue
		}
		seen[key] = bar
		out = append(out, bar)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TradeDate.Before(out[j].TradeDate) })
	return out, skipped, issues
}

func dailyBarFromHQ(row tdx.HQBar, req tdx.HQBarsRequest, loc *time.Location) (model.DailyBar, string, error) {
	key := strings.TrimSpace(row.DateTime)
	if row.Market != "" && row.Market != req.Market {
		return model.DailyBar{}, key, fmt.Errorf("provider market mismatch: got %q want %q", row.Market, req.Market)
	}
	if row.Symbol != "" && row.Symbol != req.Symbol {
		return model.DailyBar{}, key, fmt.Errorf("provider symbol mismatch: got %q want %q", row.Symbol, req.Symbol)
	}
	tradeDate, err := hqDailyTradeDate(row, loc)
	if err != nil {
		return model.DailyBar{}, key, err
	}
	if !finiteNumbers(row.Open, row.High, row.Low, row.Close, row.Volume, row.Amount) {
		return model.DailyBar{}, tradeDate.Format("2006-01-02"), fmt.Errorf("non-finite numeric value")
	}
	if row.High < row.Low {
		return model.DailyBar{}, tradeDate.Format("2006-01-02"), fmt.Errorf("high < low")
	}
	if row.Volume < 0 || row.Volume > float64(math.MaxUint64) {
		return model.DailyBar{}, tradeDate.Format("2006-01-02"), fmt.Errorf("invalid volume")
	}
	if row.Amount < 0 {
		return model.DailyBar{}, tradeDate.Format("2006-01-02"), fmt.Errorf("negative amount")
	}
	return model.DailyBar{
		Market:    req.Market,
		Symbol:    req.Symbol,
		TradeDate: tradeDate,
		Open:      row.Open,
		High:      row.High,
		Low:       row.Low,
		Close:     row.Close,
		Volume:    uint64(row.Volume),
		Amount:    row.Amount,
	}, tradeDate.Format("2006-01-02"), nil
}

func hqDailyTradeDate(row tdx.HQBar, loc *time.Location) (time.Time, error) {
	if row.Year > 0 && row.Month > 0 && row.Day > 0 {
		if row.Month < 1 || row.Month > 12 {
			return time.Time{}, fmt.Errorf("invalid trade date")
		}
		t := time.Date(row.Year, time.Month(row.Month), row.Day, 0, 0, 0, 0, loc)
		if t.Year() != row.Year || int(t.Month()) != row.Month || t.Day() != row.Day {
			return time.Time{}, fmt.Errorf("invalid trade date")
		}
		return t, nil
	}
	if len(row.DateTime) >= 10 {
		t, err := time.ParseInLocation("2006-01-02", row.DateTime[:10], loc)
		if err != nil {
			return time.Time{}, err
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf("missing trade date")
}

func finiteNumbers(values ...float64) bool {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

func parseOnlineDailyDateBound(value string, loc *time.Location) (time.Time, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false, nil
	}
	for _, layout := range []string{"2006-01-02", "20060102"} {
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			return t, true, nil
		}
	}
	return time.Time{}, true, fmt.Errorf("invalid date %q", value)
}

func hqDailyKey(bar model.DailyBar) string {
	return fmt.Sprintf("%s:%s:%s", bar.Market, bar.Symbol, bar.TradeDate.Format("2006-01-02"))
}

func equalDailyBar(a, b model.DailyBar) bool {
	return a.Market == b.Market &&
		a.Symbol == b.Symbol &&
		a.TradeDate.Equal(b.TradeDate) &&
		a.Open == b.Open &&
		a.High == b.High &&
		a.Low == b.Low &&
		a.Close == b.Close &&
		a.Volume == b.Volume &&
		a.Amount == b.Amount
}

func hqDailyIssue(runID, issueType, market, symbol, logicalKey, message, severity string, now func() time.Time) model.QualityIssue {
	if now == nil {
		now = time.Now
	}
	return model.QualityIssue{
		RunID:      runID,
		Dataset:    hqDailyDataset,
		Severity:   severity,
		IssueType:  issueType,
		Market:     market,
		Symbol:     symbol,
		LogicalKey: logicalKey,
		InputPath:  filepath.Base(hqDailyInputFormat),
		ObservedAt: now(),
		Message:    message,
	}
}
