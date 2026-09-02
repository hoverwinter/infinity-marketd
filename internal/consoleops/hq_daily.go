package consoleops

import (
	"context"

	chstore "github.com/hoverwinter/infinity-marketd/internal/clickhouse"
	"github.com/hoverwinter/infinity-marketd/internal/config"
	"github.com/hoverwinter/infinity-marketd/internal/ingest"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/querier"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

func HQDailyImporter(store *chstore.Store, cfg config.Config) querier.ConsoleHQDailyImporter {
	return func(ctx context.Context, req querier.ConsoleHQDailyImportRequest) (querier.ConsoleHQDailyImportSummary, error) {
		servers := append([]string(nil), req.Servers...)
		if len(servers) == 0 {
			servers = append(servers, cfg.TDX.HQServers...)
		}
		summary, err := ingest.ImportHQDailyBars(ctx, ingest.HQDailyImportOptions{
			Market:        req.Market,
			Symbol:        req.Symbol,
			Since:         req.Since,
			Until:         req.Until,
			Start:         req.Start,
			Count:         req.Count,
			DryRun:        req.DryRun,
			Store:         store,
			Timezone:      cfg.Runtime.Timezone,
			ClientOptions: tdx.QuoteClientOptions{Servers: servers, BatchSize: cfg.Runtime.BatchSize},
		})
		return hqDailySummaryFromIngest(summary), err
	}
}

func hqDailySummaryFromIngest(summary ingest.HQDailySummary) querier.ConsoleHQDailyImportSummary {
	return querier.ConsoleHQDailyImportSummary{
		RunID:        summary.RunID,
		Dataset:      summary.Dataset,
		TargetTable:  summary.TargetTable,
		Market:       summary.Market,
		Symbol:       summary.Symbol,
		Since:        summary.Since,
		Until:        summary.Until,
		Start:        summary.Start,
		Count:        summary.Count,
		PagesFetched: summary.PagesFetched,
		RowsFetched:  summary.RowsFetched,
		RowsWritten:  summary.RowsWritten,
		RowsSkipped:  summary.RowsSkipped,
		Issues:       nonNilIssues(summary.Issues),
		DryRun:       summary.DryRun,
	}
}

func nonNilIssues(issues []model.QualityIssue) []model.QualityIssue {
	if issues == nil {
		return []model.QualityIssue{}
	}
	return issues
}
