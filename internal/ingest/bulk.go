package ingest

import (
	"context"
	"fmt"
	"os"
	"time"

	chstore "github.com/hoverwinter/infinity-marketd/internal/clickhouse"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

const maxBulkPartitionBufferRows = 5000

type BulkOptions struct {
	Period    tdx.Period
	Files     []string
	Market    string
	Since     string
	Until     string
	Store     *chstore.Store
	Timezone  string
	BatchSize int
	Progress  func(processed int, summary BulkSummary)
}

type BulkSummary struct {
	Dataset       string
	TargetTable   string
	Files         int
	RowsWritten   uint64
	RowsSkipped   uint64
	QualityIssues int
}

func ImportDailyBulk(ctx context.Context, opts BulkOptions) (BulkSummary, error) {
	if opts.Period != tdx.PeriodDay {
		return BulkSummary{}, fmt.Errorf("bulk import only supports %s, got %s", tdx.PeriodDay, opts.Period)
	}
	if opts.Store == nil {
		return BulkSummary{}, fmt.Errorf("store is required")
	}
	if opts.Timezone == "" {
		opts.Timezone = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(opts.Timezone)
	if err != nil {
		return BulkSummary{}, err
	}
	flushRows := opts.BatchSize
	if flushRows <= 0 || flushRows > maxBulkPartitionBufferRows {
		flushRows = maxBulkPartitionBufferRows
	}

	summary := BulkSummary{
		Dataset:     datasetFor(opts.Period),
		TargetTable: targetTableFor(opts.Period),
		Files:       len(opts.Files),
	}
	buffers := make(map[string][]model.DailyBar)
	var qualityIssues []model.QualityIssue
	var watermarks []model.Watermark
	var taskRuns []model.TaskRun

	flush := func(partition string) error {
		bars := buffers[partition]
		if len(bars) == 0 {
			return nil
		}
		if err := opts.Store.InsertDailyBars(ctx, bars); err != nil {
			return err
		}
		delete(buffers, partition)
		return nil
	}

	for _, path := range opts.Files {
		started := time.Now()
		runID := newRunID()
		market, symbol, err := tdx.ParseMarketSymbol(path, opts.Market, "")
		if err != nil {
			return summary, err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return summary, err
		}
		result := tdx.ParseDayBytes(raw, path, market, symbol, loc)
		bars, skipped, err := filterDaily(result.Bars, opts.Since, opts.Until, loc)
		if err != nil {
			return summary, err
		}

		issues := issuesFromParse(runID, summary.Dataset, path, market, symbol, result.Issues)
		if len(bars) == 0 {
			issues = append(issues, zeroRowsIssue(runID, summary.Dataset, path, market, symbol))
		}
		qualityIssues = append(qualityIssues, issues...)
		summary.RowsWritten += uint64(len(bars))
		summary.RowsSkipped += skipped
		summary.QualityIssues += len(issues)

		for _, bar := range bars {
			partition := bar.TradeDate.Format("2006")
			buffers[partition] = append(buffers[partition], bar)
			if len(buffers[partition]) >= flushRows {
				if err := flush(partition); err != nil {
					return summary, err
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
		minWM, maxWM := dailyWatermarks(bars)
		watermarks = append(watermarks, model.Watermark{
			Dataset:      summary.Dataset,
			Asset:        fmt.Sprintf("%s:%s", market, symbol),
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
			Dataset:     summary.Dataset,
			TaskType:    "local_import",
			Status:      status,
			TargetTable: summary.TargetTable,
			InputPath:   path,
			InputFormat: "tdx.day.<IIIIIfII>",
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
			opts.Progress(len(watermarks), summary)
		}
	}

	for partition := range buffers {
		if err := flush(partition); err != nil {
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
