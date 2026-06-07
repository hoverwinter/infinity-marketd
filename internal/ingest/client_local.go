package ingest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	chstore "github.com/hoverwinter/infinity-marketd/internal/clickhouse"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

type GBBQOptions struct {
	File      string
	Since     string
	Until     string
	DryRun    bool
	Store     *chstore.Store
	Timezone  string
	BatchSize int
}

type BlockOptions struct {
	File      string
	Scope     string
	DryRun    bool
	Store     *chstore.Store
	Timezone  string
	BatchSize int
}

type ExDailyOptions struct {
	File     string
	Market   uint16
	Code     string
	Since    string
	Until    string
	DryRun   bool
	Store    *chstore.Store
	Timezone string
}

func ImportTDXGBBQ(ctx context.Context, opts GBBQOptions) (Summary, error) {
	loc, err := importLocation(opts.Timezone)
	if err != nil {
		return Summary{}, err
	}
	path, raw, err := readClientLocalFile(opts.File)
	if err != nil {
		return Summary{}, err
	}
	runID := newRunID()
	started := time.Now()
	summary := Summary{
		RunID:       runID,
		Dataset:     "a_share_capital_change_events",
		TargetTable: "a_share_capital_change_events",
		InputPath:   path,
		InputFormat: "tdx.gbbq.client_local",
		DryRun:      opts.DryRun,
	}
	result := tdx.ParseGBBQBytes(raw, path, loc)
	events, skipped, err := filterCapitalEvents(result.Events, opts.Since, opts.Until, loc)
	if err != nil {
		return Summary{}, err
	}
	summary.RowsSkipped = skipped
	summary.Issues = issuesFromParse(runID, summary.Dataset, path, "", "", result.Issues)
	if len(events) == 0 {
		summary.Issues = append(summary.Issues, zeroRowsIssue(runID, summary.Dataset, path, "", ""))
	}
	summary.RowsWritten = uint64(len(events))
	if opts.DryRun {
		return summary, nil
	}
	if opts.Store == nil {
		return Summary{}, fmt.Errorf("store is required when dry-run is false")
	}
	if err := opts.Store.InsertCapitalChangeEvents(ctx, events); err != nil {
		recordFailure(ctx, opts.Store, summary, started, err)
		return summary, err
	}
	return finishClientLocalImport(ctx, opts.Store, summary, started, capitalWatermarks(events), filepath.Base(path))
}

func ImportTDXBlock(ctx context.Context, opts BlockOptions) (Summary, error) {
	loc, err := importLocation(opts.Timezone)
	if err != nil {
		return Summary{}, err
	}
	path := expandHome(opts.File)
	if path == "" {
		return Summary{}, fmt.Errorf("--file is required")
	}
	if isOfflinePackage(path) {
		return Summary{}, fmt.Errorf("client-local block import does not accept offline package %s", path)
	}
	runID := newRunID()
	started := time.Now()
	scope := strings.ToLower(strings.TrimSpace(opts.Scope))
	if scope == "" {
		scope = "system"
	}
	if scope != "system" && scope != "custom" {
		return Summary{}, fmt.Errorf("unsupported block scope %q", opts.Scope)
	}
	summary := Summary{
		RunID:       runID,
		Dataset:     "tdx_block_" + scope,
		TargetTable: "tdx_block_snapshots,tdx_block_definitions,tdx_block_memberships",
		InputPath:   path,
		InputFormat: "tdx.block.client_local." + scope,
		DryRun:      opts.DryRun,
	}
	var result tdx.BlockParseResult
	if scope == "custom" {
		result, err = tdx.ParseCustomBlockDir(path, time.Now().In(loc))
	} else {
		var raw []byte
		raw, err = os.ReadFile(path)
		if err == nil {
			result = tdx.ParseSystemBlockBytes(raw, path, scope, time.Now().In(loc))
		}
	}
	if err != nil {
		return Summary{}, err
	}
	summary.RowsWritten = uint64(1 + len(result.Definitions) + len(result.Memberships))
	summary.Issues = issuesFromParse(runID, summary.Dataset, path, "", "", result.Issues)
	if len(result.Definitions) == 0 {
		summary.Issues = append(summary.Issues, zeroRowsIssue(runID, summary.Dataset, path, "", ""))
	}
	if opts.DryRun {
		return summary, nil
	}
	if opts.Store == nil {
		return Summary{}, fmt.Errorf("store is required when dry-run is false")
	}
	if err := opts.Store.InsertTDXBlockSnapshots(ctx, []model.TDXBlockSnapshot{result.Snapshot}); err != nil {
		recordFailure(ctx, opts.Store, summary, started, err)
		return summary, err
	}
	if err := opts.Store.InsertTDXBlockDefinitions(ctx, result.Definitions); err != nil {
		recordFailure(ctx, opts.Store, summary, started, err)
		return summary, err
	}
	if err := opts.Store.InsertTDXBlockMemberships(ctx, result.Memberships); err != nil {
		recordFailure(ctx, opts.Store, summary, started, err)
		return summary, err
	}
	return finishClientLocalImport(ctx, opts.Store, summary, started, &result.Snapshot.SnapshotTime, result.Snapshot.SnapshotID)
}

func ImportTDXExDaily(ctx context.Context, opts ExDailyOptions) (Summary, error) {
	loc, err := importLocation(opts.Timezone)
	if err != nil {
		return Summary{}, err
	}
	if strings.TrimSpace(opts.Code) == "" {
		return Summary{}, fmt.Errorf("--code is required")
	}
	path, raw, err := readClientLocalFile(opts.File)
	if err != nil {
		return Summary{}, err
	}
	runID := newRunID()
	started := time.Now()
	summary := Summary{
		RunID:       runID,
		Dataset:     "tdx_ex_bars_1d",
		TargetTable: "tdx_ex_bars_1d",
		InputPath:   path,
		DryRun:      opts.DryRun,
	}
	result := tdx.ParseExDailyBytes(raw, path, opts.Market, opts.Code, loc)
	summary.InputFormat = result.Format
	bars, skipped, err := filterExDaily(result.Bars, opts.Since, opts.Until, loc)
	if err != nil {
		return Summary{}, err
	}
	summary.RowsSkipped = skipped
	summary.Issues = issuesFromParse(runID, summary.Dataset, path, "", opts.Code, result.Issues)
	if len(bars) == 0 {
		summary.Issues = append(summary.Issues, zeroRowsIssue(runID, summary.Dataset, path, "", opts.Code))
	}
	summary.RowsWritten = uint64(len(bars))
	if opts.DryRun {
		return summary, nil
	}
	if opts.Store == nil {
		return Summary{}, fmt.Errorf("store is required when dry-run is false")
	}
	if err := opts.Store.InsertExDailyBars(ctx, bars); err != nil {
		recordFailure(ctx, opts.Store, summary, started, err)
		return summary, err
	}
	return finishClientLocalImport(ctx, opts.Store, summary, started, exDailyWatermarks(bars), fmt.Sprintf("%d:%s", opts.Market, opts.Code))
}

func importLocation(tz string) (*time.Location, error) {
	if tz == "" {
		tz = "Asia/Shanghai"
	}
	return time.LoadLocation(tz)
}

func readClientLocalFile(path string) (string, []byte, error) {
	path = expandHome(path)
	if path == "" {
		return "", nil, fmt.Errorf("--file is required")
	}
	if isOfflinePackage(path) {
		return "", nil, fmt.Errorf("client-local import does not accept offline package %s", path)
	}
	raw, err := os.ReadFile(path)
	return path, raw, err
}

func isOfflinePackage(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".zip"
}

func filterCapitalEvents(events []model.CapitalChangeEvent, sinceValue string, untilValue string, loc *time.Location) ([]model.CapitalChangeEvent, uint64, error) {
	since, hasSince, err := parseDateBound(sinceValue, false, loc)
	if err != nil {
		return nil, 0, err
	}
	until, hasUntil, err := parseDateBound(untilValue, true, loc)
	if err != nil {
		return nil, 0, err
	}
	var out []model.CapitalChangeEvent
	var skipped uint64
	for _, event := range events {
		if hasSince && event.EventDate.Before(since) {
			skipped++
			continue
		}
		if hasUntil && event.EventDate.After(until) {
			skipped++
			continue
		}
		out = append(out, event)
	}
	return out, skipped, nil
}

func filterExDaily(bars []model.ExDailyBar, sinceValue string, untilValue string, loc *time.Location) ([]model.ExDailyBar, uint64, error) {
	since, hasSince, err := parseDateBound(sinceValue, false, loc)
	if err != nil {
		return nil, 0, err
	}
	until, hasUntil, err := parseDateBound(untilValue, true, loc)
	if err != nil {
		return nil, 0, err
	}
	var out []model.ExDailyBar
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

func capitalWatermarks(events []model.CapitalChangeEvent) *time.Time {
	if len(events) == 0 {
		return nil
	}
	max := events[0].EventDate
	for _, event := range events[1:] {
		if event.EventDate.After(max) {
			max = event.EventDate
		}
	}
	return &max
}

func exDailyWatermarks(bars []model.ExDailyBar) *time.Time {
	if len(bars) == 0 {
		return nil
	}
	max := bars[0].TradeDate
	for _, bar := range bars[1:] {
		if bar.TradeDate.After(max) {
			max = bar.TradeDate
		}
	}
	return &max
}

func finishClientLocalImport(ctx context.Context, store *chstore.Store, summary Summary, started time.Time, watermark *time.Time, asset string) (Summary, error) {
	if err := store.InsertQualityIssues(ctx, summary.Issues); err != nil {
		recordFailure(ctx, store, summary, started, err)
		return summary, err
	}
	status := "success"
	message := "ok"
	if len(summary.Issues) > 0 {
		status = "degraded"
		message = fmt.Sprintf("%d quality issue(s)", len(summary.Issues))
	}
	now := time.Now()
	if err := store.InsertWatermark(ctx, model.Watermark{
		Dataset:      summary.Dataset,
		Asset:        asset,
		Status:       status,
		MinWatermark: watermark,
		MaxWatermark: watermark,
		RowsWritten:  summary.RowsWritten,
		Message:      message,
		UpdatedAt:    now,
	}); err != nil {
		recordFailure(ctx, store, summary, started, err)
		return summary, err
	}
	if err := recordRun(ctx, store, summary, started, nil, status); err != nil {
		return summary, err
	}
	return summary, nil
}
