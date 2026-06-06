package ingest

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	chstore "github.com/hoverwinter/infinity-marketd/internal/clickhouse"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

const defaultVIPDocZipBufferRows = 100000

type VIPDocZipOptions struct {
	ZipPath   string
	Periods   []tdx.Period
	Market    string
	Since     string
	Until     string
	DryRun    bool
	Store     *chstore.Store
	Timezone  string
	BatchSize int
	Progress  func(processed int, summary VIPDocZipSummary)
}

type VIPDocZipSummary struct {
	DryRun        bool
	ZipPath       string
	Files         int
	Files1m       int
	Files5m       int
	RowsWritten   uint64
	Rows1m        uint64
	Rows5m        uint64
	RowsSkipped   uint64
	QualityIssues int
}

type vipDocZipEntry struct {
	File        *zip.File
	Name        string
	InputPath   string
	Period      tdx.Period
	Dataset     string
	TargetTable string
	Market      string
	Symbol      string
}

type vipDocMinuteBufferKey struct {
	Table     string
	Partition string
}

func ImportVIPDocZip(ctx context.Context, opts VIPDocZipOptions) (VIPDocZipSummary, error) {
	if strings.TrimSpace(opts.ZipPath) == "" {
		return VIPDocZipSummary{}, fmt.Errorf("zip path is required")
	}
	if !opts.DryRun && opts.Store == nil {
		return VIPDocZipSummary{}, fmt.Errorf("store is required when dry-run is false")
	}
	if opts.Timezone == "" {
		opts.Timezone = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(opts.Timezone)
	if err != nil {
		return VIPDocZipSummary{}, err
	}
	periods, err := normalizeVIPDocZipPeriods(opts.Periods)
	if err != nil {
		return VIPDocZipSummary{}, err
	}
	flushRows := opts.BatchSize
	if flushRows <= 0 {
		flushRows = defaultVIPDocZipBufferRows
	}

	zipPath := expandHome(opts.ZipPath)
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return VIPDocZipSummary{}, err
	}
	defer reader.Close()

	entries, err := collectVIPDocZipEntries(zipPath, reader.File, periods, opts.Market)
	if err != nil {
		return VIPDocZipSummary{}, err
	}
	if len(entries) == 0 {
		return VIPDocZipSummary{}, fmt.Errorf("no 1m/5m vipdoc files found in %s", zipPath)
	}

	summary := VIPDocZipSummary{
		DryRun:  opts.DryRun,
		ZipPath: zipPath,
		Files:   len(entries),
	}
	for _, entry := range entries {
		switch entry.Period {
		case tdx.Period1m:
			summary.Files1m++
		case tdx.Period5m:
			summary.Files5m++
		}
	}

	buffers := make(map[vipDocMinuteBufferKey][]model.MinuteBar)
	var qualityIssues []model.QualityIssue
	var watermarks []model.Watermark
	var taskRuns []model.TaskRun

	flush := func(key vipDocMinuteBufferKey) error {
		bars := buffers[key]
		if len(bars) == 0 {
			return nil
		}
		if err := opts.Store.InsertMinuteBars(ctx, key.Table, bars); err != nil {
			return err
		}
		delete(buffers, key)
		return nil
	}

	for i, entry := range entries {
		started := time.Now()
		runID := newRunID()
		raw, err := readZipFile(entry.File)
		if err != nil {
			return summary, err
		}
		result := tdx.ParseMinuteBytes(raw, entry.InputPath, entry.Market, entry.Symbol, entry.Period, loc)
		bars, skipped, err := filterMinute(result.Bars, opts.Since, opts.Until, loc)
		if err != nil {
			return summary, err
		}

		issues := issuesFromParse(runID, entry.Dataset, entry.InputPath, entry.Market, entry.Symbol, result.Issues)
		if len(bars) == 0 {
			issues = append(issues, zeroRowsIssue(runID, entry.Dataset, entry.InputPath, entry.Market, entry.Symbol))
		}
		qualityIssues = append(qualityIssues, issues...)
		summary.RowsWritten += uint64(len(bars))
		summary.RowsSkipped += skipped
		summary.QualityIssues += len(issues)
		switch entry.Period {
		case tdx.Period1m:
			summary.Rows1m += uint64(len(bars))
		case tdx.Period5m:
			summary.Rows5m += uint64(len(bars))
		}

		if !opts.DryRun {
			for _, bar := range bars {
				key := vipDocMinuteBufferKey{Table: entry.TargetTable, Partition: bar.TradeDate.Format("200601")}
				buffers[key] = append(buffers[key], bar)
				if len(buffers[key]) >= flushRows {
					if err := flush(key); err != nil {
						return summary, err
					}
				}
			}
		}

		now := time.Now()
		status := "success"
		message := "ok"
		if len(issues) > 0 {
			status = "degraded"
			message = fmt.Sprintf("%d quality issue(s)", len(issues))
		}
		minWM, maxWM := minuteWatermarks(bars)
		watermarks = append(watermarks, model.Watermark{
			Dataset:      entry.Dataset,
			Asset:        fmt.Sprintf("%s:%s", entry.Market, entry.Symbol),
			Status:       status,
			MinWatermark: minWM,
			MaxWatermark: maxWM,
			RowsWritten:  uint64(len(bars)),
			Message:      message,
			UpdatedAt:    now,
		})
		duration := uint64(now.Sub(started).Milliseconds())
		taskRuns = append(taskRuns, model.TaskRun{
			RunID:       runID,
			Dataset:     entry.Dataset,
			TaskType:    "local_import",
			Status:      status,
			TargetTable: entry.TargetTable,
			InputPath:   entry.InputPath,
			InputFormat: result.Format,
			Params:      "",
			StartedAt:   started,
			FinishedAt:  &now,
			DurationMS:  &duration,
			RowsWritten: uint64(len(bars)),
			RowsSkipped: skipped,
			Error:       "",
			UpdatedAt:   now,
		})
		if opts.Progress != nil {
			opts.Progress(i+1, summary)
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

func normalizeVIPDocZipPeriods(periods []tdx.Period) ([]tdx.Period, error) {
	if len(periods) == 0 {
		return []tdx.Period{tdx.Period1m, tdx.Period5m}, nil
	}
	seen := make(map[tdx.Period]bool)
	var out []tdx.Period
	for _, period := range periods {
		switch period {
		case tdx.Period1m, tdx.Period5m:
			if !seen[period] {
				seen[period] = true
				out = append(out, period)
			}
		default:
			return nil, fmt.Errorf("vipdoc zip import only supports 1m/5m, got %s", period)
		}
	}
	return out, nil
}

func collectVIPDocZipEntries(zipPath string, files []*zip.File, periods []tdx.Period, marketFilter string) ([]vipDocZipEntry, error) {
	allowedPeriods := make(map[tdx.Period]bool, len(periods))
	for _, period := range periods {
		allowedPeriods[period] = true
	}
	marketFilter = strings.ToLower(strings.TrimSpace(marketFilter))
	if marketFilter != "" && marketFilter != "sh" && marketFilter != "sz" && marketFilter != "bj" {
		return nil, fmt.Errorf("unsupported market %q", marketFilter)
	}

	var entries []vipDocZipEntry
	for _, file := range files {
		if file.FileInfo().IsDir() {
			continue
		}
		period, ok := vipDocZipMinutePeriod(file.Name)
		if !ok || !allowedPeriods[period] {
			continue
		}
		inputPath := zipInputPath(zipPath, file.Name)
		market, symbol, err := tdx.ParseMarketSymbol(inputPath, "", "")
		if err != nil {
			return nil, err
		}
		if marketFilter != "" && market != marketFilter {
			continue
		}
		entries = append(entries, vipDocZipEntry{
			File:        file,
			Name:        normalizeZipEntryName(file.Name),
			InputPath:   inputPath,
			Period:      period,
			Dataset:     datasetFor(period),
			TargetTable: targetTableFor(period),
			Market:      market,
			Symbol:      symbol,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Period != entries[j].Period {
			return entries[i].Period < entries[j].Period
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

func vipDocZipMinutePeriod(name string) (tdx.Period, bool) {
	ext := strings.ToLower(filepath.Ext(normalizeZipEntryName(name)))
	switch ext {
	case ".lc1", ".1":
		return tdx.Period1m, true
	case ".lc5", ".5":
		return tdx.Period5m, true
	default:
		return "", false
	}
}

func zipInputPath(zipPath string, entryName string) string {
	return zipPath + "!" + normalizeZipEntryName(entryName)
}

func normalizeZipEntryName(name string) string {
	return strings.ReplaceAll(filepath.ToSlash(name), "\\", "/")
}

func readZipFile(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}
