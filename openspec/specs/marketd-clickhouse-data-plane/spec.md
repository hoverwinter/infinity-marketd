# marketd-clickhouse-data-plane Specification

## Purpose
TBD - created by archiving change build-tdx-clickhouse-mvp. Update Purpose after archive.
## Requirements
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
- **AND** it orders by `(market, symbol, trade_date)`

#### Scenario: One-minute table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.a_share_bars_1m` exists
- **AND** it stores market, symbol, bar_time, trade_date, open, high, low, close, volume, and amount
- **AND** it uses `ReplacingMergeTree`
- **AND** it orders by `(market, symbol, bar_time)`

#### Scenario: Five-minute table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.a_share_bars_5m` exists
- **AND** it stores market, symbol, bar_time, trade_date, open, high, low, close, volume, and amount
- **AND** it uses `ReplacingMergeTree`
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

### Requirement: A-share intraday point table
The system SHALL bootstrap a canonical A-share intraday point fact table for persisted TDX standard HQ minute-time data.

#### Scenario: Intraday point table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.a_share_intraday_points` exists
- **AND** it stores market, symbol, trade_date, point_time, point_index, price, and volume
- **AND** it uses `ReplacingMergeTree`
- **AND** it partitions by `toYYYYMM(trade_date)`
- **AND** it orders by `(market, symbol, trade_date, point_time)`

#### Scenario: Intraday point logical key
- **WHEN** marketd writes intraday point facts
- **THEN** the logical identity is market, symbol, trade_date, and point_time
- **AND** re-importing the same logical points does not require deleting existing data

#### Scenario: Intraday points are not bars
- **WHEN** `a_share_intraday_points` is created
- **THEN** it MUST NOT include open, high, low, close, amount, period, or bar_interval columns
- **AND** it MUST NOT be used as the storage target for local 1-minute OHLCV imports

#### Scenario: No source or version columns in intraday point facts
- **WHEN** `a_share_intraday_points` is created
- **THEN** it MUST NOT include source, source_key, source_file, version, or updated_at columns

### Requirement: Intraday point operational metadata
The system SHALL record intraday point imports through the existing ops tables.

#### Scenario: Intraday point task run
- **WHEN** marketd imports TDX intraday points into ClickHouse
- **THEN** it records a task run in `infinity_ops.task_runs`
- **AND** the target table identifies `a_share_intraday_points`
- **AND** the task params include market, symbol, requested date or date range, and selected TDX server options

#### Scenario: Intraday point watermark
- **WHEN** marketd successfully imports one or more intraday point rows
- **THEN** it records or advances a watermark for the imported market and symbol
- **AND** the watermark range reflects the imported point_time range

#### Scenario: Intraday point quality issues
- **WHEN** an intraday point import encounters invalid market, invalid symbol, invalid date, duplicate conflicting points, or decode failures
- **THEN** marketd records data quality issues in `infinity_ops.data_quality_issues`
- **AND** the issues include the affected market, symbol, and logical key when available

### Requirement: TDX xdxr event table
The system SHALL store normalized TDX xdxr corporate-action events outside canonical OHLCV fact tables.

#### Scenario: Xdxr event table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.a_share_xdxr_events` exists
- **AND** it stores market, symbol, event_date, category, category_name, and decoded xdxr numeric fields
- **AND** decoded numeric fields are nullable when a category does not provide them
- **AND** it uses `ReplacingMergeTree`
- **AND** it orders by `(market, symbol, event_date, category)`

#### Scenario: Xdxr events are not OHLCV columns
- **WHEN** canonical OHLCV fact tables are created
- **THEN** they MUST NOT include xdxr event fields such as `fenhong`, `peigu`, `peigujia`, `songzhuangu`, or `suogu`

### Requirement: Daily adjustment factor table
The system SHALL store reusable daily adjustment factors outside canonical OHLCV fact tables.

#### Scenario: Adjustment factor table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.a_share_adjust_factors_1d` exists
- **AND** it stores market, symbol, trade_date, qfq_factor, hfq_factor, and computed_at
- **AND** `qfq_factor` and `hfq_factor` are nullable
- **AND** it uses `ReplacingMergeTree(computed_at)`
- **AND** it partitions by `toYear(trade_date)`
- **AND** it orders by `(market, symbol, trade_date)`

#### Scenario: No adjusted K-line fact tables
- **WHEN** bootstrap completes
- **THEN** it MUST NOT create full adjusted OHLCV tables for qfq or hfq daily bars
- **AND** it MUST NOT create full adjusted OHLCV tables for qfq or hfq minute bars

#### Scenario: Factor table is rebuildable
- **WHEN** adjustment factors are refreshed
- **THEN** refreshed rows replace stale factor rows for the same market, symbol, and trade_date
- **AND** canonical OHLCV fact rows remain unchanged

### Requirement: Financial raw fact tables
The system SHALL bootstrap raw ClickHouse tables for TDX professional financial and stock metric imports.

#### Scenario: Financial raw item table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.a_share_financial_raw_items` exists
- **AND** it stores market, symbol, report_date, item_id, and value
- **AND** it uses `ReplacingMergeTree`
- **AND** it partitions by `toYear(report_date)`
- **AND** it orders by `(market, symbol, report_date, item_id)`
- **AND** it MUST NOT include source, version, or updated_at columns

#### Scenario: Stock metric raw table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.a_share_gp_metric_values` exists
- **AND** it stores market, symbol, metric_type, event_date, value1, and value2
- **AND** it uses `ReplacingMergeTree`
- **AND** it partitions by `toYear(event_date)`
- **AND** it orders by `(market, symbol, metric_type, event_date)`
- **AND** it MUST NOT include source, version, or updated_at columns

### Requirement: Financial dictionary lookup tables
The system SHALL bootstrap ClickHouse lookup tables synchronized from version-controlled TDX financial dictionaries.

#### Scenario: Financial item dictionary table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.tdx_financial_item_dictionary` exists
- **AND** it stores item_id, stable field name, display title, category, unit or value kind, source reference, and confirmation status
- **AND** it is keyed by item_id

#### Scenario: Stock metric dictionary table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.tdx_gp_metric_dictionary` exists
- **AND** it stores metric_type, stable field name, display title, value1 meaning, value2 meaning, source reference, and confirmation status
- **AND** it is keyed by metric_type

### Requirement: A-share capital-change event table
The system SHALL bootstrap a ClickHouse table for client-local TDX `gbbq` capital-change and corporate-action events.

#### Scenario: Capital-change table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.a_share_capital_change_events` exists
- **AND** it stores market, symbol, event_date, category, event_seq, event_name, cash_dividend, allotment_price, bonus_shares, allotment_shares, shrink_shares, pre_float_shares, post_float_shares, pre_total_shares, post_total_shares, ratio_denominator, and exercise_price
- **AND** it uses `ReplacingMergeTree`
- **AND** it partitions by `toYear(event_date)`
- **AND** it orders by `(market, symbol, event_date, category, event_seq)`
- **AND** it MUST NOT include source, source_file, version, or updated_at columns

### Requirement: TDX block snapshot tables
The system SHALL bootstrap ClickHouse tables for client-local TDX block snapshots, block definitions, and block memberships.

#### Scenario: Block snapshot table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.tdx_block_snapshots` exists
- **AND** it stores snapshot_id, block_scope, snapshot_time, content_hash, block_count, and member_count
- **AND** it uses `ReplacingMergeTree`
- **AND** it partitions by `toYYYYMM(snapshot_time)`
- **AND** it orders by `(block_scope, snapshot_time, snapshot_id)`

#### Scenario: Block definition table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.tdx_block_definitions` exists
- **AND** it stores snapshot_id, block_scope, block_kind, block_id, block_name, block_type, display_order, and member_count
- **AND** it uses `ReplacingMergeTree`
- **AND** it orders by `(snapshot_id, block_scope, block_id)`

#### Scenario: Block membership table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.tdx_block_memberships` exists
- **AND** it stores snapshot_id, block_scope, block_id, member_order, code, market, and symbol
- **AND** it uses `ReplacingMergeTree`
- **AND** it orders by `(snapshot_id, block_scope, block_id, market, symbol, member_order)`

### Requirement: Extension-market daily bar table
The system SHALL bootstrap a ClickHouse table for client-local TDX extension-market daily bars.

#### Scenario: Extension-market daily table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.tdx_ex_bars_1d` exists
- **AND** it stores ex_market, code, trade_date, open, high, low, close, position, trade, price, amount, and settlement_price
- **AND** it uses `ReplacingMergeTree`
- **AND** it partitions by `toYear(trade_date)`
- **AND** it orders by `(ex_market, code, trade_date)`
- **AND** it MUST NOT include source, source_file, version, or updated_at columns

### Requirement: Daily derived metrics table
The system SHALL store reusable daily derived metrics outside canonical OHLCV fact tables.

#### Scenario: Derived table contract
- **WHEN** bootstrap completes
- **THEN** `infinity_market.a_share_daily_derived` exists
- **AND** it stores market, symbol, trade_date, prev_close, pct_chg, and computed_at
- **AND** `prev_close` and `pct_chg` are nullable
- **AND** it orders by `(trade_date, market, symbol)`

#### Scenario: Percentage change calculation
- **WHEN** daily derived metrics are refreshed
- **THEN** `prev_close` is the previous valid close for the same market and symbol
- **AND** `pct_chg` is `(close - prev_close) / prev_close * 100`
- **AND** `pct_chg` is `NULL` when `prev_close` is missing or non-positive

#### Scenario: Rebuildable derived data
- **WHEN** historical daily facts are backfilled or corrected
- **THEN** an operator can refresh affected daily derived metrics
- **AND** refreshed derived rows replace stale derived values for the same market, symbol, and trade_date

### Requirement: Minute scan-derived tables
The system SHALL support optional short-retention scan tables for full-market minute scans without changing canonical minute fact table order keys.

#### Scenario: One-minute scan table contract
- **WHEN** scan table bootstrap is implemented
- **THEN** `infinity_market.a_share_bars_1m_scan` exists
- **AND** it stores trade_date, bar_time, market, symbol, close, volume, amount, selected nullable derived metrics, and computed_at
- **AND** it partitions by `toYYYYMM(trade_date)`
- **AND** it orders by `(trade_date, bar_time, market, symbol)`
- **AND** it is rebuildable from `infinity_market.a_share_bars_1m`

#### Scenario: Five-minute scan table contract
- **WHEN** scan table bootstrap is implemented
- **THEN** `infinity_market.a_share_bars_5m_scan` exists
- **AND** it stores trade_date, bar_time, market, symbol, close, volume, amount, selected nullable derived metrics, and computed_at
- **AND** it partitions by `toYYYYMM(trade_date)`
- **AND** it orders by `(trade_date, bar_time, market, symbol)`
- **AND** it is rebuildable from `infinity_market.a_share_bars_5m`

#### Scenario: Scan table retention
- **WHEN** scan tables are created
- **THEN** they are treated as short-retention derived data
- **AND** operators can delete expired scan partitions without deleting canonical minute facts
- **AND** expired scan data can be rebuilt from canonical minute facts when needed

### Requirement: Explicit minute scan refresh
The system SHALL generate minute scan data only through explicit refresh operations.

#### Scenario: Refresh one-minute scan data
- **WHEN** an operator runs the scan refresh for period `1m` and a date range
- **THEN** marketd rebuilds scan rows from `a_share_bars_1m` for that range
- **AND** refreshed rows replace stale rows for the same market, symbol, and bar_time

#### Scenario: Refresh five-minute scan data
- **WHEN** an operator runs the scan refresh for period `5m` and a date range
- **THEN** marketd rebuilds scan rows from `a_share_bars_5m` for that range
- **AND** refreshed rows replace stale rows for the same market, symbol, and bar_time

