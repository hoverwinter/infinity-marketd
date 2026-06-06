## ADDED Requirements

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
