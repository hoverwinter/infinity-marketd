package clickhouse

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hoverwinter/infinity-marketd/internal/querier"
)

func (s *Store) ConsoleWatermarks(ctx context.Context, limit int) ([]querier.ConsoleWatermark, error) {
	table, err := tableName(s.opsDB, "watermarks")
	if err != nil {
		return nil, err
	}
	rows, err := s.conn.Query(ctx, consoleWatermarksSQL(table, safeConsoleLimit(limit)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []querier.ConsoleWatermark
	for rows.Next() {
		var item querier.ConsoleWatermark
		if err := rows.Scan(&item.Dataset, &item.Asset, &item.Status, &item.MinWatermark, &item.MaxWatermark, &item.RowsWritten, &item.Message, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ConsoleTaskRuns(ctx context.Context, limit int) ([]querier.ConsoleTaskRun, error) {
	table, err := tableName(s.opsDB, "task_runs")
	if err != nil {
		return nil, err
	}
	rows, err := s.conn.Query(ctx, consoleTaskRunsSQL(table, safeConsoleLimit(limit)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []querier.ConsoleTaskRun
	for rows.Next() {
		var item querier.ConsoleTaskRun
		if err := rows.Scan(&item.RunID, &item.Dataset, &item.TaskType, &item.Status, &item.TargetTable, &item.InputPath, &item.InputFormat, &item.Params, &item.StartedAt, &item.FinishedAt, &item.DurationMS, &item.RowsWritten, &item.RowsSkipped, &item.Error, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ConsoleDataQualityIssues(ctx context.Context, limit int) ([]querier.ConsoleDataQualityIssue, error) {
	table, err := tableName(s.opsDB, "data_quality_issues")
	if err != nil {
		return nil, err
	}
	rows, err := s.conn.Query(ctx, consoleDataQualityIssuesSQL(table, safeConsoleLimit(limit)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []querier.ConsoleDataQualityIssue
	for rows.Next() {
		var item querier.ConsoleDataQualityIssue
		var market, symbol sql.NullString
		if err := rows.Scan(&item.IssueID, &item.RunID, &item.Dataset, &item.Severity, &item.IssueType, &market, &symbol, &item.LogicalKey, &item.InputPath, &item.InputRecordOffset, &item.ObservedAt, &item.Message, &item.Details); err != nil {
			return nil, err
		}
		item.Market = nullStringValue(market)
		item.Symbol = nullStringValue(symbol)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ConsoleDataQualityIssueStats(ctx context.Context, limit int) ([]querier.ConsoleQualityIssueStat, error) {
	table, err := tableName(s.opsDB, "data_quality_issues")
	if err != nil {
		return nil, err
	}
	rows, err := s.conn.Query(ctx, consoleDataQualityIssueStatsSQL(table, safeConsoleLimit(limit)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []querier.ConsoleQualityIssueStat
	for rows.Next() {
		var item querier.ConsoleQualityIssueStat
		if err := rows.Scan(&item.Dataset, &item.Severity, &item.IssueType, &item.Count); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ConsoleQuoteServiceRuns(ctx context.Context, limit int) ([]querier.ConsoleQuoteServiceRun, error) {
	table, err := tableName(s.opsDB, "quote_service_runs")
	if err != nil {
		return nil, err
	}
	rows, err := s.conn.Query(ctx, consoleQuoteServiceRunsSQL(table, safeConsoleLimit(limit)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []querier.ConsoleQuoteServiceRun
	for rows.Next() {
		var item querier.ConsoleQuoteServiceRun
		if err := rows.Scan(&item.RunID, &item.Status, &item.Markets, &item.SymbolSource, &item.BatchSize, &item.PlannedSymbols, &item.PlannedBatches, &item.SucceededBatches, &item.FailedBatches, &item.SkippedBatches, &item.RowsFetched, &item.StartedAt, &item.FinishedAt, &item.DurationMS, &item.Error, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func safeConsoleLimit(limit int) int {
	if limit <= 0 {
		return querier.DefaultConsoleLimit
	}
	if limit > querier.MaxConsoleLimit {
		return querier.MaxConsoleLimit
	}
	return limit
}

func consoleWatermarksSQL(table string, limit int) string {
	return fmt.Sprintf("SELECT dataset, asset, status, min_watermark, max_watermark, rows_written, message, updated_at FROM %s FINAL ORDER BY updated_at DESC LIMIT %d", table, limit)
}

func consoleTaskRunsSQL(table string, limit int) string {
	return fmt.Sprintf("SELECT run_id, dataset, task_type, status, target_table, input_path, input_format, params, started_at, finished_at, duration_ms, rows_written, rows_skipped, error, updated_at FROM %s FINAL ORDER BY started_at DESC LIMIT %d", table, limit)
}

func consoleDataQualityIssuesSQL(table string, limit int) string {
	return fmt.Sprintf("SELECT issue_id, run_id, dataset, severity, issue_type, market, symbol, logical_key, input_path, input_record_offset, observed_at, message, details FROM %s ORDER BY observed_at DESC LIMIT %d", table, limit)
}

func consoleDataQualityIssueStatsSQL(table string, limit int) string {
	return fmt.Sprintf("SELECT dataset, severity, issue_type, count() AS issue_count FROM %s GROUP BY dataset, severity, issue_type ORDER BY issue_count DESC LIMIT %d", table, limit)
}

func consoleQuoteServiceRunsSQL(table string, limit int) string {
	return fmt.Sprintf("SELECT %s FROM %s FINAL ORDER BY started_at DESC LIMIT %d", quoteServiceRunColumns, table, limit)
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
