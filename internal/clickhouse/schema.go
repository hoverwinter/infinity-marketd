package clickhouse

import (
	"fmt"
	"regexp"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type SchemaConfig struct {
	MarketDB string
	OpsDB    string
}

func BootstrapDDL(cfg SchemaConfig) ([]string, error) {
	marketDB, err := quoteIdent(cfg.MarketDB)
	if err != nil {
		return nil, err
	}
	opsDB, err := quoteIdent(cfg.OpsDB)
	if err != nil {
		return nil, err
	}
	return []string{
		fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", marketDB),
		fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", opsDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_bars_1d
(
    market LowCardinality(String),
    symbol String,
    trade_date Date,
    open Float64,
    high Float64,
    low Float64,
    close Float64,
    volume UInt64,
    amount Float64
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(trade_date)
ORDER BY (market, symbol, trade_date)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_bars_1m
(
    market LowCardinality(String),
    symbol String,
    bar_time DateTime('Asia/Shanghai'),
    trade_date Date,
    open Float64,
    high Float64,
    low Float64,
    close Float64,
    volume UInt64,
    amount Float64
)
ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(trade_date)
ORDER BY (market, symbol, bar_time)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_bars_5m
(
    market LowCardinality(String),
    symbol String,
    bar_time DateTime('Asia/Shanghai'),
    trade_date Date,
    open Float64,
    high Float64,
    low Float64,
    close Float64,
    volume UInt64,
    amount Float64
)
ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(trade_date)
ORDER BY (market, symbol, bar_time)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_bars_1m_scan
(
    trade_date Date,
    bar_time DateTime('Asia/Shanghai'),
    market LowCardinality(String),
    symbol String,
    close Float64,
    volume UInt64,
    amount Float64,
    prev_close Nullable(Float64),
    minute_ret Nullable(Float64),
    volume_ratio Nullable(Float64),
    computed_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(computed_at)
PARTITION BY toYYYYMM(trade_date)
ORDER BY (trade_date, bar_time, market, symbol)
TTL trade_date + INTERVAL 12 MONTH DELETE`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_bars_5m_scan
(
    trade_date Date,
    bar_time DateTime('Asia/Shanghai'),
    market LowCardinality(String),
    symbol String,
    close Float64,
    volume UInt64,
    amount Float64,
    prev_close Nullable(Float64),
    minute_ret Nullable(Float64),
    volume_ratio Nullable(Float64),
    computed_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(computed_at)
PARTITION BY toYYYYMM(trade_date)
ORDER BY (trade_date, bar_time, market, symbol)
TTL trade_date + INTERVAL 12 MONTH DELETE`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_financial_raw_items
(
    market LowCardinality(String),
    symbol String,
    report_date Date,
    item_id UInt16,
    value Float64
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(report_date)
ORDER BY (market, symbol, report_date, item_id)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_gp_metric_values
(
    market LowCardinality(String),
    symbol String,
    metric_type UInt16,
    event_date Date,
    value1 Float64,
    value2 Float64
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(event_date)
ORDER BY (market, symbol, metric_type, event_date)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.tdx_financial_item_dictionary
(
    item_id UInt16,
    name String,
    title String,
    category LowCardinality(String),
    unit LowCardinality(String),
    value_kind LowCardinality(String),
    source_ref String,
    status LowCardinality(String)
)
ENGINE = ReplacingMergeTree
ORDER BY (item_id)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.tdx_gp_metric_dictionary
(
    metric_type UInt16,
    name String,
    title String,
    value1_meaning String,
    value2_meaning String,
    source_ref String,
    status LowCardinality(String)
)
ENGINE = ReplacingMergeTree
ORDER BY (metric_type)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_intraday_points
(
    market LowCardinality(String),
    symbol String,
    trade_date Date,
    point_time DateTime('Asia/Shanghai'),
    point_index UInt16,
    price Float64,
    volume UInt64
)
ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(trade_date)
ORDER BY (market, symbol, trade_date, point_time)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_daily_derived
(
    market LowCardinality(String),
    symbol String,
    trade_date Date,
    prev_close Nullable(Float64),
    pct_chg Nullable(Float64),
    computed_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(computed_at)
PARTITION BY toYear(trade_date)
ORDER BY (trade_date, market, symbol)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_limit_events
(
    trade_date Date,
    market LowCardinality(String),
    symbol String,
    event_type LowCardinality(String),
    close_status LowCardinality(String),
    board_count UInt16,
    reason_text String,
    theme_primary LowCardinality(String),
    theme_tags Array(String),
    first_limit_minute Nullable(String),
    last_limit_minute Nullable(String),
    open_count Nullable(UInt16),
    seal_order_amount Nullable(Float64),
    amount Nullable(Float64),
    turnover_rate Nullable(Float64),
    market_value Nullable(Float64)
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(trade_date)
ORDER BY (trade_date, event_type, market, symbol)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_limit_daily_summary
(
    trade_date Date,
    prev_trade_date Nullable(Date),
    limit_up_count UInt32,
    limit_down_count UInt32,
    open_limit_count UInt32,
    seal_success_rate Nullable(Float64),
    max_board_height UInt16,
    first_board_count UInt32,
    continuous_board_count UInt32,
    prev_limit_up_promotion_rate Nullable(Float64),
    prev_ladder_promotion_rate Nullable(Float64),
    big_noodle_count Nullable(UInt32),
    high_level_break_count Nullable(UInt32),
    strong_theme_count Nullable(UInt32),
    two_board_count UInt32,
    three_board_count UInt32,
    four_board_count UInt32,
    five_plus_board_count UInt32
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(trade_date)
ORDER BY (trade_date)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_limit_relay_events
(
    trade_date Date,
    prev_trade_date Date,
    market LowCardinality(String),
    symbol String,
    sample_group LowCardinality(String),
    prev_board_count UInt16,
    prev_reason_text String,
    prev_theme_primary LowCardinality(String),
    prev_first_limit_minute Nullable(String),
    today_status LowCardinality(String),
    today_board_count UInt16,
    today_pct_chg Nullable(Float64)
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(trade_date)
ORDER BY (trade_date, sample_group, market, symbol)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_limit_performance_index_bars_1d
(
    index_code LowCardinality(String),
    trade_date Date,
    open Float64,
    high Float64,
    low Float64,
    close Float64,
    volume Nullable(UInt64),
    amount Nullable(Float64)
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(trade_date)
ORDER BY (index_code, trade_date)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_market_breadth_daily
(
    trade_date Date,
    up_count UInt32,
    down_count UInt32,
    flat_count Nullable(UInt32),
    unchanged_or_suspended_count Nullable(UInt32),
    up_gt_3_count Nullable(UInt32),
    up_gt_5_count Nullable(UInt32),
    up_gt_7_count Nullable(UInt32),
    down_gt_3_count Nullable(UInt32),
    down_gt_5_count Nullable(UInt32),
    down_gt_7_count Nullable(UInt32),
    limit_up_count Nullable(UInt32),
    limit_down_count Nullable(UInt32),
    total_count UInt32
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(trade_date)
ORDER BY (trade_date)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_limit_theme_daily
(
    trade_date Date,
    theme_name LowCardinality(String),
    limit_up_count UInt32,
    ladder_count UInt32,
    broken_count UInt32,
    limit_down_count UInt32,
    leader_market LowCardinality(String),
    leader_symbol String,
    leader_board_count UInt16,
    strength_rank UInt16
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(trade_date)
ORDER BY (trade_date, theme_name)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_xdxr_events
	(
	    market LowCardinality(String),
	    symbol String,
	    event_date Date,
	    category UInt8,
	    category_name String,
	    fenhong Nullable(Float64),
	    peigujia Nullable(Float64),
	    songzhuangu Nullable(Float64),
	    peigu Nullable(Float64),
	    suogu Nullable(Float64),
	    panqianliutong Nullable(Float64),
	    panhouliutong Nullable(Float64),
	    qianzongguben Nullable(Float64),
	    houzongguben Nullable(Float64),
	    fenshu Nullable(Float64),
	    xingquanjia Nullable(Float64)
	)
	ENGINE = ReplacingMergeTree
	PARTITION BY toYear(event_date)
	ORDER BY (market, symbol, event_date, category)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_adjust_factors_1d
	(
	    market LowCardinality(String),
	    symbol String,
	    trade_date Date,
	    qfq_factor Nullable(Float64),
	    hfq_factor Nullable(Float64),
	    computed_at DateTime64(3) DEFAULT now64(3)
	)
	ENGINE = ReplacingMergeTree(computed_at)
	PARTITION BY toYear(trade_date)
	ORDER BY (market, symbol, trade_date)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.a_share_capital_change_events
(
    market LowCardinality(String),
    symbol String,
    event_date Date,
    category UInt8,
    event_seq UInt16,
    event_name LowCardinality(String),
    cash_dividend Nullable(Float64),
    allotment_price Nullable(Float64),
    bonus_shares Nullable(Float64),
    allotment_shares Nullable(Float64),
    shrink_shares Nullable(Float64),
    pre_float_shares Nullable(Float64),
    post_float_shares Nullable(Float64),
    pre_total_shares Nullable(Float64),
    post_total_shares Nullable(Float64),
    ratio_denominator Nullable(Float64),
    exercise_price Nullable(Float64)
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(event_date)
ORDER BY (market, symbol, event_date, category, event_seq)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.tdx_block_snapshots
(
    snapshot_id String,
    block_scope LowCardinality(String),
    snapshot_time DateTime64(3, 'Asia/Shanghai'),
    content_hash String,
    block_count UInt32,
    member_count UInt32
)
ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(snapshot_time)
ORDER BY (block_scope, snapshot_time, snapshot_id)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.tdx_block_definitions
(
    snapshot_id String,
    block_scope LowCardinality(String),
    block_kind LowCardinality(String),
    block_id String,
    block_name String,
    block_type UInt16,
    display_order UInt32,
    member_count UInt32
)
ENGINE = ReplacingMergeTree
ORDER BY (snapshot_id, block_scope, block_id)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.tdx_block_memberships
(
    snapshot_id String,
    block_scope LowCardinality(String),
    block_id String,
    member_order UInt32,
    code String,
    market LowCardinality(String),
    symbol String
)
ENGINE = ReplacingMergeTree
ORDER BY (snapshot_id, block_scope, block_id, market, symbol, member_order)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.tdx_ex_bars_1d
(
    ex_market UInt16,
    code String,
    trade_date Date,
    open Float64,
    high Float64,
    low Float64,
    close Float64,
    position Int64,
    trade Int64,
    price Nullable(Float64),
    amount Nullable(Float64),
    settlement_price Nullable(Float64)
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(trade_date)
ORDER BY (ex_market, code, trade_date)`, marketDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.watermarks
	(
    dataset LowCardinality(String),
    asset LowCardinality(String),
    status LowCardinality(String),
    min_watermark Nullable(DateTime64(3)),
    max_watermark Nullable(DateTime64(3)),
    rows_written UInt64,
    message String,
    updated_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (dataset, asset)`, opsDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.task_runs
(
    run_id String,
    dataset LowCardinality(String),
    task_type LowCardinality(String),
    status LowCardinality(String),
    target_table LowCardinality(String),
    input_path String,
    input_format LowCardinality(String),
    params String,
    started_at DateTime64(3),
    finished_at Nullable(DateTime64(3)),
    duration_ms Nullable(UInt64),
    rows_written UInt64,
    rows_skipped UInt64,
    error String,
    updated_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(started_at)
ORDER BY (dataset, started_at, run_id)`, opsDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.quote_service_runs
(
    run_id String,
    status LowCardinality(String),
    markets Array(LowCardinality(String)),
    symbol_source LowCardinality(String),
    batch_size UInt32,
    planned_symbols UInt32,
    planned_batches UInt32,
    succeeded_batches UInt32,
    failed_batches UInt32,
    skipped_batches UInt32,
    rows_fetched UInt64,
    started_at DateTime64(3),
    finished_at Nullable(DateTime64(3)),
    duration_ms Nullable(UInt64),
    error String,
    updated_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(started_at)
ORDER BY (run_id)`, opsDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.quote_service_batches
(
    run_id String,
    batch_no UInt32,
    status LowCardinality(String),
    symbol_count UInt32,
    first_symbol String,
    last_symbol String,
    attempts UInt32,
    rows_fetched UInt64,
    started_at Nullable(DateTime64(3)),
    finished_at Nullable(DateTime64(3)),
    duration_ms Nullable(UInt64),
    failure_kind LowCardinality(String),
    error String,
    updated_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (run_id, batch_no)`, opsDB),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.data_quality_issues
(
    issue_id String,
    run_id String,
    dataset LowCardinality(String),
    severity LowCardinality(String),
    issue_type LowCardinality(String),
    market Nullable(String),
    symbol Nullable(String),
    logical_key String,
    input_path String,
    input_record_offset Nullable(UInt64),
    observed_at DateTime64(3),
    message String,
    details String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(observed_at)
ORDER BY (dataset, observed_at, severity, issue_type)`, opsDB),
	}, nil
}

func quoteIdent(value string) (string, error) {
	if !identifierPattern.MatchString(value) {
		return "", fmt.Errorf("invalid ClickHouse identifier %q", value)
	}
	return "`" + value + "`", nil
}

func tableName(database string, table string) (string, error) {
	db, err := quoteIdent(database)
	if err != nil {
		return "", err
	}
	tbl, err := quoteIdent(table)
	if err != nil {
		return "", err
	}
	return db + "." + tbl, nil
}
