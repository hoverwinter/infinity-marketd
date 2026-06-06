package clickhouse

import (
	"context"
	"fmt"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

// SaveQuoteServiceRun upserts a realtime quote sweep run record. Because the
// table is a ReplacingMergeTree(updated_at), re-saving the same run_id with a
// newer updated_at replaces the prior version.
func (s *Store) SaveQuoteServiceRun(ctx context.Context, run model.QuoteServiceRun) error {
	table, err := tableName(s.opsDB, "quote_service_runs")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (run_id, status, markets, symbol_source, batch_size, planned_symbols, planned_batches, succeeded_batches, failed_batches, skipped_batches, rows_fetched, started_at, finished_at, duration_ms, error, updated_at) VALUES")
	if err != nil {
		return err
	}
	markets := run.Markets
	if markets == nil {
		markets = []string{}
	}
	if err := batch.Append(run.RunID, run.Status, markets, run.SymbolSource, run.BatchSize, run.PlannedSymbols, run.PlannedBatches, run.SucceededBatches, run.FailedBatches, run.SkippedBatches, run.RowsFetched, run.StartedAt, run.FinishedAt, run.DurationMS, run.Error, run.UpdatedAt); err != nil {
		return err
	}
	return batch.Send()
}

// SaveQuoteServiceBatch upserts a single batch progress record.
func (s *Store) SaveQuoteServiceBatch(ctx context.Context, b model.QuoteServiceBatch) error {
	table, err := tableName(s.opsDB, "quote_service_batches")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (run_id, batch_no, status, symbol_count, first_symbol, last_symbol, attempts, rows_fetched, started_at, finished_at, duration_ms, failure_kind, error, updated_at) VALUES")
	if err != nil {
		return err
	}
	if err := batch.Append(b.RunID, b.BatchNo, b.Status, b.SymbolCount, b.FirstSymbol, b.LastSymbol, b.Attempts, b.RowsFetched, b.StartedAt, b.FinishedAt, b.DurationMS, b.FailureKind, b.Error, b.UpdatedAt); err != nil {
		return err
	}
	return batch.Send()
}

const quoteServiceRunColumns = "run_id, status, markets, symbol_source, batch_size, planned_symbols, planned_batches, succeeded_batches, failed_batches, skipped_batches, rows_fetched, started_at, finished_at, duration_ms, error, updated_at"

func scanQuoteServiceRun(rows interface {
	Scan(dest ...any) error
}) (model.QuoteServiceRun, error) {
	var run model.QuoteServiceRun
	err := rows.Scan(&run.RunID, &run.Status, &run.Markets, &run.SymbolSource, &run.BatchSize, &run.PlannedSymbols, &run.PlannedBatches, &run.SucceededBatches, &run.FailedBatches, &run.SkippedBatches, &run.RowsFetched, &run.StartedAt, &run.FinishedAt, &run.DurationMS, &run.Error, &run.UpdatedAt)
	return run, err
}

// LoadQuoteServiceRun returns the latest version of one run, or ok=false if absent.
func (s *Store) LoadQuoteServiceRun(ctx context.Context, runID string) (model.QuoteServiceRun, bool, error) {
	table, err := tableName(s.opsDB, "quote_service_runs")
	if err != nil {
		return model.QuoteServiceRun{}, false, err
	}
	rows, err := s.conn.Query(ctx, fmt.Sprintf("SELECT %s FROM %s FINAL WHERE run_id = ? LIMIT 1", quoteServiceRunColumns, table), runID)
	if err != nil {
		return model.QuoteServiceRun{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return model.QuoteServiceRun{}, false, rows.Err()
	}
	run, err := scanQuoteServiceRun(rows)
	if err != nil {
		return model.QuoteServiceRun{}, false, err
	}
	return run, true, rows.Err()
}

// LatestQuoteServiceRuns returns the most recently started runs.
func (s *Store) LatestQuoteServiceRuns(ctx context.Context, limit int) ([]model.QuoteServiceRun, error) {
	table, err := tableName(s.opsDB, "quote_service_runs")
	if err != nil {
		return nil, err
	}
	rows, err := s.conn.Query(ctx, fmt.Sprintf("SELECT %s FROM %s FINAL ORDER BY started_at DESC LIMIT %d", quoteServiceRunColumns, table, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.QuoteServiceRun
	for rows.Next() {
		run, err := scanQuoteServiceRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// LoadQuoteServiceBatches returns all batch records for a run ordered by batch_no.
func (s *Store) LoadQuoteServiceBatches(ctx context.Context, runID string) ([]model.QuoteServiceBatch, error) {
	table, err := tableName(s.opsDB, "quote_service_batches")
	if err != nil {
		return nil, err
	}
	rows, err := s.conn.Query(ctx, fmt.Sprintf("SELECT run_id, batch_no, status, symbol_count, first_symbol, last_symbol, attempts, rows_fetched, started_at, finished_at, duration_ms, failure_kind, error, updated_at FROM %s FINAL WHERE run_id = ? ORDER BY batch_no", table), runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.QuoteServiceBatch
	for rows.Next() {
		var b model.QuoteServiceBatch
		if err := rows.Scan(&b.RunID, &b.BatchNo, &b.Status, &b.SymbolCount, &b.FirstSymbol, &b.LastSymbol, &b.Attempts, &b.RowsFetched, &b.StartedAt, &b.FinishedAt, &b.DurationMS, &b.FailureKind, &b.Error, &b.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
