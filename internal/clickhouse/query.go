package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/querier"
)

func (s *Store) Health(ctx context.Context) error {
	return s.conn.Ping(ctx)
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

func (s *Store) dailyBars(ctx context.Context, query querier.BarQuery) (querier.BarResult, error) {
	table, err := tableName(s.marketDB, tableForPeriod(query.Period))
	if err != nil {
		return querier.BarResult{}, err
	}
	where, args, err := barWhere(query, "trade_date", parseDateBound)
	if err != nil {
		return querier.BarResult{}, err
	}
	stmt := fmt.Sprintf("SELECT market, symbol, trade_date, open, high, low, close, volume, amount FROM %s WHERE %s ORDER BY trade_date ASC LIMIT %d", table, where, query.Limit)
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

func (s *Store) minuteBars(ctx context.Context, query querier.BarQuery) (querier.BarResult, error) {
	table, err := tableName(s.marketDB, tableForPeriod(query.Period))
	if err != nil {
		return querier.BarResult{}, err
	}
	where, args, err := barWhere(query, "bar_time", parseDateTimeBound)
	if err != nil {
		return querier.BarResult{}, err
	}
	stmt := fmt.Sprintf("SELECT market, symbol, bar_time, trade_date, open, high, low, close, volume, amount FROM %s WHERE %s ORDER BY bar_time ASC LIMIT %d", table, where, query.Limit)
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

func barWhere(query querier.BarQuery, timeColumn string, parseBound func(string) (time.Time, error)) (string, []any, error) {
	clauses := []string{"market = ?", "symbol = ?"}
	args := []any{query.Market, query.Symbol}
	if query.Since != "" {
		t, err := parseBound(query.Since)
		if err != nil {
			return "", nil, err
		}
		clauses = append(clauses, timeColumn+" >= ?")
		args = append(args, t)
	}
	if query.Until != "" {
		t, err := parseBound(query.Until)
		if err != nil {
			return "", nil, err
		}
		clauses = append(clauses, timeColumn+" < ?")
		args = append(args, t)
	}
	return strings.Join(clauses, " AND "), args, nil
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
