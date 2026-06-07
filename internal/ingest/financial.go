package ingest

import (
	"context"
	"fmt"
	"strings"
	"time"

	chstore "github.com/hoverwinter/infinity-marketd/internal/clickhouse"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
	"github.com/hoverwinter/infinity-marketd/internal/tdx/finance"
)

const defaultFinancialBufferRows = 100000

type TDXFinancialOptions struct {
	File      string
	DryRun    bool
	Store     *chstore.Store
	Timezone  string
	BatchSize int
	Progress  func(processed int, total int, summary TDXFinancialSummary)
}

type TDXFinancialSummary struct {
	DryRun          bool
	InputPath       string
	Dataset         string
	TargetTable     string
	InputFormat     string
	FilesDiscovered int
	FilesProcessed  int
	ManifestFiles   int
	RowsWritten     uint64
	RowsSkipped     uint64
	DictionaryCount int
	ManifestIssues  int
	QualityIssues   int
}

type TDXGPOptions = TDXFinancialOptions
type TDXGPSummary = TDXFinancialSummary

type financialBufferKey struct {
	Partition string
}

func ImportTDXFinancial(ctx context.Context, opts TDXFinancialOptions) (TDXFinancialSummary, error) {
	if strings.TrimSpace(opts.File) == "" {
		return TDXFinancialSummary{}, fmt.Errorf("file is required")
	}
	if !opts.DryRun && opts.Store == nil {
		return TDXFinancialSummary{}, fmt.Errorf("store is required when dry-run is false")
	}
	loc, err := financialLocation(opts.Timezone)
	if err != nil {
		return TDXFinancialSummary{}, err
	}
	dictionary, err := finance.LoadFinancialItemDictionary()
	if err != nil {
		return TDXFinancialSummary{}, err
	}
	dictionaryMap := finance.FinancialItemDictionaryMap(dictionary)
	inputPath := expandHome(opts.File)
	result, err := finance.ParseFinancialZip(inputPath, loc, dictionaryMap)
	if err != nil {
		return TDXFinancialSummary{}, err
	}
	summary := TDXFinancialSummary{
		DryRun:          opts.DryRun,
		InputPath:       inputPath,
		Dataset:         "a_share_financial_raw_items",
		TargetTable:     "a_share_financial_raw_items",
		InputFormat:     result.Format,
		FilesDiscovered: result.FilesDiscovered,
		ManifestFiles:   result.ManifestFiles,
		DictionaryCount: len(dictionary),
	}
	summary.ManifestIssues = len(result.Issues)

	var qualityIssues []model.QualityIssue
	for _, issue := range result.Issues {
		qualityIssues = append(qualityIssues, qualityIssueFromParse(newRunID(), summary.Dataset, inputPath, "", "", issue))
	}

	flushRows := opts.BatchSize
	if flushRows <= 0 {
		flushRows = defaultFinancialBufferRows
	}
	buffers := make(map[financialBufferKey][]model.FinancialRawItem)
	flush := func(key financialBufferKey) error {
		rows := buffers[key]
		if len(rows) == 0 {
			return nil
		}
		if err := opts.Store.InsertFinancialRawItems(ctx, rows); err != nil {
			return err
		}
		delete(buffers, key)
		return nil
	}

	var watermarks []model.Watermark
	var taskRuns []model.TaskRun
	if !opts.DryRun {
		if err := opts.Store.InsertFinancialItemDictionary(ctx, dictionary); err != nil {
			return summary, err
		}
	}
	for i, entry := range result.Entries {
		runID := newRunID()
		started := time.Now()
		issues := issuesFromParse(runID, summary.Dataset, entry.InputPath, "", "", entry.Issues)
		if len(entry.Rows) == 0 {
			issues = append(issues, zeroRowsIssue(runID, summary.Dataset, entry.InputPath, "", ""))
		}
		qualityIssues = append(qualityIssues, issues...)
		summary.FilesProcessed++
		summary.RowsWritten += uint64(len(entry.Rows))
		summary.RowsSkipped += uint64(skippedFromIssues(entry.Issues))
		summary.QualityIssues += len(issues)
		if !opts.DryRun {
			for _, row := range entry.Rows {
				key := financialBufferKey{Partition: row.ReportDate.Format("2006")}
				buffers[key] = append(buffers[key], row)
				if len(buffers[key]) >= flushRows {
					if err := flush(key); err != nil {
						return summary, err
					}
				}
			}
		}
		now := time.Now()
		status, message := statusForIssues(issues)
		minWM, maxWM := financialRawWatermarks(entry.Rows)
		watermarks = append(watermarks, model.Watermark{
			Dataset:      summary.Dataset,
			Asset:        entry.Name,
			Status:       status,
			MinWatermark: minWM,
			MaxWatermark: maxWM,
			RowsWritten:  uint64(len(entry.Rows)),
			Message:      message,
			UpdatedAt:    now,
		})
		duration := uint64(now.Sub(started).Milliseconds())
		taskRuns = append(taskRuns, model.TaskRun{
			RunID:       runID,
			Dataset:     summary.Dataset,
			TaskType:    "local_import",
			Status:      status,
			TargetTable: summary.TargetTable,
			InputPath:   entry.InputPath,
			InputFormat: "tdx.gpcw.<hIHL3L,index,fields:f32>",
			Params:      "",
			StartedAt:   started,
			FinishedAt:  &now,
			DurationMS:  &duration,
			RowsWritten: uint64(len(entry.Rows)),
			RowsSkipped: uint64(skippedFromIssues(entry.Issues)),
			Error:       "",
			UpdatedAt:   now,
		})
		if opts.Progress != nil {
			opts.Progress(i+1, len(result.Entries), summary)
		}
	}
	if opts.DryRun {
		return summary, nil
	}
	for key := range buffers {
		if err := flush(key); err != nil {
			return summary, err
		}
	}
	if err := opts.Store.InsertQualityIssues(ctx, qualityIssues); err != nil {
		return summary, err
	}
	if err := opts.Store.InsertWatermarks(ctx, watermarks); err != nil {
		return summary, err
	}
	if err := opts.Store.InsertTaskRuns(ctx, taskRuns); err != nil {
		return summary, err
	}
	return summary, nil
}

func ImportTDXGP(ctx context.Context, opts TDXGPOptions) (TDXGPSummary, error) {
	if strings.TrimSpace(opts.File) == "" {
		return TDXGPSummary{}, fmt.Errorf("file is required")
	}
	if !opts.DryRun && opts.Store == nil {
		return TDXGPSummary{}, fmt.Errorf("store is required when dry-run is false")
	}
	loc, err := financialLocation(opts.Timezone)
	if err != nil {
		return TDXGPSummary{}, err
	}
	dictionary, err := finance.LoadGPMetricDictionary()
	if err != nil {
		return TDXGPSummary{}, err
	}
	dictionaryMap := finance.GPMetricDictionaryMap(dictionary)
	inputPath := expandHome(opts.File)
	result, err := finance.ParseGPZip(inputPath, loc, dictionaryMap)
	if err != nil {
		return TDXGPSummary{}, err
	}
	summary := TDXGPSummary{
		DryRun:          opts.DryRun,
		InputPath:       inputPath,
		Dataset:         "a_share_gp_metric_values",
		TargetTable:     "a_share_gp_metric_values",
		InputFormat:     result.Format,
		FilesDiscovered: result.FilesDiscovered,
		ManifestFiles:   result.ManifestFiles,
		DictionaryCount: len(dictionary),
	}
	summary.ManifestIssues = len(result.Issues)

	var qualityIssues []model.QualityIssue
	for _, issue := range result.Issues {
		qualityIssues = append(qualityIssues, qualityIssueFromParse(newRunID(), summary.Dataset, inputPath, "", "", issue))
	}

	flushRows := opts.BatchSize
	if flushRows <= 0 {
		flushRows = defaultFinancialBufferRows
	}
	buffers := make(map[financialBufferKey][]model.GPMetricValue)
	flush := func(key financialBufferKey) error {
		rows := buffers[key]
		if len(rows) == 0 {
			return nil
		}
		if err := opts.Store.InsertGPMetricValues(ctx, rows); err != nil {
			return err
		}
		delete(buffers, key)
		return nil
	}

	var watermarks []model.Watermark
	var taskRuns []model.TaskRun
	if !opts.DryRun {
		if err := opts.Store.InsertGPMetricDictionary(ctx, dictionary); err != nil {
			return summary, err
		}
	}
	for i, entry := range result.Entries {
		runID := newRunID()
		started := time.Now()
		issues := issuesFromParse(runID, summary.Dataset, entry.InputPath, entry.Market, entry.Symbol, entry.Issues)
		if len(entry.Rows) == 0 {
			issues = append(issues, zeroRowsIssue(runID, summary.Dataset, entry.InputPath, entry.Market, entry.Symbol))
		}
		qualityIssues = append(qualityIssues, issues...)
		summary.FilesProcessed++
		summary.RowsWritten += uint64(len(entry.Rows))
		summary.RowsSkipped += uint64(skippedFromIssues(entry.Issues))
		summary.QualityIssues += len(issues)
		if !opts.DryRun {
			for _, row := range entry.Rows {
				key := financialBufferKey{Partition: row.EventDate.Format("2006")}
				buffers[key] = append(buffers[key], row)
				if len(buffers[key]) >= flushRows {
					if err := flush(key); err != nil {
						return summary, err
					}
				}
			}
		}
		now := time.Now()
		status, message := statusForIssues(issues)
		minWM, maxWM := gpMetricWatermarks(entry.Rows)
		watermarks = append(watermarks, model.Watermark{
			Dataset:      summary.Dataset,
			Asset:        fmt.Sprintf("%s:%s", entry.Market, entry.Symbol),
			Status:       status,
			MinWatermark: minWM,
			MaxWatermark: maxWM,
			RowsWritten:  uint64(len(entry.Rows)),
			Message:      message,
			UpdatedAt:    now,
		})
		duration := uint64(now.Sub(started).Milliseconds())
		taskRuns = append(taskRuns, model.TaskRun{
			RunID:       runID,
			Dataset:     summary.Dataset,
			TaskType:    "local_import",
			Status:      status,
			TargetTable: summary.TargetTable,
			InputPath:   entry.InputPath,
			InputFormat: "tdx.gp.<BIff>",
			Params:      "",
			StartedAt:   started,
			FinishedAt:  &now,
			DurationMS:  &duration,
			RowsWritten: uint64(len(entry.Rows)),
			RowsSkipped: uint64(skippedFromIssues(entry.Issues)),
			Error:       "",
			UpdatedAt:   now,
		})
		if opts.Progress != nil {
			opts.Progress(i+1, len(result.Entries), summary)
		}
	}
	if opts.DryRun {
		return summary, nil
	}
	for key := range buffers {
		if err := flush(key); err != nil {
			return summary, err
		}
	}
	if err := opts.Store.InsertQualityIssues(ctx, qualityIssues); err != nil {
		return summary, err
	}
	if err := opts.Store.InsertWatermarks(ctx, watermarks); err != nil {
		return summary, err
	}
	if err := opts.Store.InsertTaskRuns(ctx, taskRuns); err != nil {
		return summary, err
	}
	return summary, nil
}

func financialLocation(timezone string) (*time.Location, error) {
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	return time.LoadLocation(timezone)
}

func qualityIssueFromParse(runID string, dataset string, path string, market string, symbol string, issue tdx.ParseIssue) model.QualityIssue {
	return model.QualityIssue{
		RunID:             runID,
		Dataset:           dataset,
		Severity:          severityFor(issue.Type),
		IssueType:         issue.Type,
		Market:            market,
		Symbol:            symbol,
		LogicalKey:        issue.LogicalKey,
		InputPath:         path,
		InputRecordOffset: issue.Offset,
		ObservedAt:        time.Now(),
		Message:           issue.Message,
		Details:           "",
	}
}

func statusForIssues(issues []model.QualityIssue) (string, string) {
	if len(issues) == 0 {
		return "success", "ok"
	}
	for _, issue := range issues {
		if issue.Severity == "error" {
			return "failed", fmt.Sprintf("%d quality issue(s)", len(issues))
		}
	}
	return "degraded", fmt.Sprintf("%d quality issue(s)", len(issues))
}

func skippedFromIssues(issues []tdx.ParseIssue) int {
	skipped := 0
	for _, issue := range issues {
		switch issue.Type {
		case "invalid_date", "unsupported_market", "unsupported_symbol", "duplicate_logical_key", "unknown_dictionary_id", "incomplete_trailing_bytes":
			skipped++
		}
	}
	return skipped
}

func financialRawWatermarks(rows []model.FinancialRawItem) (*time.Time, *time.Time) {
	if len(rows) == 0 {
		return nil, nil
	}
	min := rows[0].ReportDate
	max := rows[0].ReportDate
	for _, row := range rows[1:] {
		if row.ReportDate.Before(min) {
			min = row.ReportDate
		}
		if row.ReportDate.After(max) {
			max = row.ReportDate
		}
	}
	return &min, &max
}

func gpMetricWatermarks(rows []model.GPMetricValue) (*time.Time, *time.Time) {
	if len(rows) == 0 {
		return nil, nil
	}
	min := rows[0].EventDate
	max := rows[0].EventDate
	for _, row := range rows[1:] {
		if row.EventDate.Before(min) {
			min = row.EventDate
		}
		if row.EventDate.After(max) {
			max = row.EventDate
		}
	}
	return &min, &max
}
