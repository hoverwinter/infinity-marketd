## ADDED Requirements

### Requirement: Marketd service boundary
The system SHALL provide a Go `marketd` data-plane component that owns local structured market data ingestion and ClickHouse writes.

#### Scenario: Independent data plane
- **WHEN** `marketd` imports market data
- **THEN** it MUST NOT depend on Infinity gateway, frontend, Python dagent, pandas, mootdx, or Python parsing code
- **AND** Infinity may consume the resulting ClickHouse tables through SQL or later API integration

#### Scenario: Gateway does not parse TDX files
- **WHEN** an HTTP request reaches Infinity gateway
- **THEN** the gateway MUST NOT parse local TDX files or perform bulk market data imports inside the request path

### Requirement: ClickHouse database bootstrap
The system SHALL bootstrap the ClickHouse databases required by marketd.

#### Scenario: Bootstrap creates databases
- **WHEN** an operator runs `marketd bootstrap`
- **THEN** ClickHouse contains `infinity_market`
- **AND** ClickHouse contains `infinity_ops`

#### Scenario: Bootstrap is idempotent
- **WHEN** an operator runs `marketd bootstrap` multiple times
- **THEN** the command succeeds without dropping existing data
- **AND** existing tables remain queryable

### Requirement: Canonical A-share fact tables
The system SHALL store A-share OHLCV bars as canonical facts without source, version, or cross-row derived columns.

#### Scenario: Daily table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.a_share_bars_1d` exists
- **AND** it stores market, symbol, trade_date, open, high, low, close, volume, and amount
- **AND** it MUST NOT store `pct_chg`
- **AND** it uses `ReplacingMergeTree`
- **AND** it partitions by `toYear(trade_date)`
- **AND** it orders by `(market, symbol, trade_date)`

#### Scenario: One-minute table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.a_share_bars_1m` exists
- **AND** it stores market, symbol, bar_time, trade_date, open, high, low, close, volume, and amount
- **AND** it uses `ReplacingMergeTree`
- **AND** it partitions by `toYYYYMM(trade_date)`
- **AND** it orders by `(market, symbol, bar_time)`

#### Scenario: Five-minute table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.a_share_bars_5m` exists
- **AND** it stores market, symbol, bar_time, trade_date, open, high, low, close, volume, and amount
- **AND** it uses `ReplacingMergeTree`
- **AND** it partitions by `toYYYYMM(trade_date)`
- **AND** it orders by `(market, symbol, bar_time)`

#### Scenario: No source or version columns in facts
- **WHEN** a fact table is created
- **THEN** it MUST NOT include source, source_key, source_file, version, or updated_at columns

#### Scenario: No cross-row derived columns in facts
- **WHEN** a fact table is created
- **THEN** it MUST NOT include columns whose values require looking up another row in the same symbol history

### Requirement: Marketd resolves input conflicts
The system SHALL resolve conflicting input records before inserting facts into ClickHouse.

#### Scenario: Duplicate identical logical key
- **WHEN** an import contains duplicate records for the same logical key with identical normalized values
- **THEN** marketd writes one fact row for that logical key
- **AND** the import may record skipped duplicate counts

#### Scenario: Duplicate conflicting logical key
- **WHEN** an import contains duplicate records for the same logical key with different normalized values
- **THEN** marketd records a data quality issue
- **AND** marketd MUST NOT rely on ClickHouse merge order to choose the fact

### Requirement: Operational tables
The system SHALL store operator-visible watermarks, task runs, and data quality issues in `infinity_ops`.

#### Scenario: Watermark table
- **WHEN** bootstrap completes
- **THEN** `infinity_ops.watermarks` exists
- **AND** it is keyed by dataset and asset

#### Scenario: Task run table
- **WHEN** bootstrap completes
- **THEN** `infinity_ops.task_runs` exists
- **AND** it records run_id, dataset, task_type, status, target_table, input_path, input_format, params, timing, row counts, and error

#### Scenario: Quality issue table
- **WHEN** bootstrap completes
- **THEN** `infinity_ops.data_quality_issues` exists
- **AND** it records issue_id, run_id, dataset, severity, issue_type, market, symbol, logical_key, input_path, input_record_offset, observed_at, message, and details

### Requirement: Status command
The system SHALL provide an operator status command.

#### Scenario: Status reports connectivity and watermarks
- **WHEN** an operator runs `marketd status`
- **THEN** the command reports ClickHouse connectivity
- **AND** it reports the latest known watermarks from `infinity_ops.watermarks`
