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
