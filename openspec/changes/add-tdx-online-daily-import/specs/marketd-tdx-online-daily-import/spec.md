## ADDED Requirements

### Requirement: Online daily import command
The system SHALL provide an explicit `marketd import-tdx-hq-day` command that imports TDX standard HQ security daily K-line rows into canonical A-share daily facts.

#### Scenario: Import one symbol
- **WHEN** an operator runs `marketd import-tdx-hq-day --market sh --symbol 600519`
- **THEN** the command requests standard HQ security daily K-line data for that symbol
- **AND** it writes normalized raw OHLCV rows to `infinity_market.a_share_bars_1d`
- **AND** it records task run and watermark metadata for asset `sh:600519`

#### Scenario: Dry run
- **WHEN** an operator runs `marketd import-tdx-hq-day` with `--dry-run`
- **THEN** the command fetches, decodes, normalizes, and validates provider rows
- **AND** it reports target table, row counts, skipped rows, and quality issue count
- **AND** it MUST NOT write market fact rows, task runs, watermarks, or quality issues

#### Scenario: Reject invalid request
- **WHEN** an operator omits market or symbol, provides an unsupported market, provides an invalid symbol, provides an invalid date bound, or provides an inverted date range
- **THEN** the command rejects the request before writing ClickHouse rows

### Requirement: Online daily paging and bounds
The system SHALL page TDX standard HQ daily K-line reads and filter imported rows to the requested date bounds.

#### Scenario: Respect provider page limit
- **WHEN** online daily import needs more than one provider page
- **THEN** it requests pages with count no greater than the TDX standard HQ K-line maximum
- **AND** it advances the provider start offset between pages

#### Scenario: Date-bound import
- **WHEN** an operator provides `--since` and/or `--until`
- **THEN** the command writes only rows whose decoded trade date is within the requested inclusive bounds
- **AND** it counts rows outside the bounds as skipped

#### Scenario: Empty provider result
- **WHEN** the selected TDX server returns no daily K-line rows for a valid request
- **THEN** the command records an operator-visible quality issue in non-dry-run mode
- **AND** it does not write synthetic daily bars

### Requirement: Online daily normalization
The system SHALL normalize online TDX daily K-line rows into the existing raw daily bar model without adding provider metadata to market fact rows.

#### Scenario: Normalize raw daily fields
- **WHEN** a provider daily K-line row is valid
- **THEN** it maps to market, symbol, trade_date, open, high, low, close, volume, and amount
- **AND** it uses logical key `(market, symbol, trade_date)`
- **AND** it writes to `infinity_market.a_share_bars_1d`

#### Scenario: Duplicate provider rows
- **WHEN** one online import response contains duplicate rows for the same market, symbol, and trade date
- **THEN** the importer writes at most one row for that logical key
- **AND** it records a quality issue for duplicates or conflicts

#### Scenario: Invalid provider row
- **WHEN** a provider row has a missing trade date, mismatched market or symbol, negative volume, negative amount, or `high < low`
- **THEN** the importer skips that row
- **AND** it records a quality issue that identifies the affected logical key when available

### Requirement: Online daily import boundaries
The system SHALL keep online daily import independent from provider read APIs and adjustment refreshes.

#### Scenario: Provider reads remain read-only
- **WHEN** an operator or HTTP client calls `hq-bars`, `/api/tdx/hq/bars`, or another `/api/tdx/*` provider endpoint
- **THEN** the online daily importer MUST NOT run as a side effect
- **AND** those provider reads MUST NOT write to `a_share_bars_1d`

#### Scenario: Stable queries remain ClickHouse-backed
- **WHEN** an HTTP client calls `/api/v1/bars`
- **THEN** the query path MUST NOT fetch upstream TDX data to fill missing daily bars
- **AND** it reads persisted daily bars from ClickHouse

#### Scenario: Adjustment refresh remains explicit
- **WHEN** online daily import writes raw daily bars
- **THEN** it MUST NOT refresh TDX XDXR events
- **AND** it MUST NOT refresh daily adjustment factors
- **AND** adjusted `/api/v1/bars?adjust=qfq|hfq` continues to require persisted factors

### Requirement: Console immediate online daily import
The system SHALL provide a Console operator action that immediately triggers a single-symbol online daily import.

#### Scenario: Trigger import from Console API
- **WHEN** an operator submits `POST /api/console/imports/tdx-hq-day` with market, symbol, and optional bounds
- **THEN** the server runs the same online daily importer used by `marketd import-tdx-hq-day`
- **AND** it returns the import summary as JSON
- **AND** successful non-dry-run imports write raw daily bars and ops metadata

#### Scenario: Console dry run
- **WHEN** the Console import request includes `dry_run=true`
- **THEN** the server fetches, decodes, normalizes, and validates provider rows
- **AND** it returns the summary as JSON
- **AND** it MUST NOT write market fact rows, task runs, watermarks, or quality issues

#### Scenario: Reject invalid Console request
- **WHEN** the Console import request has invalid market, symbol, date bounds, start, count, server candidates, or dry-run value
- **THEN** the server returns a validation error
- **AND** it MUST NOT write ClickHouse rows

#### Scenario: No scheduled execution
- **WHEN** the Console import action is submitted
- **THEN** the server executes only the requested immediate import
- **AND** it MUST NOT create delayed, recurring, or scheduled jobs
