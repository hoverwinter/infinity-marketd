package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/querier"
)

func (s *Store) Health(ctx context.Context) error {
	return s.conn.Ping(ctx)
}

func (s *Store) LimitEvents(ctx context.Context, q querier.LimitQuery) (querier.LimitResult[querier.LimitEvent], error) {
	return queryLimitRows[querier.LimitEvent](ctx, s, q, "events")
}

func (s *Store) LimitSummaries(ctx context.Context, q querier.LimitQuery) (querier.LimitResult[querier.LimitDailySummary], error) {
	result, err := queryLimitRows[querier.LimitDailySummary](ctx, s, q, "summary")
	if err != nil {
		return result, err
	}
	if len(result.Rows) == 0 {
		return result, nil
	}
	dates := make([]any, 0, len(result.Rows))
	for _, row := range result.Rows {
		dates = append(dates, row.TradeDate)
	}
	table, err := tableName(s.marketDB, "a_share_limit_events")
	if err != nil {
		return result, err
	}
	var counts []querier.LimitDailySummary
	placeholders := strings.TrimSuffix(strings.Repeat("toDate(?),", len(dates)), ",")
	stmt := fmt.Sprintf(`SELECT
    toString(r.trade_date) AS trade_date,
    toUInt32(countIf(event_type='limit_up')) AS limit_up_count,
    toUInt32(countIf(event_type='limit_down')) AS limit_down_count,
    toUInt32(countIf(event_type='open_limit')) AS open_limit_count,
    toUInt16(maxIf(board_count,event_type='limit_up')) AS max_board_height,
    toUInt32(countIf(event_type='limit_up' AND board_count=1)) AS first_board_count,
    toUInt32(countIf(event_type='limit_up' AND board_count>=2)) AS continuous_board_count,
    toUInt32(countIf(event_type='limit_up' AND board_count=2)) AS two_board_count,
    toUInt32(countIf(event_type='limit_up' AND board_count=3)) AS three_board_count,
    toUInt32(countIf(event_type='limit_up' AND board_count=4)) AS four_board_count,
    toUInt32(countIf(event_type='limit_up' AND board_count>=5)) AS five_plus_board_count,
    countIf(event_type='limit_up') / nullIf(countIf(event_type='limit_up')+
        countIf(event_type='open_limit' AND close_status!='broken_reseal'),0) AS seal_success_rate
FROM %s AS r FINAL WHERE r.trade_date IN (%s) GROUP BY r.trade_date`, table, placeholders)
	if err := s.conn.Select(ctx, &counts, stmt, dates...); err != nil {
		return result, err
	}
	byDate := map[string]querier.LimitDailySummary{}
	for _, row := range counts {
		byDate[row.TradeDate] = row
	}
	for i := range result.Rows {
		if fresh, ok := byDate[result.Rows[i].TradeDate]; ok {
			old := result.Rows[i]
			// A missing rate may mean the historical broken-board pool was never observed.
			if old.SealSuccessRate == nil {
				fresh.SealSuccessRate = nil
			}
			fresh.PrevTradeDate = old.PrevTradeDate
			fresh.PrevLimitUpPromotionRate = old.PrevLimitUpPromotionRate
			fresh.PrevLadderPromotionRate = old.PrevLadderPromotionRate
			fresh.BigNoodleCount = old.BigNoodleCount
			fresh.HighLevelBreakCount = old.HighLevelBreakCount
			fresh.StrongThemeCount = old.StrongThemeCount
			result.Rows[i] = fresh
		}
	}
	return result, nil
}

func (s *Store) LimitRelay(ctx context.Context, q querier.LimitQuery) (querier.LimitResult[querier.LimitRelayEvent], error) {
	q, err := querier.NormalizeLimitQuery(q, "relay")
	result := querier.LimitResult[querier.LimitRelayEvent]{Query: q, Rows: []querier.LimitRelayEvent{}}
	if err != nil {
		return result, err
	}
	base := querier.LimitQuery{TradeDate: q.TradeDate, Since: q.Since, Until: q.Until}
	summaries, err := allLimitRows[querier.LimitDailySummary](ctx, s, base, "summary")
	if err != nil {
		return result, err
	}
	stored, err := allLimitRows[querier.LimitRelayEvent](ctx, s, base, "relay")
	if err != nil {
		return result, err
	}
	prevByDate := map[string]string{}
	storedByDate := map[string][]querier.LimitRelayEvent{}
	for _, row := range stored {
		storedByDate[row.TradeDate] = append(storedByDate[row.TradeDate], row)
		if prev, ok := prevByDate[row.TradeDate]; ok && prev != row.PrevTradeDate {
			return result, fmt.Errorf("conflicting previous trading dates on %s", row.TradeDate)
		}
		prevByDate[row.TradeDate] = row.PrevTradeDate
	}
	for _, row := range summaries {
		if row.PrevTradeDate != nil {
			prevByDate[row.TradeDate] = *row.PrevTradeDate
		}
	}
	if q.PrevTradeDate != "" {
		prevByDate[q.TradeDate] = q.PrevTradeDate
	}
	since, until := q.Since, q.Until
	if q.TradeDate != "" {
		since, until = q.TradeDate, q.TradeDate
	}
	earliest := since
	for date, prev := range prevByDate {
		if prev >= date || prev == "" {
			return result, fmt.Errorf("invalid previous trading date on %s", date)
		}
		if prev < earliest {
			earliest = prev
		}
	}
	events, err := allLimitRows[querier.LimitEvent](ctx, s, querier.LimitQuery{Since: earliest, Until: until, Market: q.Market, Symbol: q.Symbol}, "events")
	if err != nil {
		return result, err
	}
	eventsByDate := map[string][]querier.LimitEvent{}
	for _, row := range events {
		eventsByDate[row.TradeDate] = append(eventsByDate[row.TradeDate], row)
	}
	derived, err := tableName(s.marketDB, "a_share_daily_derived")
	if err != nil {
		return result, err
	}
	var performance []querier.ReviewDailyPerformance
	stmt := "SELECT toString(r.trade_date) AS trade_date, market, symbol, pct_chg FROM " + derived + " AS r FINAL WHERE r.trade_date >= toDate(?) AND r.trade_date <= toDate(?)"
	args := []any{since, until}
	if q.Market != "" {
		stmt += " AND market = ?"
		args = append(args, q.Market)
	}
	if q.Symbol != "" {
		stmt += " AND symbol = ?"
		args = append(args, q.Symbol)
	}
	if err := s.conn.Select(ctx, &performance, stmt+" LIMIT 600001", args...); err != nil {
		return result, err
	}
	if len(performance) > 600000 {
		return result, fmt.Errorf("daily performance exceeds reconstruction limit")
	}
	performanceByDate := map[string][]querier.ReviewDailyPerformance{}
	for _, row := range performance {
		performanceByDate[row.TradeDate] = append(performanceByDate[row.TradeDate], row)
	}
	for date, prev := range prevByDate {
		rows := storedByDate[date]
		if len(eventsByDate[prev]) > 0 {
			day := querier.LimitQuery{TradeDate: date, Limit: 200000}
			rebuilt := querier.ReconstructLimitRelay(day, prev, eventsByDate[prev], eventsByDate[date], rows, performanceByDate[date])
			if rebuilt.HasMore {
				return result, fmt.Errorf("daily relay exceeds reconstruction limit")
			}
			rows = rebuilt.Rows
		}
		for _, row := range rows {
			if (q.Market == "" || row.Market == q.Market) && (q.Symbol == "" || row.Symbol == q.Symbol) && (q.Theme == "" || row.PrevThemePrimary == q.Theme) && (q.SampleGroup == "" || row.SampleGroup == q.SampleGroup) && (q.PrevTradeDate == "" || row.PrevTradeDate == q.PrevTradeDate) {
				result.Rows = append(result.Rows, row)
			}
		}
	}
	sort.Slice(result.Rows, func(i, j int) bool {
		a, b := result.Rows[i], result.Rows[j]
		if a.TradeDate != b.TradeDate {
			return a.TradeDate < b.TradeDate
		}
		if a.SampleGroup != b.SampleGroup {
			return a.SampleGroup < b.SampleGroup
		}
		if a.Market != b.Market {
			return a.Market < b.Market
		}
		return a.Symbol < b.Symbol
	})
	return pageLimitResult(result), nil
}

func (s *Store) LimitThemes(ctx context.Context, q querier.LimitQuery) (querier.LimitResult[querier.LimitThemeDaily], error) {
	q, err := querier.NormalizeLimitQuery(q, "themes")
	result := querier.LimitResult[querier.LimitThemeDaily]{Query: q, Rows: []querier.LimitThemeDaily{}}
	if err != nil {
		return result, err
	}
	base := querier.LimitQuery{TradeDate: q.TradeDate, Since: q.Since, Until: q.Until}
	stored, err := allLimitRows[querier.LimitThemeDaily](ctx, s, base, "themes")
	if err != nil {
		return result, err
	}
	events, err := allLimitRows[querier.LimitEvent](ctx, s, base, "events")
	if err != nil {
		return result, err
	}
	dates := map[string]bool{}
	for _, event := range events {
		dates[event.TradeDate] = true
	}
	rows := querier.AggregateLimitThemes(events)
	for _, row := range stored {
		if !dates[row.TradeDate] {
			rows = append(rows, row)
		}
	}
	for _, row := range rows {
		if q.Theme == "" || row.ThemeName == q.Theme {
			result.Rows = append(result.Rows, row)
		}
	}
	sort.Slice(result.Rows, func(i, j int) bool {
		a, b := result.Rows[i], result.Rows[j]
		if a.TradeDate != b.TradeDate {
			return a.TradeDate < b.TradeDate
		}
		if a.StrengthRank != b.StrengthRank {
			return a.StrengthRank < b.StrengthRank
		}
		return a.ThemeName < b.ThemeName
	})
	return pageLimitResult(result), nil
}

func allLimitRows[T any](ctx context.Context, s *Store, q querier.LimitQuery, kind string) ([]T, error) {
	return querier.ReadCompleteLimitRows(ctx, q, func(ctx context.Context, page querier.LimitQuery) (querier.LimitResult[T], error) {
		return queryLimitRows[T](ctx, s, page, kind)
	})
}

func pageLimitResult[T any](r querier.LimitResult[T]) querier.LimitResult[T] {
	if r.Query.Offset >= len(r.Rows) {
		r.Rows = []T{}
		return r
	}
	r.Rows = r.Rows[r.Query.Offset:]
	r.HasMore = len(r.Rows) > r.Query.Limit
	if r.HasMore {
		r.Rows = r.Rows[:r.Query.Limit]
	}
	return r
}

func (s *Store) LimitPerformanceIndices(ctx context.Context, q querier.LimitQuery) (querier.LimitResult[querier.LimitPerformanceIndexBar], error) {
	return queryLimitRows[querier.LimitPerformanceIndexBar](ctx, s, q, "indices")
}

func (s *Store) MarketBreadth(ctx context.Context, q querier.LimitQuery) (querier.LimitResult[querier.MarketBreadthDaily], error) {
	return queryLimitRows[querier.MarketBreadthDaily](ctx, s, q, "breadth")
}

func queryLimitRows[T any](ctx context.Context, s *Store, q querier.LimitQuery, kind string) (querier.LimitResult[T], error) {
	q, err := querier.NormalizeLimitQuery(q, kind)
	result := querier.LimitResult[T]{Query: q, Rows: []T{}}
	if err != nil {
		return result, err
	}
	stmt, args, err := limitReviewSQL(s.marketDB, kind, q)
	if err != nil {
		return result, err
	}
	if err = s.conn.Select(ctx, &result.Rows, stmt, args...); err != nil {
		return result, err
	}
	if len(result.Rows) > q.Limit {
		result.HasMore = true
		result.Rows = result.Rows[:q.Limit]
	}
	return result, nil
}

func limitReviewSQL(db, kind string, q querier.LimitQuery) (string, []any, error) {
	var name, columns, order string
	switch kind {
	case "events":
		name = "a_share_limit_events"
		columns = "toString(r.trade_date) AS trade_date, market, symbol, event_type, close_status, board_count, reason_text, theme_primary, theme_tags, first_limit_minute, last_limit_minute, open_count, seal_order_amount, amount, turnover_rate, market_value"
		order = "trade_date, event_type, market, symbol"
	case "summary":
		name = "a_share_limit_daily_summary"
		columns = "toString(r.trade_date) AS trade_date, toString(r.prev_trade_date) AS prev_trade_date, limit_up_count, limit_down_count, open_limit_count, seal_success_rate, max_board_height, first_board_count, continuous_board_count, prev_limit_up_promotion_rate, prev_ladder_promotion_rate, big_noodle_count, high_level_break_count, strong_theme_count, two_board_count, three_board_count, four_board_count, five_plus_board_count"
		order = "trade_date"
	case "relay":
		name = "a_share_limit_relay_events"
		columns = "toString(r.trade_date) AS trade_date, toString(r.prev_trade_date) AS prev_trade_date, market, symbol, sample_group, prev_board_count, prev_reason_text, prev_theme_primary, prev_first_limit_minute, today_status, today_board_count, today_pct_chg"
		order = "trade_date, sample_group, market, symbol"
	case "themes":
		name = "a_share_limit_theme_daily"
		columns = "toString(r.trade_date) AS trade_date, theme_name, limit_up_count, ladder_count, broken_count, limit_down_count, leader_market, leader_symbol, leader_board_count, strength_rank"
		order = "trade_date, strength_rank, theme_name"
	case "indices":
		name = "a_share_limit_performance_index_bars_1d"
		columns = "index_code, toString(r.trade_date) AS trade_date, open, high, low, close, volume, amount"
		order = "trade_date, index_code"
	case "breadth":
		name = "a_share_market_breadth_daily"
		columns = "toString(r.trade_date) AS trade_date, up_count, down_count, flat_count, unchanged_or_suspended_count, up_gt_3_count, up_gt_5_count, up_gt_7_count, down_gt_3_count, down_gt_5_count, down_gt_7_count, limit_up_count, limit_down_count, total_count"
		order = "trade_date"
	default:
		return "", nil, fmt.Errorf("unsupported review dataset %q", kind)
	}
	table, err := tableName(db, name)
	if err != nil {
		return "", nil, err
	}
	clauses := []string{}
	args := []any{}
	if q.TradeDate != "" {
		clauses = append(clauses, "r.trade_date = toDate(?)")
		args = append(args, q.TradeDate)
	} else {
		clauses = append(clauses, "r.trade_date >= toDate(?)", "r.trade_date <= toDate(?)")
		args = append(args, q.Since, q.Until)
	}
	for _, filter := range []struct{ column, value string }{{"market", q.Market}, {"symbol", q.Symbol}, {"event_type", q.EventType}, {"sample_group", q.SampleGroup}, {"index_code", q.IndexCode}} {
		if filter.value != "" {
			clauses = append(clauses, filter.column+" = ?")
			args = append(args, filter.value)
		}
	}
	if q.PrevTradeDate != "" {
		clauses = append(clauses, "r.prev_trade_date = toDate(?)")
		args = append(args, q.PrevTradeDate)
	}
	if q.Theme != "" {
		column := "theme_primary"
		if kind == "themes" {
			column = "theme_name"
		} else if kind == "relay" {
			column = "prev_theme_primary"
		}
		if kind == "events" {
			clauses = append(clauses, "(theme_primary = ? OR has(theme_tags, ?))")
			args = append(args, q.Theme, q.Theme)
		} else {
			clauses = append(clauses, column+" = ?")
			args = append(args, q.Theme)
		}
	}
	return fmt.Sprintf("SELECT %s FROM %s AS r FINAL WHERE %s ORDER BY %s LIMIT %d OFFSET %d", columns, table, strings.Join(clauses, " AND "), order, q.Limit+1, q.Offset), args, nil
}

func (s *Store) Bars(ctx context.Context, query querier.BarQuery) (querier.BarResult, error) {
	normalized, err := querier.NormalizeQuery(query)
	if err != nil {
		return querier.BarResult{}, err
	}
	if normalized.Period == "1d" {
		return s.dailyBars(ctx, normalized)
	}
	return s.minuteBars(ctx, normalized)
}

func (s *Store) IntradayPoints(ctx context.Context, query querier.IntradayPointQuery) (querier.IntradayPointResult, error) {
	normalized, err := querier.NormalizeIntradayPointQuery(query)
	if err != nil {
		return querier.IntradayPointResult{}, err
	}
	table, err := tableName(s.marketDB, "a_share_intraday_points")
	if err != nil {
		return querier.IntradayPointResult{}, err
	}
	where, args, err := intradayPointWhere(normalized)
	if err != nil {
		return querier.IntradayPointResult{}, err
	}
	columns := "market, symbol, trade_date, point_time, point_index, price, volume"
	stmt := fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY point_time ASC LIMIT %d", columns, table, where, normalized.Limit)
	rows, err := s.conn.Query(ctx, stmt, args...)
	if err != nil {
		return querier.IntradayPointResult{}, err
	}
	defer rows.Close()
	result := querier.IntradayPointResult{Query: normalized}
	for rows.Next() {
		var point querier.IntradayPoint
		var tradeDate time.Time
		if err := rows.Scan(&point.Market, &point.Symbol, &tradeDate, &point.PointTime, &point.PointIndex, &point.Price, &point.Volume); err != nil {
			return querier.IntradayPointResult{}, err
		}
		point.TradeDate = tradeDate.Format("2006-01-02")
		result.Points = append(result.Points, point)
	}
	return result, rows.Err()
}

func (s *Store) dailyBars(ctx context.Context, query querier.BarQuery) (querier.BarResult, error) {
	table, err := tableName(s.marketDB, tableForPeriod(query.Period))
	if err != nil {
		return querier.BarResult{}, err
	}
	if query.Adjust != "none" {
		factorTable, err := tableName(s.marketDB, "a_share_adjust_factors_1d")
		if err != nil {
			return querier.BarResult{}, err
		}
		return s.adjustedDailyBars(ctx, query, table, factorTable)
	}
	where, args, err := barWhere(query, "trade_date", parseDateBound, parseDateUntilBound)
	if err != nil {
		return querier.BarResult{}, err
	}
	columns := "market, symbol, trade_date, open, high, low, close, volume, amount"
	stmt := barsSQL(table, columns, where, "trade_date", query.Limit, hasTimeBounds(query))
	rows, err := s.conn.Query(ctx, stmt, args...)
	if err != nil {
		return querier.BarResult{}, err
	}
	defer rows.Close()
	result := querier.BarResult{Query: query}
	for rows.Next() {
		var bar querier.Bar
		var tradeDate time.Time
		if err := rows.Scan(&bar.Market, &bar.Symbol, &tradeDate, &bar.Open, &bar.High, &bar.Low, &bar.Close, &bar.Volume, &bar.Amount); err != nil {
			return querier.BarResult{}, err
		}
		bar.Period = query.Period
		bar.TradeDate = tradeDate.Format("2006-01-02")
		result.Bars = append(result.Bars, bar)
	}
	return result, rows.Err()
}

func (s *Store) adjustedDailyBars(ctx context.Context, query querier.BarQuery, table string, factorTable string) (querier.BarResult, error) {
	where, args, err := barWhere(query, "b.trade_date", parseDateBound, parseDateUntilBound)
	if err != nil {
		return querier.BarResult{}, err
	}
	where = strings.ReplaceAll(where, "market = ?", "b.market = ?")
	where = strings.ReplaceAll(where, "symbol = ?", "b.symbol = ?")
	factorColumn := factorColumnForAdjust(query.Adjust)
	columns := "b.market, b.symbol, b.trade_date, b.open, b.high, b.low, b.close, b.volume, b.amount, f." + factorColumn
	stmt := adjustedBarsSQL(table, factorTable, columns, where, "b.trade_date", query.Limit, hasTimeBounds(query))
	rows, err := s.conn.Query(ctx, stmt, args...)
	if err != nil {
		return querier.BarResult{}, err
	}
	defer rows.Close()
	result := querier.BarResult{Query: query}
	for rows.Next() {
		var bar querier.Bar
		var tradeDate time.Time
		var factor sql.NullFloat64
		if err := rows.Scan(&bar.Market, &bar.Symbol, &tradeDate, &bar.Open, &bar.High, &bar.Low, &bar.Close, &bar.Volume, &bar.Amount, &factor); err != nil {
			return querier.BarResult{}, err
		}
		if !factor.Valid {
			return querier.BarResult{}, missingFactor(query, tradeDate)
		}
		bar.Period = query.Period
		bar.TradeDate = tradeDate.Format("2006-01-02")
		applyFactor(&bar, factor.Float64)
		result.Bars = append(result.Bars, bar)
	}
	return result, rows.Err()
}

func (s *Store) minuteBars(ctx context.Context, query querier.BarQuery) (querier.BarResult, error) {
	table, err := tableName(s.marketDB, tableForPeriod(query.Period))
	if err != nil {
		return querier.BarResult{}, err
	}
	if query.Adjust != "none" {
		factorTable, err := tableName(s.marketDB, "a_share_adjust_factors_1d")
		if err != nil {
			return querier.BarResult{}, err
		}
		return s.adjustedMinuteBars(ctx, query, table, factorTable)
	}
	where, args, err := barWhere(query, "bar_time", parseDateTimeBound, parseDateTimeUntilBound)
	if err != nil {
		return querier.BarResult{}, err
	}
	columns := "market, symbol, bar_time, trade_date, open, high, low, close, volume, amount"
	stmt := barsSQL(table, columns, where, "bar_time", query.Limit, hasTimeBounds(query))
	rows, err := s.conn.Query(ctx, stmt, args...)
	if err != nil {
		return querier.BarResult{}, err
	}
	defer rows.Close()
	result := querier.BarResult{Query: query}
	for rows.Next() {
		var bar querier.Bar
		var tradeDate time.Time
		var barTime time.Time
		if err := rows.Scan(&bar.Market, &bar.Symbol, &barTime, &tradeDate, &bar.Open, &bar.High, &bar.Low, &bar.Close, &bar.Volume, &bar.Amount); err != nil {
			return querier.BarResult{}, err
		}
		bar.Period = query.Period
		bar.TradeDate = tradeDate.Format("2006-01-02")
		bar.BarTime = &barTime
		result.Bars = append(result.Bars, bar)
	}
	return result, rows.Err()
}

func (s *Store) adjustedMinuteBars(ctx context.Context, query querier.BarQuery, table string, factorTable string) (querier.BarResult, error) {
	where, args, err := barWhere(query, "b.bar_time", parseDateTimeBound, parseDateTimeUntilBound)
	if err != nil {
		return querier.BarResult{}, err
	}
	where = strings.ReplaceAll(where, "market = ?", "b.market = ?")
	where = strings.ReplaceAll(where, "symbol = ?", "b.symbol = ?")
	factorColumn := factorColumnForAdjust(query.Adjust)
	columns := "b.market, b.symbol, b.bar_time, b.trade_date, b.open, b.high, b.low, b.close, b.volume, b.amount, f." + factorColumn
	stmt := adjustedBarsSQL(table, factorTable, columns, where, "b.bar_time", query.Limit, hasTimeBounds(query))
	rows, err := s.conn.Query(ctx, stmt, args...)
	if err != nil {
		return querier.BarResult{}, err
	}
	defer rows.Close()
	result := querier.BarResult{Query: query}
	for rows.Next() {
		var bar querier.Bar
		var tradeDate time.Time
		var barTime time.Time
		var factor sql.NullFloat64
		if err := rows.Scan(&bar.Market, &bar.Symbol, &barTime, &tradeDate, &bar.Open, &bar.High, &bar.Low, &bar.Close, &bar.Volume, &bar.Amount, &factor); err != nil {
			return querier.BarResult{}, err
		}
		if !factor.Valid {
			return querier.BarResult{}, missingFactor(query, tradeDate)
		}
		bar.Period = query.Period
		bar.TradeDate = tradeDate.Format("2006-01-02")
		bar.BarTime = &barTime
		applyFactor(&bar, factor.Float64)
		result.Bars = append(result.Bars, bar)
	}
	return result, rows.Err()
}

func barWhere(query querier.BarQuery, timeColumn string, parseSince func(string) (time.Time, error), parseUntil func(string) (time.Time, string, error)) (string, []any, error) {
	clauses := []string{"market = ?", "symbol = ?"}
	args := []any{query.Market, query.Symbol}
	if query.Since != "" {
		t, err := parseSince(query.Since)
		if err != nil {
			return "", nil, err
		}
		clauses = append(clauses, timeColumn+" >= ?")
		args = append(args, t)
	}
	if query.Until != "" {
		t, operator, err := parseUntil(query.Until)
		if err != nil {
			return "", nil, err
		}
		clauses = append(clauses, timeColumn+" "+operator+" ?")
		args = append(args, t)
	}
	return strings.Join(clauses, " AND "), args, nil
}

func intradayPointWhere(query querier.IntradayPointQuery) (string, []any, error) {
	clauses := []string{"market = ?", "symbol = ?"}
	args := []any{query.Market, query.Symbol}
	if query.Date != "" {
		t, err := parseDateBound(query.Date)
		if err != nil {
			return "", nil, err
		}
		clauses = append(clauses, "trade_date = ?")
		args = append(args, t)
		return strings.Join(clauses, " AND "), args, nil
	}
	if query.Since != "" {
		t, err := parseDateTimeBound(query.Since)
		if err != nil {
			return "", nil, err
		}
		clauses = append(clauses, "point_time >= ?")
		args = append(args, t)
	}
	if query.Until != "" {
		t, operator, err := parseDateTimeUntilBound(query.Until)
		if err != nil {
			return "", nil, err
		}
		clauses = append(clauses, "point_time "+operator+" ?")
		args = append(args, t)
	}
	return strings.Join(clauses, " AND "), args, nil
}

func adjustedBarsSQL(table string, factorTable string, columns string, where string, timeColumn string, limit int, hasBounds bool) string {
	joinCondition := "b.market = f.market AND b.symbol = f.symbol AND b.trade_date = f.trade_date"
	if hasBounds {
		return fmt.Sprintf("SELECT %s FROM %s AS b LEFT JOIN %s AS f ON %s WHERE %s ORDER BY %s ASC LIMIT %d", columns, table, factorTable, joinCondition, where, timeColumn, limit)
	}
	innerWhere := strings.ReplaceAll(where, "b.", "")
	innerTimeColumn := strings.ReplaceAll(timeColumn, "b.", "")
	return fmt.Sprintf("SELECT %s FROM (SELECT * FROM %s WHERE %s ORDER BY %s DESC LIMIT %d) AS b LEFT JOIN %s AS f ON %s ORDER BY %s ASC", columns, table, innerWhere, innerTimeColumn, limit, factorTable, joinCondition, timeColumn)
}

func barsSQL(table string, columns string, where string, timeColumn string, limit int, hasBounds bool) string {
	if hasBounds {
		return fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY %s ASC LIMIT %d", columns, table, where, timeColumn, limit)
	}
	return fmt.Sprintf("SELECT %s FROM (SELECT %s FROM %s WHERE %s ORDER BY %s DESC LIMIT %d) ORDER BY %s ASC", columns, columns, table, where, timeColumn, limit, timeColumn)
}

func dailyPctChgScanSQL(derivedTable string) string {
	return fmt.Sprintf("SELECT market, symbol, trade_date, prev_close, pct_chg FROM %s FINAL WHERE trade_date = ? AND pct_chg > ? ORDER BY pct_chg DESC", derivedTable)
}

func factorColumnForAdjust(adjust string) string {
	if adjust == "hfq" {
		return "hfq_factor"
	}
	return "qfq_factor"
}

func applyFactor(bar *querier.Bar, factor float64) {
	bar.Open *= factor
	bar.High *= factor
	bar.Low *= factor
	bar.Close *= factor
}

func missingFactor(query querier.BarQuery, tradeDate time.Time) error {
	return querier.MissingAdjustmentFactorError{Message: fmt.Sprintf("missing %s adjustment factor for %s:%s on %s", query.Adjust, query.Market, query.Symbol, tradeDate.Format("2006-01-02"))}
}

func hasTimeBounds(query querier.BarQuery) bool {
	return query.Since != "" || query.Until != ""
}

func tableForPeriod(period string) string {
	switch period {
	case "1d":
		return "a_share_bars_1d"
	case "1m":
		return "a_share_bars_1m"
	case "5m":
		return "a_share_bars_5m"
	default:
		return ""
	}
}

func parseDateBound(value string) (time.Time, error) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	t, err := time.ParseInLocation("2006-01-02", value, loc)
	if err == nil {
		return t, nil
	}
	return time.Time{}, querier.ValidationError{Message: fmt.Sprintf("invalid date %q, expected YYYY-MM-DD", value)}
}

func parseDateUntilBound(value string) (time.Time, string, error) {
	t, err := parseDateBound(value)
	if err != nil {
		return time.Time{}, "", err
	}
	return t, "<=", nil
}

func parseDateTimeBound(value string) (time.Time, error) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05", "2006-01-02T15:04:05", time.RFC3339} {
		t, err := time.ParseInLocation(layout, value, loc)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, querier.ValidationError{Message: fmt.Sprintf("invalid datetime %q", value)}
}

func parseDateTimeUntilBound(value string) (time.Time, string, error) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if t, err := time.ParseInLocation("2006-01-02", value, loc); err == nil {
		return t.AddDate(0, 0, 1), "<", nil
	}
	t, err := parseDateTimeBound(value)
	if err != nil {
		return time.Time{}, "", err
	}
	return t, "<=", nil
}
