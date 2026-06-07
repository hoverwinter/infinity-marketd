package clickhouse

import (
	"context"
	"database/sql"
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

type MinuteScanRefresh struct {
	Period string
	Since  time.Time
	Until  time.Time
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

func (s *Store) CountMinuteScanSourceRows(ctx context.Context, refresh MinuteScanRefresh) (uint64, error) {
	sourceTable, _, err := minuteScanTables(s.marketDB, refresh.Period)
	if err != nil {
		return 0, err
	}
	var rows uint64
	if err := s.conn.QueryRow(ctx, minuteScanCountSQL(sourceTable), refresh.Since, refresh.Until).Scan(&rows); err != nil {
		return 0, err
	}
	return rows, nil
}

func (s *Store) RefreshMinuteScan(ctx context.Context, refresh MinuteScanRefresh) (uint64, error) {
	sourceTable, targetTable, err := minuteScanTables(s.marketDB, refresh.Period)
	if err != nil {
		return 0, err
	}
	rows, err := s.CountMinuteScanSourceRows(ctx, refresh)
	if err != nil {
		return 0, err
	}
	if err := s.conn.Exec(ctx, minuteScanRefreshSQL(targetTable, sourceTable), refresh.Since, refresh.Until); err != nil {
		return 0, err
	}
	return rows, nil
}

func (s *Store) InsertFinancialRawItems(ctx context.Context, rows []model.FinancialRawItem) error {
	if len(rows) == 0 {
		return nil
	}
	for start := 0; start < len(rows); {
		partitions := map[string]struct{}{dailyPartitionKey(rows[start].ReportDate): {}}
		end := start + 1
		for end < len(rows) {
			partition := dailyPartitionKey(rows[end].ReportDate)
			if _, exists := partitions[partition]; !exists && len(partitions) >= maxPartitionsPerInsertBlock {
				break
			}
			partitions[partition] = struct{}{}
			end++
		}
		if err := s.insertFinancialRawItemsBatch(ctx, rows[start:end]); err != nil {
			return err
		}
		start = end
	}
	return nil
}

func (s *Store) insertFinancialRawItemsBatch(ctx context.Context, rows []model.FinancialRawItem) error {
	table, err := tableName(s.marketDB, "a_share_financial_raw_items")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (market, symbol, report_date, item_id, value) VALUES")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := batch.Append(row.Market, row.Symbol, row.ReportDate, row.ItemID, row.Value); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertGPMetricValues(ctx context.Context, rows []model.GPMetricValue) error {
	if len(rows) == 0 {
		return nil
	}
	for start := 0; start < len(rows); {
		partitions := map[string]struct{}{dailyPartitionKey(rows[start].EventDate): {}}
		end := start + 1
		for end < len(rows) {
			partition := dailyPartitionKey(rows[end].EventDate)
			if _, exists := partitions[partition]; !exists && len(partitions) >= maxPartitionsPerInsertBlock {
				break
			}
			partitions[partition] = struct{}{}
			end++
		}
		if err := s.insertGPMetricValuesBatch(ctx, rows[start:end]); err != nil {
			return err
		}
		start = end
	}
	return nil
}

func (s *Store) insertGPMetricValuesBatch(ctx context.Context, rows []model.GPMetricValue) error {
	table, err := tableName(s.marketDB, "a_share_gp_metric_values")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (market, symbol, metric_type, event_date, value1, value2) VALUES")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := batch.Append(row.Market, row.Symbol, row.MetricType, row.EventDate, row.Value1, row.Value2); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertFinancialItemDictionary(ctx context.Context, rows []model.FinancialItemDictionaryEntry) error {
	if len(rows) == 0 {
		return nil
	}
	table, err := tableName(s.marketDB, "tdx_financial_item_dictionary")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (item_id, name, title, category, unit, value_kind, source_ref, status) VALUES")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := batch.Append(row.ItemID, row.Name, row.Title, row.Category, row.Unit, row.ValueKind, row.SourceRef, row.Status); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertGPMetricDictionary(ctx context.Context, rows []model.GPMetricDictionaryEntry) error {
	if len(rows) == 0 {
		return nil
	}
	table, err := tableName(s.marketDB, "tdx_gp_metric_dictionary")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (metric_type, name, title, value1_meaning, value2_meaning, source_ref, status) VALUES")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := batch.Append(row.MetricType, row.Name, row.Title, row.Value1Meaning, row.Value2Meaning, row.SourceRef, row.Status); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertIntradayPoints(ctx context.Context, points []model.IntradayPoint) error {
	if len(points) == 0 {
		return nil
	}
	for start := 0; start < len(points); {
		partitions := map[string]struct{}{minutePartitionKey(points[start].TradeDate): {}}
		end := start + 1
		for end < len(points) {
			partition := minutePartitionKey(points[end].TradeDate)
			if _, exists := partitions[partition]; !exists && len(partitions) >= maxPartitionsPerInsertBlock {
				break
			}
			partitions[partition] = struct{}{}
			end++
		}
		if err := s.insertIntradayPointsBatch(ctx, points[start:end]); err != nil {
			return err
		}
		start = end
	}
	return nil
}

func (s *Store) insertIntradayPointsBatch(ctx context.Context, points []model.IntradayPoint) error {
	target, err := tableName(s.marketDB, "a_share_intraday_points")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+target+" (market, symbol, trade_date, point_time, point_index, price, volume) VALUES")
	if err != nil {
		return err
	}
	for _, point := range points {
		if err := batch.Append(point.Market, point.Symbol, point.TradeDate, point.PointTime, point.PointIndex, point.Price, point.Volume); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertXDXREvents(ctx context.Context, events []model.XDXREvent) error {
	if len(events) == 0 {
		return nil
	}
	table, err := tableName(s.marketDB, "a_share_xdxr_events")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (market, symbol, event_date, category, category_name, fenhong, peigujia, songzhuangu, peigu, suogu, panqianliutong, panhouliutong, qianzongguben, houzongguben, fenshu, xingquanjia) VALUES")
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := batch.Append(event.Market, event.Symbol, event.EventDate, event.Category, event.CategoryName, event.FenHong, event.PeiGuJia, event.SongZhuanGu, event.PeiGu, event.SuoGu, event.PanQianLiuTong, event.PanHouLiuTong, event.QianZongGuBen, event.HouZongGuBen, event.FenShu, event.XingQuanJia); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertAdjustFactors(ctx context.Context, factors []model.AdjustFactor) error {
	if len(factors) == 0 {
		return nil
	}
	table, err := tableName(s.marketDB, "a_share_adjust_factors_1d")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (market, symbol, trade_date, qfq_factor, hfq_factor, computed_at) VALUES")
	if err != nil {
		return err
	}
	for _, factor := range factors {
		computedAt := factor.ComputedAt
		if computedAt.IsZero() {
			computedAt = time.Now()
		}
		if err := batch.Append(factor.Market, factor.Symbol, factor.TradeDate, factor.QFQFactor, factor.HFQFactor, computedAt); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertDailyDerived(ctx context.Context, rows []model.DailyDerived) error {
	if len(rows) == 0 {
		return nil
	}
	for start := 0; start < len(rows); {
		partitions := map[string]struct{}{dailyPartitionKey(rows[start].TradeDate): {}}
		end := start + 1
		for end < len(rows) {
			partition := dailyPartitionKey(rows[end].TradeDate)
			if _, exists := partitions[partition]; !exists && len(partitions) >= maxPartitionsPerInsertBlock {
				break
			}
			partitions[partition] = struct{}{}
			end++
		}
		if err := s.insertDailyDerivedBatch(ctx, rows[start:end]); err != nil {
			return err
		}
		start = end
	}
	return nil
}

func (s *Store) insertDailyDerivedBatch(ctx context.Context, rows []model.DailyDerived) error {
	table, err := tableName(s.marketDB, "a_share_daily_derived")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (market, symbol, trade_date, prev_close, pct_chg, computed_at) VALUES")
	if err != nil {
		return err
	}
	for _, row := range rows {
		computedAt := row.ComputedAt
		if computedAt.IsZero() {
			computedAt = time.Now()
		}
		if err := batch.Append(row.Market, row.Symbol, row.TradeDate, row.PrevClose, row.PctChg, computedAt); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertCapitalChangeEvents(ctx context.Context, events []model.CapitalChangeEvent) error {
	if len(events) == 0 {
		return nil
	}
	table, err := tableName(s.marketDB, "a_share_capital_change_events")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (market, symbol, event_date, category, event_seq, event_name, cash_dividend, allotment_price, bonus_shares, allotment_shares, shrink_shares, pre_float_shares, post_float_shares, pre_total_shares, post_total_shares, ratio_denominator, exercise_price) VALUES")
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := batch.Append(event.Market, event.Symbol, event.EventDate, event.Category, event.EventSeq, event.EventName, event.CashDividend, event.AllotmentPrice, event.BonusShares, event.AllotmentShares, event.ShrinkShares, event.PreFloatShares, event.PostFloatShares, event.PreTotalShares, event.PostTotalShares, event.RatioDenominator, event.ExercisePrice); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertTDXBlockSnapshots(ctx context.Context, snapshots []model.TDXBlockSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	table, err := tableName(s.marketDB, "tdx_block_snapshots")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (snapshot_id, block_scope, snapshot_time, content_hash, block_count, member_count) VALUES")
	if err != nil {
		return err
	}
	for _, snapshot := range snapshots {
		if err := batch.Append(snapshot.SnapshotID, snapshot.BlockScope, snapshot.SnapshotTime, snapshot.ContentHash, snapshot.BlockCount, snapshot.MemberCount); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertTDXBlockDefinitions(ctx context.Context, definitions []model.TDXBlockDefinition) error {
	if len(definitions) == 0 {
		return nil
	}
	table, err := tableName(s.marketDB, "tdx_block_definitions")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (snapshot_id, block_scope, block_kind, block_id, block_name, block_type, display_order, member_count) VALUES")
	if err != nil {
		return err
	}
	for _, def := range definitions {
		if err := batch.Append(def.SnapshotID, def.BlockScope, def.BlockKind, def.BlockID, def.BlockName, def.BlockType, def.DisplayOrder, def.MemberCount); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertTDXBlockMemberships(ctx context.Context, memberships []model.TDXBlockMembership) error {
	if len(memberships) == 0 {
		return nil
	}
	table, err := tableName(s.marketDB, "tdx_block_memberships")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (snapshot_id, block_scope, block_id, member_order, code, market, symbol) VALUES")
	if err != nil {
		return err
	}
	for _, member := range memberships {
		if err := batch.Append(member.SnapshotID, member.BlockScope, member.BlockID, member.MemberOrder, member.Code, member.Market, member.Symbol); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertExDailyBars(ctx context.Context, bars []model.ExDailyBar) error {
	if len(bars) == 0 {
		return nil
	}
	table, err := tableName(s.marketDB, "tdx_ex_bars_1d")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (ex_market, code, trade_date, open, high, low, close, position, trade, price, amount, settlement_price) VALUES")
	if err != nil {
		return err
	}
	for _, bar := range bars {
		if err := batch.Append(bar.ExMarket, bar.Code, bar.TradeDate, bar.Open, bar.High, bar.Low, bar.Close, bar.Position, bar.Trade, bar.Price, bar.Amount, bar.SettlementPrice); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) DailyBarsForSymbol(ctx context.Context, market string, symbol string) ([]model.DailyBar, error) {
	table, err := tableName(s.marketDB, "a_share_bars_1d")
	if err != nil {
		return nil, err
	}
	rows, err := s.conn.Query(ctx, fmt.Sprintf("SELECT market, symbol, trade_date, open, high, low, close, volume, amount FROM %s FINAL WHERE market = ? AND symbol = ? ORDER BY trade_date ASC", table), market, symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var bars []model.DailyBar
	for rows.Next() {
		var bar model.DailyBar
		if err := rows.Scan(&bar.Market, &bar.Symbol, &bar.TradeDate, &bar.Open, &bar.High, &bar.Low, &bar.Close, &bar.Volume, &bar.Amount); err != nil {
			return nil, err
		}
		bars = append(bars, bar)
	}
	return bars, rows.Err()
}

func (s *Store) DailySymbols(ctx context.Context, market string) ([]model.Symbol, error) {
	table, err := tableName(s.marketDB, "a_share_bars_1d")
	if err != nil {
		return nil, err
	}
	where := ""
	var args []any
	if market != "" {
		where = " WHERE market = ?"
		args = append(args, market)
	}
	rows, err := s.conn.Query(ctx, fmt.Sprintf("SELECT market, symbol FROM %s%s GROUP BY market, symbol ORDER BY market, symbol", table, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var symbols []model.Symbol
	for rows.Next() {
		var item model.Symbol
		if err := rows.Scan(&item.Market, &item.Symbol); err != nil {
			return nil, err
		}
		symbols = append(symbols, item)
	}
	return symbols, rows.Err()
}

func (s *Store) XDXREventsForSymbol(ctx context.Context, market string, symbol string) ([]model.XDXREvent, error) {
	table, err := tableName(s.marketDB, "a_share_xdxr_events")
	if err != nil {
		return nil, err
	}
	columns := "market, symbol, event_date, category, category_name, fenhong, peigujia, songzhuangu, peigu, suogu, panqianliutong, panhouliutong, qianzongguben, houzongguben, fenshu, xingquanjia"
	rows, err := s.conn.Query(ctx, fmt.Sprintf("SELECT %s FROM %s FINAL WHERE market = ? AND symbol = ? ORDER BY event_date ASC, category ASC", columns, table), market, symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []model.XDXREvent
	for rows.Next() {
		var event model.XDXREvent
		var fenHong, peiGuJia, songZhuanGu, peiGu, suoGu sql.NullFloat64
		var panQian, panHou, qianZong, houZong, fenShu, xingQuan sql.NullFloat64
		if err := rows.Scan(&event.Market, &event.Symbol, &event.EventDate, &event.Category, &event.CategoryName, &fenHong, &peiGuJia, &songZhuanGu, &peiGu, &suoGu, &panQian, &panHou, &qianZong, &houZong, &fenShu, &xingQuan); err != nil {
			return nil, err
		}
		event.FenHong = nullFloatPtr(fenHong)
		event.PeiGuJia = nullFloatPtr(peiGuJia)
		event.SongZhuanGu = nullFloatPtr(songZhuanGu)
		event.PeiGu = nullFloatPtr(peiGu)
		event.SuoGu = nullFloatPtr(suoGu)
		event.PanQianLiuTong = nullFloatPtr(panQian)
		event.PanHouLiuTong = nullFloatPtr(panHou)
		event.QianZongGuBen = nullFloatPtr(qianZong)
		event.HouZongGuBen = nullFloatPtr(houZong)
		event.FenShu = nullFloatPtr(fenShu)
		event.XingQuanJia = nullFloatPtr(xingQuan)
		events = append(events, event)
	}
	return events, rows.Err()
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

func nullFloatPtr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}

func minuteScanTables(database string, period string) (string, string, error) {
	source := ""
	target := ""
	switch period {
	case "1m":
		source = "a_share_bars_1m"
		target = "a_share_bars_1m_scan"
	case "5m":
		source = "a_share_bars_5m"
		target = "a_share_bars_5m_scan"
	default:
		return "", "", fmt.Errorf("unsupported minute scan period %q", period)
	}
	sourceTable, err := tableName(database, source)
	if err != nil {
		return "", "", err
	}
	targetTable, err := tableName(database, target)
	if err != nil {
		return "", "", err
	}
	return sourceTable, targetTable, nil
}

func minuteScanCountSQL(sourceTable string) string {
	return fmt.Sprintf("SELECT count() FROM %s FINAL WHERE trade_date >= ? AND trade_date <= ?", sourceTable)
}

func minuteScanRefreshSQL(targetTable string, sourceTable string) string {
	return fmt.Sprintf(`INSERT INTO %s (trade_date, bar_time, market, symbol, close, volume, amount, prev_close, minute_ret, volume_ratio, computed_at)
SELECT trade_date,
       bar_time,
       market,
       symbol,
       close,
       volume,
       amount,
       prev_close,
       if(isNotNull(prev_close) AND prev_close > 0, (close - prev_close) / prev_close * 100, CAST(NULL, 'Nullable(Float64)')) AS minute_ret,
       CAST(NULL, 'Nullable(Float64)') AS volume_ratio,
       now64(3) AS computed_at
FROM
(
    SELECT trade_date,
           bar_time,
           market,
           symbol,
           close,
           volume,
           amount,
           lagInFrame(toNullable(close), 1) OVER (PARTITION BY market, symbol ORDER BY bar_time ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING) AS prev_close
    FROM %s FINAL
    WHERE trade_date >= ? AND trade_date <= ?
)
ORDER BY trade_date, bar_time, market, symbol`, targetTable, sourceTable)
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
