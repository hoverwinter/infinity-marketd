## ADDED Requirements

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
