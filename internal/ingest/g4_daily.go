package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	chstore "github.com/hoverwinter/infinity-marketd/internal/clickhouse"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

const g4DailyDataset = "a_share_bars_1d"

type FetchG4DayPackageFunc func(context.Context, time.Time, tdx.G4DayFetchOptions) ([]byte, string, error)

type G4DailyImportOptions struct {
	File         string
	Date         string
	BaseURL      string
	DryRun       bool
	Store        *chstore.Store
	Timezone     string
	HTTPClient   *http.Client
	FetchPackage FetchG4DayPackageFunc
	Now          func() time.Time

	ops       OnlineOps
	writeBars func(context.Context, []model.DailyBar) error
}

type G4DailySummary struct {
	RunID            string               `json:"run_id"`
	Dataset          string               `json:"dataset"`
	TargetTable      string               `json:"target_table"`
	Source           string               `json:"source"`
	TradeDate        string               `json:"trade_date"`
	SHA256           string               `json:"sha256"`
	PackageBytes     uint64               `json:"package_bytes"`
	Records          uint64               `json:"records"`
	SHRecords        uint64               `json:"sh_records"`
	SZRecords        uint64               `json:"sz_records"`
	BJRecords        uint64               `json:"bj_records"`
	EquityRecords    uint64               `json:"equity_records"`
	NoTradeRecords   uint64               `json:"no_trade_records"`
	NonEquityRecords uint64               `json:"non_equity_records"`
	RowsWritten      uint64               `json:"rows_written"`
	RowsSkipped      uint64               `json:"rows_skipped"`
	Issues           []model.QualityIssue `json:"issues"`
	DryRun           bool                 `json:"dry_run"`
}

func ImportG4DailyBars(ctx context.Context, opts G4DailyImportOptions) (G4DailySummary, error) {
	if opts.Timezone == "" {
		opts.Timezone = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(opts.Timezone)
	if err != nil {
		return G4DailySummary{}, err
	}
	requestedDate, hasDate, err := parseOnlineDailyDateBound(opts.Date, loc)
	if err != nil {
		return G4DailySummary{}, fmt.Errorf("parse --date: %w", err)
	}
	filePath := strings.TrimSpace(opts.File)
	if filePath == "" && !hasDate {
		return G4DailySummary{}, fmt.Errorf("--date is required when --file is not provided")
	}
	if filePath != "" && strings.TrimSpace(opts.BaseURL) != "" {
		return G4DailySummary{}, fmt.Errorf("--base-url cannot be used with --file")
	}
	if filePath != "" {
		filePath = filepath.Clean(expandHome(filePath))
	}
	if opts.FetchPackage == nil {
		opts.FetchPackage = tdx.FetchG4DayPackage
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}

	summary := G4DailySummary{
		Dataset:     g4DailyDataset,
		TargetTable: g4DailyDataset,
		Source:      filePath,
		DryRun:      opts.DryRun,
	}
	if hasDate {
		summary.TradeDate = requestedDate.Format("2006-01-02")
	}

	var ops OnlineOps = opts.ops
	if ops == nil && opts.Store != nil {
		ops = opts.Store
	}
	writeBars := opts.writeBars
	if writeBars == nil && opts.Store != nil {
		writeBars = opts.Store.InsertDailyBars
	}
	if !opts.DryRun && writeBars == nil {
		return summary, fmt.Errorf("store is required when dry-run is false")
	}

	sourceMode := "remote"
	if filePath != "" {
		sourceMode = "file"
	}
	params, _ := json.Marshal(map[string]any{
		"source_mode": sourceMode,
		"file":        filePath,
		"date":        opts.Date,
		"base_url":    opts.BaseURL,
	})
	produce := func(ctx context.Context, runID string) ([]model.DailyBar, uint64, []model.QualityIssue, error) {
		var raw []byte
		var source string
		var err error
		if filePath != "" {
			source = filePath
			raw, err = tdx.ReadG4DayPackageFile(filePath)
		} else {
			raw, source, err = opts.FetchPackage(ctx, requestedDate, tdx.G4DayFetchOptions{BaseURL: opts.BaseURL, HTTPClient: opts.HTTPClient})
		}
		if source != "" {
			summary.Source = source
		}
		if err != nil {
			return nil, 0, nil, err
		}
		var expectedDate *time.Time
		if hasDate {
			expectedDate = &requestedDate
		}
		parsed, err := tdx.ParseG4DayPackage(raw, source, expectedDate, loc)
		copyG4DailyParseSummary(&summary, parsed)
		if err != nil {
			return nil, parsed.NonEquityRecords + parsed.NoTradeRecords, nil, err
		}
		var issues []model.QualityIssue
		if len(parsed.Bars) == 0 {
			issues = append(issues, zeroRowsIssue(runID, g4DailyDataset, source, "", ""))
		}
		return parsed.Bars, parsed.NonEquityRecords + parsed.NoTradeRecords, issues, nil
	}

	result, err := RunOnlineJob(ctx, OnlineJob[model.DailyBar]{
		Dataset:     g4DailyDataset,
		TargetTable: g4DailyDataset,
		TaskType:    "tdx_g4_daily_import",
		InputFormat: tdx.G4DayInputFormat,
		Asset:       "all",
		Params:      string(params),
		DryRun:      opts.DryRun,
		Ops:         ops,
		Now:         opts.Now,
		Produce:     produce,
		Write: func(ctx context.Context, rows []model.DailyBar) error {
			return writeBars(ctx, rows)
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

func copyG4DailyParseSummary(summary *G4DailySummary, parsed tdx.G4DayParseResult) {
	if !parsed.TradeDate.IsZero() {
		summary.TradeDate = parsed.TradeDate.Format("2006-01-02")
	}
	summary.SHA256 = parsed.SHA256
	summary.PackageBytes = parsed.PackageBytes
	summary.Records = parsed.Records
	summary.SHRecords = parsed.SHRecords
	summary.SZRecords = parsed.SZRecords
	summary.BJRecords = parsed.BJRecords
	summary.EquityRecords = parsed.EquityRecords
	summary.NoTradeRecords = parsed.NoTradeRecords
	summary.NonEquityRecords = parsed.NonEquityRecords
}
