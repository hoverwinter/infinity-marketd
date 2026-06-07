## ADDED Requirements

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
