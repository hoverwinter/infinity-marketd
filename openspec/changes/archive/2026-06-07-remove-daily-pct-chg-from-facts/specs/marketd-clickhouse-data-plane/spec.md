## MODIFIED Requirements

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

## ADDED Requirements

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
