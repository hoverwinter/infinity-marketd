package securitymaster

import (
	"context"
	"fmt"
	"time"
)

type Source interface {
	Fetch(ctx context.Context, markets []string) ([]SourceRow, error)
}

type RefreshOptions struct {
	SourceName string
	Markets    []string
	DryRun     bool
	Source     Source
	Store      Writer
	Now        func() time.Time
}

type RefreshSummary struct {
	RunID           int64    `json:"run_id,omitempty"`
	Source          string   `json:"source"`
	Markets         []string `json:"markets"`
	Status          string   `json:"status"`
	DryRun          bool     `json:"dry_run"`
	RowsSeen        int      `json:"rows_seen"`
	RowsUpserted    int      `json:"rows_upserted"`
	RowsSkipped     int      `json:"rows_skipped"`
	AliasesUpserted int      `json:"aliases_upserted"`
	HistoryUpserted int      `json:"history_upserted"`
	Error           string   `json:"error,omitempty"`
}

func Refresh(ctx context.Context, opts RefreshOptions) (RefreshSummary, error) {
	if opts.Source == nil {
		return RefreshSummary{}, fmt.Errorf("security master source is required")
	}
	sourceName := opts.SourceName
	if sourceName == "" {
		sourceName = "unknown"
	}
	markets, err := NormalizeMarkets(opts.Markets)
	if err != nil {
		return RefreshSummary{}, err
	}
	now := time.Now().UTC
	if opts.Now != nil {
		now = opts.Now
	}
	summary := RefreshSummary{
		Source:  sourceName,
		Markets: markets,
		Status:  RefreshStatusRunning,
		DryRun:  opts.DryRun,
	}
	var runID int64
	if !opts.DryRun {
		if opts.Store == nil {
			return summary, fmt.Errorf("security master store is required")
		}
		runID, err = opts.Store.BeginRefreshRun(ctx, RefreshRun{
			Source:    sourceName,
			Markets:   markets,
			StartedAt: now(),
			Status:    RefreshStatusRunning,
		})
		if err != nil {
			return summary, err
		}
		summary.RunID = runID
	}
	rows, err := opts.Source.Fetch(ctx, markets)
	if err != nil {
		summary.Status = RefreshStatusFailed
		summary.Error = err.Error()
		finishRefresh(ctx, opts, runID, summary, now())
		return summary, err
	}
	summary.RowsSeen = len(rows)
	normalized := make([]NormalizedRow, 0, len(rows))
	for _, row := range rows {
		item, err := NormalizeSourceRow(row, sourceName)
		if err != nil {
			summary.RowsSkipped++
			continue
		}
		normalized = append(normalized, item)
	}
	if opts.DryRun {
		summary.Status = RefreshStatusDryRun
		summary.RowsUpserted = len(normalized)
		for _, item := range normalized {
			summary.AliasesUpserted += len(item.Aliases)
			summary.HistoryUpserted += len(item.History)
		}
		return summary, nil
	}
	for _, item := range normalized {
		if err := opts.Store.UpsertSecurity(ctx, item.Security); err != nil {
			summary.Status = RefreshStatusFailed
			summary.Error = err.Error()
			finishRefresh(ctx, opts, runID, summary, now())
			return summary, err
		}
		summary.RowsUpserted++
		aliasCount, err := opts.Store.UpsertAliases(ctx, item.Aliases)
		if err != nil {
			summary.Status = RefreshStatusFailed
			summary.Error = err.Error()
			finishRefresh(ctx, opts, runID, summary, now())
			return summary, err
		}
		summary.AliasesUpserted += aliasCount
		historyCount, err := opts.Store.UpsertNameHistory(ctx, item.History)
		if err != nil {
			summary.Status = RefreshStatusFailed
			summary.Error = err.Error()
			finishRefresh(ctx, opts, runID, summary, now())
			return summary, err
		}
		summary.HistoryUpserted += historyCount
	}
	summary.Status = RefreshStatusSucceeded
	if err := finishRefresh(ctx, opts, runID, summary, now()); err != nil {
		return summary, err
	}
	return summary, nil
}

func finishRefresh(ctx context.Context, opts RefreshOptions, runID int64, summary RefreshSummary, finished time.Time) error {
	if opts.DryRun || opts.Store == nil || runID == 0 {
		return nil
	}
	return opts.Store.FinishRefreshRun(ctx, runID, RefreshRun{
		Source:          summary.Source,
		Markets:         summary.Markets,
		FinishedAt:      &finished,
		Status:          summary.Status,
		RowsSeen:        summary.RowsSeen,
		RowsUpserted:    summary.RowsUpserted,
		RowsSkipped:     summary.RowsSkipped,
		AliasesUpserted: summary.AliasesUpserted,
		HistoryUpserted: summary.HistoryUpserted,
		Error:           summary.Error,
	})
}
