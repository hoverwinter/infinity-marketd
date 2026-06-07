## ADDED Requirements

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
