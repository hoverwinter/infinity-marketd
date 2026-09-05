package clickhouse

import (
	"context"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

func (s *Store) InsertLimitEvents(ctx context.Context, rows []model.LimitEvent) error {
	return insertReviewRows(ctx, rows, func(row model.LimitEvent) time.Time { return row.TradeDate }, s.insertLimitEventsBatch)
}

func (s *Store) insertLimitEventsBatch(ctx context.Context, rows []model.LimitEvent) error {
	table, err := tableName(s.marketDB, "a_share_limit_events")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (trade_date, market, symbol, event_type, close_status, board_count, reason_text, theme_primary, theme_tags, first_limit_minute, last_limit_minute, open_count, seal_order_amount, amount, turnover_rate, market_value) VALUES")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := batch.Append(row.TradeDate, row.Market, row.Symbol, row.EventType, row.CloseStatus, row.BoardCount, row.ReasonText, row.ThemePrimary, row.ThemeTags, row.FirstLimitMinute, row.LastLimitMinute, row.OpenCount, row.SealOrderAmount, row.Amount, row.TurnoverRate, row.MarketValue); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertLimitDailySummaries(ctx context.Context, rows []model.LimitDailySummary) error {
	return insertReviewRows(ctx, rows, func(row model.LimitDailySummary) time.Time { return row.TradeDate }, s.insertLimitDailySummariesBatch)
}

func (s *Store) insertLimitDailySummariesBatch(ctx context.Context, rows []model.LimitDailySummary) error {
	table, err := tableName(s.marketDB, "a_share_limit_daily_summary")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (trade_date, prev_trade_date, limit_up_count, limit_down_count, open_limit_count, seal_success_rate, max_board_height, first_board_count, continuous_board_count, prev_limit_up_promotion_rate, prev_ladder_promotion_rate, big_noodle_count, high_level_break_count, strong_theme_count, two_board_count, three_board_count, four_board_count, five_plus_board_count) VALUES")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := batch.Append(row.TradeDate, row.PrevTradeDate, row.LimitUpCount, row.LimitDownCount, row.OpenLimitCount, row.SealSuccessRate, row.MaxBoardHeight, row.FirstBoardCount, row.ContinuousBoardCount, row.PrevLimitUpPromotionRate, row.PrevLadderPromotionRate, row.BigNoodleCount, row.HighLevelBreakCount, row.StrongThemeCount, row.TwoBoardCount, row.ThreeBoardCount, row.FourBoardCount, row.FivePlusBoardCount); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertLimitRelayEvents(ctx context.Context, rows []model.LimitRelayEvent) error {
	return insertReviewRows(ctx, rows, func(row model.LimitRelayEvent) time.Time { return row.TradeDate }, s.insertLimitRelayEventsBatch)
}

func (s *Store) insertLimitRelayEventsBatch(ctx context.Context, rows []model.LimitRelayEvent) error {
	table, err := tableName(s.marketDB, "a_share_limit_relay_events")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (trade_date, prev_trade_date, market, symbol, sample_group, prev_board_count, prev_reason_text, prev_theme_primary, prev_first_limit_minute, today_status, today_board_count, today_pct_chg) VALUES")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := batch.Append(row.TradeDate, row.PrevTradeDate, row.Market, row.Symbol, row.SampleGroup, row.PrevBoardCount, row.PrevReasonText, row.PrevThemePrimary, row.PrevFirstLimitMinute, row.TodayStatus, row.TodayBoardCount, row.TodayPctChg); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertLimitPerformanceIndexBars(ctx context.Context, rows []model.LimitPerformanceIndexBar) error {
	return insertReviewRows(ctx, rows, func(row model.LimitPerformanceIndexBar) time.Time { return row.TradeDate }, s.insertLimitPerformanceIndexBarsBatch)
}

func (s *Store) insertLimitPerformanceIndexBarsBatch(ctx context.Context, rows []model.LimitPerformanceIndexBar) error {
	table, err := tableName(s.marketDB, "a_share_limit_performance_index_bars_1d")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (index_code, trade_date, open, high, low, close, volume, amount) VALUES")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := batch.Append(row.IndexCode, row.TradeDate, row.Open, row.High, row.Low, row.Close, row.Volume, row.Amount); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertMarketBreadthDaily(ctx context.Context, rows []model.MarketBreadthDaily) error {
	return insertReviewRows(ctx, rows, func(row model.MarketBreadthDaily) time.Time { return row.TradeDate }, s.insertMarketBreadthDailyBatch)
}

func (s *Store) insertMarketBreadthDailyBatch(ctx context.Context, rows []model.MarketBreadthDaily) error {
	table, err := tableName(s.marketDB, "a_share_market_breadth_daily")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (trade_date, up_count, down_count, flat_count, unchanged_or_suspended_count, up_gt_3_count, up_gt_5_count, up_gt_7_count, down_gt_3_count, down_gt_5_count, down_gt_7_count, limit_up_count, limit_down_count, total_count) VALUES")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := batch.Append(row.TradeDate, row.UpCount, row.DownCount, row.FlatCount, row.UnchangedOrSuspendedCount, row.UpGT3Count, row.UpGT5Count, row.UpGT7Count, row.DownGT3Count, row.DownGT5Count, row.DownGT7Count, row.LimitUpCount, row.LimitDownCount, row.TotalCount); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) InsertLimitThemeDaily(ctx context.Context, rows []model.LimitThemeDaily) error {
	return insertReviewRows(ctx, rows, func(row model.LimitThemeDaily) time.Time { return row.TradeDate }, s.insertLimitThemeDailyBatch)
}

func (s *Store) insertLimitThemeDailyBatch(ctx context.Context, rows []model.LimitThemeDaily) error {
	table, err := tableName(s.marketDB, "a_share_limit_theme_daily")
	if err != nil {
		return err
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" (trade_date, theme_name, limit_up_count, ladder_count, broken_count, limit_down_count, leader_market, leader_symbol, leader_board_count, strength_rank) VALUES")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := batch.Append(row.TradeDate, row.ThemeName, row.LimitUpCount, row.LadderCount, row.BrokenCount, row.LimitDownCount, row.LeaderMarket, row.LeaderSymbol, row.LeaderBoardCount, row.StrengthRank); err != nil {
			return err
		}
	}
	return batch.Send()
}

func insertReviewRows[T any](ctx context.Context, rows []T, dateOf func(T) time.Time, insert func(context.Context, []T) error) error {
	for start := 0; start < len(rows); {
		partitions := map[string]struct{}{dailyPartitionKey(dateOf(rows[start])): {}}
		end := start + 1
		for end < len(rows) {
			partition := dailyPartitionKey(dateOf(rows[end]))
			if _, exists := partitions[partition]; !exists && len(partitions) >= maxPartitionsPerInsertBlock {
				break
			}
			partitions[partition] = struct{}{}
			end++
		}
		if err := insert(ctx, rows[start:end]); err != nil {
			return err
		}
		start = end
	}
	return nil
}
