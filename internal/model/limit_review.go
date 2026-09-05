package model

import "time"

type LimitEvent struct {
	TradeDate        time.Time `json:"trade_date" ch:"trade_date"`
	Market           string    `json:"market" ch:"market"`
	Symbol           string    `json:"symbol" ch:"symbol"`
	EventType        string    `json:"event_type" ch:"event_type"`
	CloseStatus      string    `json:"close_status" ch:"close_status"`
	BoardCount       uint16    `json:"board_count" ch:"board_count"`
	ReasonText       string    `json:"reason_text" ch:"reason_text"`
	ThemePrimary     string    `json:"theme_primary" ch:"theme_primary"`
	ThemeTags        []string  `json:"theme_tags" ch:"theme_tags"`
	FirstLimitMinute *string   `json:"first_limit_minute" ch:"first_limit_minute"`
	LastLimitMinute  *string   `json:"last_limit_minute" ch:"last_limit_minute"`
	OpenCount        *uint16   `json:"open_count" ch:"open_count"`
	SealOrderAmount  *float64  `json:"seal_order_amount" ch:"seal_order_amount"`
	Amount           *float64  `json:"amount" ch:"amount"`
	TurnoverRate     *float64  `json:"turnover_rate" ch:"turnover_rate"`
	MarketValue      *float64  `json:"market_value" ch:"market_value"`
}

type LimitDailySummary struct {
	TradeDate                time.Time  `json:"trade_date" ch:"trade_date"`
	PrevTradeDate            *time.Time `json:"prev_trade_date" ch:"prev_trade_date"`
	LimitUpCount             uint32     `json:"limit_up_count" ch:"limit_up_count"`
	LimitDownCount           uint32     `json:"limit_down_count" ch:"limit_down_count"`
	OpenLimitCount           uint32     `json:"open_limit_count" ch:"open_limit_count"`
	SealSuccessRate          *float64   `json:"seal_success_rate" ch:"seal_success_rate"`
	MaxBoardHeight           uint16     `json:"max_board_height" ch:"max_board_height"`
	FirstBoardCount          uint32     `json:"first_board_count" ch:"first_board_count"`
	ContinuousBoardCount     uint32     `json:"continuous_board_count" ch:"continuous_board_count"`
	PrevLimitUpPromotionRate *float64   `json:"prev_limit_up_promotion_rate" ch:"prev_limit_up_promotion_rate"`
	PrevLadderPromotionRate  *float64   `json:"prev_ladder_promotion_rate" ch:"prev_ladder_promotion_rate"`
	BigNoodleCount           *uint32    `json:"big_noodle_count" ch:"big_noodle_count"`
	HighLevelBreakCount      *uint32    `json:"high_level_break_count" ch:"high_level_break_count"`
	StrongThemeCount         *uint32    `json:"strong_theme_count" ch:"strong_theme_count"`
	TwoBoardCount            uint32     `json:"two_board_count" ch:"two_board_count"`
	ThreeBoardCount          uint32     `json:"three_board_count" ch:"three_board_count"`
	FourBoardCount           uint32     `json:"four_board_count" ch:"four_board_count"`
	FivePlusBoardCount       uint32     `json:"five_plus_board_count" ch:"five_plus_board_count"`
}

type LimitRelayEvent struct {
	TradeDate            time.Time `json:"trade_date" ch:"trade_date"`
	PrevTradeDate        time.Time `json:"prev_trade_date" ch:"prev_trade_date"`
	Market               string    `json:"market" ch:"market"`
	Symbol               string    `json:"symbol" ch:"symbol"`
	SampleGroup          string    `json:"sample_group" ch:"sample_group"`
	PrevBoardCount       uint16    `json:"prev_board_count" ch:"prev_board_count"`
	PrevReasonText       string    `json:"prev_reason_text" ch:"prev_reason_text"`
	PrevThemePrimary     string    `json:"prev_theme_primary" ch:"prev_theme_primary"`
	PrevFirstLimitMinute *string   `json:"prev_first_limit_minute" ch:"prev_first_limit_minute"`
	TodayStatus          string    `json:"today_status" ch:"today_status"`
	TodayBoardCount      uint16    `json:"today_board_count" ch:"today_board_count"`
	TodayPctChg          *float64  `json:"today_pct_chg" ch:"today_pct_chg"`
}

type LimitPerformanceIndexBar struct {
	IndexCode string    `json:"index_code" ch:"index_code"`
	TradeDate time.Time `json:"trade_date" ch:"trade_date"`
	Open      float64   `json:"open" ch:"open"`
	High      float64   `json:"high" ch:"high"`
	Low       float64   `json:"low" ch:"low"`
	Close     float64   `json:"close" ch:"close"`
	Volume    *uint64   `json:"volume" ch:"volume"`
	Amount    *float64  `json:"amount" ch:"amount"`
}

type MarketBreadthDaily struct {
	TradeDate                 time.Time `json:"trade_date" ch:"trade_date"`
	UpCount                   uint32    `json:"up_count" ch:"up_count"`
	DownCount                 uint32    `json:"down_count" ch:"down_count"`
	FlatCount                 *uint32   `json:"flat_count" ch:"flat_count"`
	UnchangedOrSuspendedCount *uint32   `json:"unchanged_or_suspended_count" ch:"unchanged_or_suspended_count"`
	UpGT3Count                *uint32   `json:"up_gt_3_count" ch:"up_gt_3_count"`
	UpGT5Count                *uint32   `json:"up_gt_5_count" ch:"up_gt_5_count"`
	UpGT7Count                *uint32   `json:"up_gt_7_count" ch:"up_gt_7_count"`
	DownGT3Count              *uint32   `json:"down_gt_3_count" ch:"down_gt_3_count"`
	DownGT5Count              *uint32   `json:"down_gt_5_count" ch:"down_gt_5_count"`
	DownGT7Count              *uint32   `json:"down_gt_7_count" ch:"down_gt_7_count"`
	LimitUpCount              *uint32   `json:"limit_up_count" ch:"limit_up_count"`
	LimitDownCount            *uint32   `json:"limit_down_count" ch:"limit_down_count"`
	TotalCount                uint32    `json:"total_count" ch:"total_count"`
}

type LimitThemeDaily struct {
	TradeDate        time.Time `json:"trade_date" ch:"trade_date"`
	ThemeName        string    `json:"theme_name" ch:"theme_name"`
	LimitUpCount     uint32    `json:"limit_up_count" ch:"limit_up_count"`
	LadderCount      uint32    `json:"ladder_count" ch:"ladder_count"`
	BrokenCount      uint32    `json:"broken_count" ch:"broken_count"`
	LimitDownCount   uint32    `json:"limit_down_count" ch:"limit_down_count"`
	LeaderMarket     string    `json:"leader_market" ch:"leader_market"`
	LeaderSymbol     string    `json:"leader_symbol" ch:"leader_symbol"`
	LeaderBoardCount uint16    `json:"leader_board_count" ch:"leader_board_count"`
	StrengthRank     uint16    `json:"strength_rank" ch:"strength_rank"`
}
