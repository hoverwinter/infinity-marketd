## ADDED Requirements

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
