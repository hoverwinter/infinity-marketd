## ADDED Requirements

### Requirement: TDX intraday point import command
The system SHALL provide an explicit marketd command to fetch and persist TDX standard HQ minute-time points for A-share symbols.

#### Scenario: Import historical intraday points for one date
- **WHEN** an operator runs `marketd import-tdx-intraday-points` with market, symbol, and a historical trade date
- **THEN** marketd requests `get_history_minute_time_data` equivalent data for that date
- **AND** it writes decoded points to `infinity_market.a_share_intraday_points`

#### Scenario: Import current-day intraday points
- **WHEN** an operator runs `marketd import-tdx-intraday-points` for the current trading day
- **THEN** marketd requests `get_minute_time_data` equivalent data
- **AND** it writes decoded points using a deterministic trade_date in `Asia/Shanghai`

#### Scenario: Import bounded date range
- **WHEN** an operator runs `marketd import-tdx-intraday-points` with a since and until date
- **THEN** marketd fetches historical minute-time data one date at a time
- **AND** it writes all returned points within the requested date range

#### Scenario: Dry run
- **WHEN** an operator runs the intraday point import command with `--dry-run`
- **THEN** marketd fetches and decodes the requested data
- **AND** it reports row counts and issues without writing ClickHouse market fact rows

### Requirement: Intraday point normalization
The system SHALL normalize TDX standard HQ minute-time responses into persisted intraday point facts without changing their data shape.

#### Scenario: Normalize point fields
- **WHEN** marketd receives decoded TDX HQ minute-time points
- **THEN** each point maps to market, symbol, trade_date, point_time, point_index, price, and volume
- **AND** point_time is represented in `Asia/Shanghai`
- **AND** price is stored as the decoded decimal price
- **AND** volume is stored as the decoded TDX minute-time volume field

#### Scenario: Preserve empty historical response
- **WHEN** the selected TDX server returns no historical minute-time points for a requested date
- **THEN** marketd treats the date as an empty result rather than a decode failure
- **AND** it does not write synthetic point rows

#### Scenario: Reject invalid import request
- **WHEN** the operator provides an unsupported market, invalid six-digit symbol, invalid date, inverted date range, or missing required date selection
- **THEN** marketd rejects the request before writing ClickHouse rows

#### Scenario: Resolve duplicate response points
- **WHEN** one TDX response contains duplicate points for the same market, symbol, trade_date, and point_time
- **THEN** marketd writes one row for identical duplicate values
- **AND** it records a data quality issue and skips conflicting duplicate values

### Requirement: Intraday points do not modify OHLCV facts
The system SHALL keep TDX intraday point persistence independent from canonical OHLCV fact tables.

#### Scenario: Intraday import does not write one-minute bars
- **WHEN** marketd imports TDX intraday points
- **THEN** it MUST NOT write rows to `infinity_market.a_share_bars_1m`

#### Scenario: Live provider reads remain read-only
- **WHEN** an operator or HTTP client calls existing live TDX minute-time read commands or `/api/tdx/*` endpoints
- **THEN** the system returns decoded live data
- **AND** it MUST NOT write rows to `infinity_market.a_share_intraday_points`
