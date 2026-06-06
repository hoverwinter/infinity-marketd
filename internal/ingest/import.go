package ingest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	chstore "github.com/hoverwinter/infinity-marketd/internal/clickhouse"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

type ImportOptions struct {
	Period    tdx.Period
	File      string
	Root      string
	Code      string
	Market    string
	Since     string
	Until     string
	DryRun    bool
	Store     *chstore.Store
	Timezone  string
	BatchSize int
}

type Summary struct {
	RunID       string
	Dataset     string
	TargetTable string
	InputPath   string
	InputFormat string
	RowsWritten uint64
	RowsSkipped uint64
	Issues      []model.QualityIssue
	DryRun      bool
}

func Import(ctx context.Context, opts ImportOptions) (Summary, error) {
	if opts.Timezone == "" {
		opts.Timezone = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(opts.Timezone)
	if err != nil {
		return Summary{}, err
	}
	runID := newRunID()
	started := time.Now()

	path, market, symbol, err := resolveInput(opts)
	if err != nil {
		return Summary{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Summary{}, err
	}

	summary := Summary{
		RunID:       runID,
		Dataset:     datasetFor(opts.Period),
		TargetTable: targetTableFor(opts.Period),
		InputPath:   path,
		DryRun:      opts.DryRun,
	}
	var minWM, maxWM *time.Time
	var rowsSkipped uint64

	switch opts.Period {
	case tdx.PeriodDay:
		result := tdx.ParseDayBytes(raw, path, market, symbol, loc)
		bars, skipped, err := filterDaily(result.Bars, opts.Since, opts.Until, loc)
		if err != nil {
			return Summary{}, err
		}
		rowsSkipped += skipped
		issues := issuesFromParse(runID, summary.Dataset, path, market, symbol, result.Issues)
		if len(bars) == 0 {
			issues = append(issues, zeroRowsIssue(runID, summary.Dataset, path, market, symbol))
		}
		summary.Issues = issues
		summary.InputFormat = "tdx.day.<IIIIIfII>"
		summary.RowsWritten = uint64(len(bars))
		summary.RowsSkipped = rowsSkipped
		minWM, maxWM = dailyWatermarks(bars)
		if !opts.DryRun {
			if err := opts.Store.InsertDailyBars(ctx, bars); err != nil {
				recordFailure(ctx, opts.Store, summary, started, err)
				return summary, err
			}
		}
	case tdx.Period1m, tdx.Period5m:
		result := tdx.ParseMinuteBytes(raw, path, market, symbol, opts.Period, loc)
		bars, skipped, err := filterMinute(result.Bars, opts.Since, opts.Until, loc)
		if err != nil {
			return Summary{}, err
		}
		rowsSkipped += skipped
		issues := issuesFromParse(runID, summary.Dataset, path, market, symbol, result.Issues)
		if len(bars) == 0 {
			issues = append(issues, zeroRowsIssue(runID, summary.Dataset, path, market, symbol))
		}
		summary.Issues = issues
		summary.InputFormat = result.Format
		summary.RowsWritten = uint64(len(bars))
		summary.RowsSkipped = rowsSkipped
		minWM, maxWM = minuteWatermarks(bars)
		if !opts.DryRun {
			if err := opts.Store.InsertMinuteBars(ctx, summary.TargetTable, bars); err != nil {
				recordFailure(ctx, opts.Store, summary, started, err)
				return summary, err
			}
		}
	default:
		return Summary{}, fmt.Errorf("unsupported period %q", opts.Period)
	}

	if opts.DryRun {
		return summary, nil
	}
	if opts.Store == nil {
		return Summary{}, fmt.Errorf("store is required when dry-run is false")
	}
	if err := opts.Store.InsertQualityIssues(ctx, summary.Issues); err != nil {
		recordFailure(ctx, opts.Store, summary, started, err)
		return summary, err
	}
	now := time.Now()
	status := "success"
	message := "ok"
	if len(summary.Issues) > 0 {
		status = "degraded"
		message = fmt.Sprintf("%d quality issue(s)", len(summary.Issues))
	}
	if err := opts.Store.InsertWatermark(ctx, model.Watermark{
		Dataset:      summary.Dataset,
		Asset:        fmt.Sprintf("%s:%s", market, symbol),
		Status:       status,
		MinWatermark: minWM,
		MaxWatermark: maxWM,
		RowsWritten:  summary.RowsWritten,
		Message:      message,
		UpdatedAt:    now,
	}); err != nil {
		recordFailure(ctx, opts.Store, summary, started, err)
		return summary, err
	}
	if err := recordRun(ctx, opts.Store, summary, started, nil, status); err != nil {
		return summary, err
	}
	return summary, nil
}

func resolveInput(opts ImportOptions) (string, string, string, error) {
	path := opts.File
	if path == "" {
		if opts.Code == "" {
			return "", "", "", fmt.Errorf("either --file or --code is required")
		}
		market := opts.Market
		if market == "" {
			market = tdx.InferMarketFromCode(opts.Code)
		}
		discovered, err := tdx.DiscoverFile(expandHome(opts.Root), opts.Period, market, opts.Code)
		if err != nil {
			return "", "", "", err
		}
		path = discovered
	}
	path = expandHome(path)
	market, symbol, err := tdx.ParseMarketSymbol(path, opts.Market, opts.Code)
	if err != nil {
		return "", "", "", err
	}
	return path, market, symbol, nil
}

func filterDaily(bars []model.DailyBar, sinceValue string, untilValue string, loc *time.Location) ([]model.DailyBar, uint64, error) {
	since, hasSince, err := parseDateBound(sinceValue, false, loc)
	if err != nil {
		return nil, 0, err
	}
	until, hasUntil, err := parseDateBound(untilValue, true, loc)
	if err != nil {
		return nil, 0, err
	}
	var out []model.DailyBar
	var skipped uint64
	for _, bar := range bars {
		if hasSince && bar.TradeDate.Before(since) {
			skipped++
			continue
		}
		if hasUntil && bar.TradeDate.After(until) {
			skipped++
			continue
		}
		out = append(out, bar)
	}
	return out, skipped, nil
}

func filterMinute(bars []model.MinuteBar, sinceValue string, untilValue string, loc *time.Location) ([]model.MinuteBar, uint64, error) {
	since, hasSince, err := parseTimeBound(sinceValue, false, loc)
	if err != nil {
		return nil, 0, err
	}
	until, hasUntil, err := parseTimeBound(untilValue, true, loc)
	if err != nil {
		return nil, 0, err
	}
	var out []model.MinuteBar
	var skipped uint64
	for _, bar := range bars {
		if hasSince && bar.BarTime.Before(since) {
			skipped++
			continue
		}
		if hasUntil && bar.BarTime.After(until) {
			skipped++
			continue
		}
		out = append(out, bar)
	}
	return out, skipped, nil
}

func parseDateBound(value string, endOfDay bool, loc *time.Location) (time.Time, bool, error) {
	if value == "" {
		return time.Time{}, false, nil
	}
	t, err := time.ParseInLocation("2006-01-02", value, loc)
	if err != nil {
		return time.Time{}, false, err
	}
	if endOfDay {
		t = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, loc)
	}
	return t, true, nil
}

func parseTimeBound(value string, endOfDay bool, loc *time.Location) (time.Time, bool, error) {
	if value == "" {
		return time.Time{}, false, nil
	}
	layouts := []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			if layout == "2006-01-02" && endOfDay {
				t = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, loc)
			}
			return t, true, nil
		}
	}
	return time.Time{}, false, fmt.Errorf("invalid time bound %q", value)
}

func issuesFromParse(runID string, dataset string, path string, market string, symbol string, parseIssues []tdx.ParseIssue) []model.QualityIssue {
	now := time.Now()
	out := make([]model.QualityIssue, 0, len(parseIssues))
	for _, issue := range parseIssues {
		out = append(out, model.QualityIssue{
			RunID:             runID,
			Dataset:           dataset,
			Severity:          severityFor(issue.Type),
			IssueType:         issue.Type,
			Market:            market,
			Symbol:            symbol,
			LogicalKey:        issue.LogicalKey,
			InputPath:         path,
			InputRecordOffset: issue.Offset,
			ObservedAt:        now,
			Message:           issue.Message,
			Details:           "",
		})
	}
	return out
}

func zeroRowsIssue(runID string, dataset string, path string, market string, symbol string) model.QualityIssue {
	return model.QualityIssue{
		RunID:      runID,
		Dataset:    dataset,
		Severity:   "error",
		IssueType:  "zero_valid_rows",
		Market:     market,
		Symbol:     symbol,
		LogicalKey: filepath.Base(path),
		InputPath:  path,
		ObservedAt: time.Now(),
		Message:    "import produced zero valid rows",
	}
}

func dailyWatermarks(bars []model.DailyBar) (*time.Time, *time.Time) {
	if len(bars) == 0 {
		return nil, nil
	}
	min := bars[0].TradeDate
	max := bars[0].TradeDate
	for _, bar := range bars[1:] {
		if bar.TradeDate.Before(min) {
			min = bar.TradeDate
		}
		if bar.TradeDate.After(max) {
			max = bar.TradeDate
		}
	}
	return &min, &max
}

func minuteWatermarks(bars []model.MinuteBar) (*time.Time, *time.Time) {
	if len(bars) == 0 {
		return nil, nil
	}
	min := bars[0].BarTime
	max := bars[0].BarTime
	for _, bar := range bars[1:] {
		if bar.BarTime.Before(min) {
			min = bar.BarTime
		}
		if bar.BarTime.After(max) {
			max = bar.BarTime
		}
	}
	return &min, &max
}

func recordFailure(ctx context.Context, store *chstore.Store, summary Summary, started time.Time, failure error) {
	_ = recordRun(ctx, store, summary, started, failure, "failed")
}

func recordRun(ctx context.Context, store *chstore.Store, summary Summary, started time.Time, failure error, status string) error {
	if store == nil {
		return nil
	}
	finished := time.Now()
	duration := uint64(finished.Sub(started).Milliseconds())
	errText := ""
	if failure != nil {
		errText = failure.Error()
	}
	return store.InsertTaskRun(ctx, model.TaskRun{
		RunID:       summary.RunID,
		Dataset:     summary.Dataset,
		TaskType:    "local_import",
		Status:      status,
		TargetTable: summary.TargetTable,
		InputPath:   summary.InputPath,
		InputFormat: summary.InputFormat,
		Params:      "",
		StartedAt:   started,
		FinishedAt:  &finished,
		DurationMS:  &duration,
		RowsWritten: summary.RowsWritten,
		RowsSkipped: summary.RowsSkipped,
		Error:       errText,
		UpdatedAt:   finished,
	})
}

func datasetFor(period tdx.Period) string {
	switch period {
	case tdx.PeriodDay:
		return "a_share_bars_1d"
	case tdx.Period1m:
		return "a_share_bars_1m"
	case tdx.Period5m:
		return "a_share_bars_5m"
	default:
		return string(period)
	}
}

func targetTableFor(period tdx.Period) string {
	return datasetFor(period)
}

func severityFor(issueType string) string {
	switch issueType {
	case "conflicting_logical_key", "zero_valid_rows":
		return "error"
	default:
		return "warning"
	}
}

func newRunID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("run-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func expandHome(path string) string {
	if path == "" || path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
