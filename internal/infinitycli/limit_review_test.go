package infinitycli

import (
	"context"
	"github.com/hoverwinter/infinity-marketd/internal/querier"
)

func (cliRepo) LimitEvents(_ context.Context, q querier.LimitQuery) (querier.LimitResult[querier.LimitEvent], error) {
	return querier.LimitResult[querier.LimitEvent]{Query: q, Rows: []querier.LimitEvent{}}, nil
}
func (cliRepo) LimitSummaries(_ context.Context, q querier.LimitQuery) (querier.LimitResult[querier.LimitDailySummary], error) {
	return querier.LimitResult[querier.LimitDailySummary]{Query: q, Rows: []querier.LimitDailySummary{}}, nil
}
func (cliRepo) LimitRelay(_ context.Context, q querier.LimitQuery) (querier.LimitResult[querier.LimitRelayEvent], error) {
	return querier.LimitResult[querier.LimitRelayEvent]{Query: q, Rows: []querier.LimitRelayEvent{}}, nil
}
func (cliRepo) LimitThemes(_ context.Context, q querier.LimitQuery) (querier.LimitResult[querier.LimitThemeDaily], error) {
	return querier.LimitResult[querier.LimitThemeDaily]{Query: q, Rows: []querier.LimitThemeDaily{}}, nil
}
func (cliRepo) LimitPerformanceIndices(_ context.Context, q querier.LimitQuery) (querier.LimitResult[querier.LimitPerformanceIndexBar], error) {
	return querier.LimitResult[querier.LimitPerformanceIndexBar]{Query: q, Rows: []querier.LimitPerformanceIndexBar{}}, nil
}
func (cliRepo) MarketBreadth(_ context.Context, q querier.LimitQuery) (querier.LimitResult[querier.MarketBreadthDaily], error) {
	return querier.LimitResult[querier.MarketBreadthDaily]{Query: q, Rows: []querier.MarketBreadthDaily{}}, nil
}
