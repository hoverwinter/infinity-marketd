package clickhouse

import (
	"context"
	"fmt"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/hoverwinter/infinity-marketd/internal/config"
	"github.com/hoverwinter/infinity-marketd/internal/model"
)

const maxPartitionsPerInsertBlock = 90

type Store struct {
	conn     ch.Conn
	marketDB string
	opsDB    string
}

type WatermarkStatus struct {
	Dataset string
	Asset   string
	Status  string
	Message string
	Updated time.Time
}

func Open(ctx context.Context, cfg config.ClickHouseConfig) (*Store, error) {
	conn, err := ch.Open(&ch.Options{
		Addr: []string{cfg.Addr},
		Auth: ch.Auth{
			Database: "default",
			Username: cfg.User,
			Password: cfg.Password,
		},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(ctx); err != nil {
		return nil, err
	}
	return &Store{conn: conn, marketDB: cfg.Databases.Market, opsDB: cfg.Databases.Ops}, nil
}

func (s *Store) Close() error {
	return s.conn.Close()
}

func (s *Store) Bootstrap(ctx context.Context) error {
	ddl, err := BootstrapDDL(SchemaConfig{MarketDB: s.marketDB, OpsDB: s.opsDB})
	if err != nil {
		return err
	}
	for _, stmt := range ddl {
		if err := s.conn.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) InsertDailyBars(ctx context.Context, bars []model.DailyBar) error {
	if len(bars) == 0 {
		return nil
	}
	for start := 0; start < len(bars); {
		partitions := map[string]struct{}{dailyPartitionKey(bars[start].TradeDate): {}}
		end := start + 1
		for end < len(bars) {
			partition := dailyPartitionKey(bars[end].TradeDate)
			if _, exists := partitions[partition]; !exists && len(partitions) >= maxPartitionsPerInsertBlock {
				break
			}
			partitions[partition] = struct{}{}
			end++
		}
		if err := s.insertDailyBarsBatch(ctx, bars[start:end]); err != nil {
			return err
		}
		start = end
	}
	return nil
}

func (s *Store) insertDailyBarsBatch(ctx context.Context, bars []model.DailyBar) error {
	table, err := tableName(s.marketDB, "a_share_bars_1d")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (market, symbol, trade_date, open, high, low, close, volume, amount) VALUES")
	if err != nil {
		return err
	}
	for _, bar := range bars {
		if err := batch.Append(bar.Market, bar.Symbol, bar.TradeDate, bar.Open, bar.High, bar.Low, bar.Close, bar.Volume, bar.Amount); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertMinuteBars(ctx context.Context, table string, bars []model.MinuteBar) error {
	if len(bars) == 0 {
		return nil
	}
	for start := 0; start < len(bars); {
		partitions := map[string]struct{}{minutePartitionKey(bars[start].TradeDate): {}}
		end := start + 1
		for end < len(bars) {
			partition := minutePartitionKey(bars[end].TradeDate)
			if _, exists := partitions[partition]; !exists && len(partitions) >= maxPartitionsPerInsertBlock {
				break
			}
			partitions[partition] = struct{}{}
			end++
		}
		if err := s.insertMinuteBarsBatch(ctx, table, bars[start:end]); err != nil {
			return err
		}
		start = end
	}
	return nil
}

func (s *Store) insertMinuteBarsBatch(ctx context.Context, table string, bars []model.MinuteBar) error {
	target, err := tableName(s.marketDB, table)
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+target+" (market, symbol, bar_time, trade_date, open, high, low, close, volume, amount) VALUES")
	if err != nil {
		return err
	}
	for _, bar := range bars {
		if err := batch.Append(bar.Market, bar.Symbol, bar.BarTime, bar.TradeDate, bar.Open, bar.High, bar.Low, bar.Close, bar.Volume, bar.Amount); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertTaskRun(ctx context.Context, run model.TaskRun) error {
	return s.InsertTaskRuns(ctx, []model.TaskRun{run})
}

func (s *Store) InsertTaskRuns(ctx context.Context, runs []model.TaskRun) error {
	if len(runs) == 0 {
		return nil
	}
	table, err := tableName(s.opsDB, "task_runs")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (run_id, dataset, task_type, status, target_table, input_path, input_format, params, started_at, finished_at, duration_ms, rows_written, rows_skipped, error, updated_at) VALUES")
	if err != nil {
		return err
	}
	for _, run := range runs {
		if err := batch.Append(run.RunID, run.Dataset, run.TaskType, run.Status, run.TargetTable, run.InputPath, run.InputFormat, run.Params, run.StartedAt, run.FinishedAt, run.DurationMS, run.RowsWritten, run.RowsSkipped, run.Error, run.UpdatedAt); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertWatermark(ctx context.Context, wm model.Watermark) error {
	return s.InsertWatermarks(ctx, []model.Watermark{wm})
}

func (s *Store) InsertWatermarks(ctx context.Context, watermarks []model.Watermark) error {
	if len(watermarks) == 0 {
		return nil
	}
	table, err := tableName(s.opsDB, "watermarks")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (dataset, asset, status, min_watermark, max_watermark, rows_written, message, updated_at) VALUES")
	if err != nil {
		return err
	}
	for _, wm := range watermarks {
		if err := batch.Append(wm.Dataset, wm.Asset, wm.Status, wm.MinWatermark, wm.MaxWatermark, wm.RowsWritten, wm.Message, wm.UpdatedAt); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertQualityIssues(ctx context.Context, issues []model.QualityIssue) error {
	if len(issues) == 0 {
		return nil
	}
	table, err := tableName(s.opsDB, "data_quality_issues")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (issue_id, run_id, dataset, severity, issue_type, market, symbol, logical_key, input_path, input_record_offset, observed_at, message, details) VALUES")
	if err != nil {
		return err
	}
	for _, issue := range issues {
		if err := batch.Append(issueID(issue), issue.RunID, issue.Dataset, issue.Severity, issue.IssueType, nullableString(issue.Market), nullableString(issue.Symbol), issue.LogicalKey, issue.InputPath, issue.InputRecordOffset, issue.ObservedAt, issue.Message, issue.Details); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) LatestWatermarks(ctx context.Context, limit int) ([]WatermarkStatus, error) {
	table, err := tableName(s.opsDB, "watermarks")
	if err != nil {
		return nil, err
	}
	rows, err := s.conn.Query(ctx, fmt.Sprintf("SELECT dataset, asset, status, message, updated_at FROM %s ORDER BY updated_at DESC LIMIT %d", table, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WatermarkStatus
	for rows.Next() {
		var item WatermarkStatus
		if err := rows.Scan(&item.Dataset, &item.Asset, &item.Status, &item.Message, &item.Updated); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func issueID(issue model.QualityIssue) string {
	if issue.ObservedAt.IsZero() {
		return fmt.Sprintf("%s:%s:%s", issue.RunID, issue.IssueType, issue.LogicalKey)
	}
	return fmt.Sprintf("%s:%s:%s:%d", issue.RunID, issue.IssueType, issue.LogicalKey, issue.ObservedAt.UnixNano())
}

func dailyPartitionKey(t time.Time) string {
	return t.Format("2006")
}

func minutePartitionKey(t time.Time) string {
	return t.Format("200601")
}
