package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

type NormalizedReviewImportSummary struct {
	RunID       string               `json:"run_id"`
	Dataset     string               `json:"dataset"`
	TargetTable string               `json:"target_table"`
	RowsWritten uint64               `json:"rows_written"`
	RowsSkipped uint64               `json:"rows_skipped"`
	Issues      []model.QualityIssue `json:"issues"`
	DryRun      bool                 `json:"dry_run"`
}

type rawPerformanceIndexBar struct {
	IndexCode string   `json:"index_code"`
	TradeDate string   `json:"trade_date"`
	Open      float64  `json:"open"`
	High      float64  `json:"high"`
	Low       float64  `json:"low"`
	Close     float64  `json:"close"`
	Volume    *uint64  `json:"volume"`
	Amount    *float64 `json:"amount"`
}

type rawMarketBreadthDaily struct {
	TradeDate                 string  `json:"trade_date"`
	UpCount                   *uint32 `json:"up_count"`
	DownCount                 *uint32 `json:"down_count"`
	FlatCount                 *uint32 `json:"flat_count"`
	UnchangedOrSuspendedCount *uint32 `json:"unchanged_or_suspended_count"`
	UpGT3Count                *uint32 `json:"up_gt_3_count"`
	UpGT5Count                *uint32 `json:"up_gt_5_count"`
	UpGT7Count                *uint32 `json:"up_gt_7_count"`
	DownGT3Count              *uint32 `json:"down_gt_3_count"`
	DownGT5Count              *uint32 `json:"down_gt_5_count"`
	DownGT7Count              *uint32 `json:"down_gt_7_count"`
	LimitUpCount              *uint32 `json:"limit_up_count"`
	LimitDownCount            *uint32 `json:"limit_down_count"`
	TotalCount                *uint32 `json:"total_count"`
}

func ImportLimitPerformanceJSON(ctx context.Context, opts LimitReviewImportOptions) (summary NormalizedReviewImportSummary, retErr error) {
	const dataset = "a_share_limit_performance_index_bars_1d"
	loc, now, since, until, err := normalizeLimitReviewImportOptions(opts)
	if err != nil {
		return summary, err
	}
	path := expandHome(strings.TrimSpace(opts.File))
	if path == "" {
		return NormalizedReviewImportSummary{}, fmt.Errorf("--file is required")
	}
	if !opts.DryRun && opts.Store == nil {
		return NormalizedReviewImportSummary{}, fmt.Errorf("store is required when dry-run is false")
	}
	runID := newRunID()
	started := now()
	summary = NormalizedReviewImportSummary{RunID: runID, Dataset: dataset, TargetTable: dataset, DryRun: opts.DryRun, Issues: []model.QualityIssue{}}
	recorded := false
	defer func() {
		if opts.DryRun || recorded || retErr == nil {
			return
		}
		issue := limitReviewIssue(runID, path, "invalid_normalized_review", "", "", retErr.Error(), "error", now)
		issue.Dataset = dataset
		summary.Issues = append(summary.Issues, issue)
		if err := opts.Store.InsertQualityIssues(ctx, summary.Issues); err != nil {
			retErr = fmt.Errorf("%w; record issues: %v", retErr, err)
		}
		if err := recordDatasetTaskRun(ctx, opts.Store, dataset, runID, "normalized_json_import", dataset, path, "json", "{}", started, now, 0, summary.RowsSkipped, "failed", retErr); err != nil {
			retErr = fmt.Errorf("%w; record task: %v", retErr, err)
		}
	}()
	raw, err := os.ReadFile(path)
	if err != nil {
		return summary, err
	}
	var input []rawPerformanceIndexBar
	if err := decodeNormalizedRows(raw, "bars", &input); err != nil {
		return summary, err
	}
	seen := map[string]model.LimitPerformanceIndexBar{}
	for _, rawRow := range input {
		tradeDate, err := parseReviewDate(rawRow.TradeDate, loc)
		if err != nil {
			return summary, err
		}
		if (since != nil && tradeDate.Before(*since)) || (until != nil && tradeDate.After(*until)) {
			summary.RowsSkipped++
			continue
		}
		if !containsString([]string{"prev_limit_up_perf", "prev_non_st_limit_up_perf", "prev_ladder_perf", "prev_limit_down_perf"}, rawRow.IndexCode) {
			return summary, fmt.Errorf("unsupported index_code %q", rawRow.IndexCode)
		}
		if !finiteNumbers(rawRow.Open, rawRow.High, rawRow.Low, rawRow.Close) || rawRow.Low <= 0 || rawRow.High < rawRow.Open || rawRow.High < rawRow.Close || rawRow.Low > rawRow.Open || rawRow.Low > rawRow.Close {
			return summary, fmt.Errorf("invalid OHLC for %s on %s", rawRow.IndexCode, rawRow.TradeDate)
		}
		if err := validateOptionalNonNegative("amount", rawRow.Amount); err != nil {
			return summary, err
		}
		row := model.LimitPerformanceIndexBar{IndexCode: rawRow.IndexCode, TradeDate: tradeDate, Open: rawRow.Open, High: rawRow.High, Low: rawRow.Low, Close: rawRow.Close, Volume: rawRow.Volume, Amount: rawRow.Amount}
		key := rawRow.IndexCode + "|" + rawRow.TradeDate
		if prev, ok := seen[key]; ok {
			summary.RowsSkipped++
			if !reflect.DeepEqual(prev, row) {
				return summary, fmt.Errorf("conflicting logical key %s", key)
			}
			continue
		}
		seen[key] = row
	}
	keys := sortedMapKeys(seen)
	rows := make([]model.LimitPerformanceIndexBar, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, seen[key])
	}
	summary.RowsWritten = uint64(len(rows))
	if opts.DryRun {
		return summary, nil
	}
	writeErr := opts.Store.InsertLimitPerformanceIndexBars(ctx, rows)
	minWM, maxWM := performanceWatermarks(rows)
	recorded = true
	return finishNormalizedReviewImport(ctx, opts, summary, path, "json.limit_performance_indices.v1", started, now, minWM, maxWM, writeErr)
}

func ImportMarketBreadthJSON(ctx context.Context, opts LimitReviewImportOptions) (summary NormalizedReviewImportSummary, retErr error) {
	const dataset = "a_share_market_breadth_daily"
	loc, now, since, until, err := normalizeLimitReviewImportOptions(opts)
	if err != nil {
		return summary, err
	}
	path := expandHome(strings.TrimSpace(opts.File))
	if path == "" {
		return NormalizedReviewImportSummary{}, fmt.Errorf("--file is required")
	}
	if !opts.DryRun && opts.Store == nil {
		return NormalizedReviewImportSummary{}, fmt.Errorf("store is required when dry-run is false")
	}
	runID := newRunID()
	started := now()
	summary = NormalizedReviewImportSummary{RunID: runID, Dataset: dataset, TargetTable: dataset, DryRun: opts.DryRun, Issues: []model.QualityIssue{}}
	recorded := false
	defer func() {
		if opts.DryRun || recorded || retErr == nil {
			return
		}
		issue := limitReviewIssue(runID, path, "invalid_normalized_review", "", "", retErr.Error(), "error", now)
		issue.Dataset = dataset
		summary.Issues = append(summary.Issues, issue)
		if err := opts.Store.InsertQualityIssues(ctx, summary.Issues); err != nil {
			retErr = fmt.Errorf("%w; record issues: %v", retErr, err)
		}
		if err := recordDatasetTaskRun(ctx, opts.Store, dataset, runID, "normalized_json_import", dataset, path, "json", "{}", started, now, 0, summary.RowsSkipped, "failed", retErr); err != nil {
			retErr = fmt.Errorf("%w; record task: %v", retErr, err)
		}
	}()
	raw, err := os.ReadFile(path)
	if err != nil {
		return summary, err
	}
	var input []rawMarketBreadthDaily
	if err := decodeNormalizedRows(raw, "rows", &input); err != nil {
		return summary, err
	}
	seen := map[string]model.MarketBreadthDaily{}
	for _, rawRow := range input {
		tradeDate, err := parseReviewDate(rawRow.TradeDate, loc)
		if err != nil {
			return summary, err
		}
		if (since != nil && tradeDate.Before(*since)) || (until != nil && tradeDate.After(*until)) {
			summary.RowsSkipped++
			continue
		}
		if err := validateBreadth(rawRow); err != nil {
			return summary, fmt.Errorf("%s: %w", rawRow.TradeDate, err)
		}
		row := model.MarketBreadthDaily{TradeDate: tradeDate, UpCount: *rawRow.UpCount, DownCount: *rawRow.DownCount, FlatCount: rawRow.FlatCount, UnchangedOrSuspendedCount: rawRow.UnchangedOrSuspendedCount, UpGT3Count: rawRow.UpGT3Count, UpGT5Count: rawRow.UpGT5Count, UpGT7Count: rawRow.UpGT7Count, DownGT3Count: rawRow.DownGT3Count, DownGT5Count: rawRow.DownGT5Count, DownGT7Count: rawRow.DownGT7Count, LimitUpCount: rawRow.LimitUpCount, LimitDownCount: rawRow.LimitDownCount, TotalCount: *rawRow.TotalCount}
		if prev, ok := seen[rawRow.TradeDate]; ok {
			summary.RowsSkipped++
			if !reflect.DeepEqual(prev, row) {
				return summary, fmt.Errorf("conflicting logical key %s", rawRow.TradeDate)
			}
			continue
		}
		seen[rawRow.TradeDate] = row
	}
	keys := sortedMapKeys(seen)
	rows := make([]model.MarketBreadthDaily, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, seen[key])
	}
	summary.RowsWritten = uint64(len(rows))
	if opts.DryRun {
		return summary, nil
	}
	writeErr := opts.Store.InsertMarketBreadthDaily(ctx, rows)
	minWM, maxWM := breadthWatermarks(rows)
	recorded = true
	return finishNormalizedReviewImport(ctx, opts, summary, path, "json.market_breadth.v1", started, now, minWM, maxWM, writeErr)
}

func decodeNormalizedRows[T any](raw []byte, field string, target *[]T) error {
	trimmed := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(trimmed, "[") {
		envelope := map[string]json.RawMessage{}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return err
		}
		value, ok := envelope[field]
		if !ok {
			return fmt.Errorf("top-level array or %q field is required", field)
		}
		raw = value
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return err
	}
	if len(*target) == 0 {
		return fmt.Errorf("input rows must not be empty or null")
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func validateBreadth(row rawMarketBreadthDaily) error {
	if row.UpCount == nil || row.DownCount == nil || row.TotalCount == nil {
		return fmt.Errorf("up_count, down_count, and total_count are required")
	}
	if *row.TotalCount == 0 {
		return fmt.Errorf("total_count must be positive")
	}
	total := uint64(*row.UpCount) + uint64(*row.DownCount)
	for _, count := range []*uint32{row.FlatCount, row.UnchangedOrSuspendedCount} {
		if count != nil {
			total += uint64(*count)
		}
	}
	if total > uint64(*row.TotalCount) {
		return fmt.Errorf("market counts exceed total_count")
	}
	for _, side := range []struct {
		count      uint32
		thresholds []*uint32
		limit      *uint32
	}{{*row.UpCount, []*uint32{row.UpGT3Count, row.UpGT5Count, row.UpGT7Count}, row.LimitUpCount}, {*row.DownCount, []*uint32{row.DownGT3Count, row.DownGT5Count, row.DownGT7Count}, row.LimitDownCount}} {
		max := side.count
		for _, threshold := range side.thresholds {
			if threshold != nil {
				if *threshold > max {
					return fmt.Errorf("threshold counts are not nested")
				}
				max = *threshold
			}
		}
		if side.limit != nil && *side.limit > side.count {
			return fmt.Errorf("limit counts exceed directional counts")
		}
	}
	return nil
}

func sortedMapKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func performanceWatermarks(rows []model.LimitPerformanceIndexBar) (*time.Time, *time.Time) {
	dates := make([]time.Time, 0, len(rows))
	for _, row := range rows {
		dates = append(dates, row.TradeDate)
	}
	return timeBounds(dates)
}

func breadthWatermarks(rows []model.MarketBreadthDaily) (*time.Time, *time.Time) {
	dates := make([]time.Time, 0, len(rows))
	for _, row := range rows {
		dates = append(dates, row.TradeDate)
	}
	return timeBounds(dates)
}

func timeBounds(values []time.Time) (*time.Time, *time.Time) {
	if len(values) == 0 {
		return nil, nil
	}
	min, max := values[0], values[0]
	for _, value := range values[1:] {
		if value.Before(min) {
			min = value
		}
		if value.After(max) {
			max = value
		}
	}
	return &min, &max
}

func finishNormalizedReviewImport(ctx context.Context, opts LimitReviewImportOptions, summary NormalizedReviewImportSummary, path, inputFormat string, started time.Time, now func() time.Time, minWM, maxWM *time.Time, writeErr error) (NormalizedReviewImportSummary, error) {
	status := "success"
	if writeErr != nil {
		status = "failed"
		summary.RowsWritten = 0
	}
	if writeErr == nil && (minWM != nil || maxWM != nil) {
		writeErr = opts.Store.InsertWatermark(ctx, model.Watermark{Dataset: summary.Dataset, Asset: "all", Status: status, MinWatermark: minWM, MaxWatermark: maxWM, RowsWritten: summary.RowsWritten, Message: status, UpdatedAt: now()})
	}
	if writeErr != nil {
		status = "failed"
	}
	params, _ := json.Marshal(map[string]any{"since": opts.Since, "until": opts.Until})
	if runErr := recordDatasetTaskRun(ctx, opts.Store, summary.Dataset, summary.RunID, "normalized_json_import", summary.TargetTable, path, inputFormat, string(params), started, now, summary.RowsWritten, summary.RowsSkipped, status, writeErr); writeErr == nil {
		writeErr = runErr
	}
	if writeErr != nil {
		return summary, writeErr
	}
	return summary, nil
}
