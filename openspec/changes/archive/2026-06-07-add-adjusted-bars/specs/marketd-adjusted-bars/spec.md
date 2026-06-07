## ADDED Requirements

### Requirement: TDX xdxr event refresh
The system SHALL refresh and persist normalized TDX xdxr corporate-action events for supported A-share symbols.

#### Scenario: Refresh single symbol xdxr events
- **WHEN** an operator refreshes xdxr events for `market=sh` and `symbol=600519`
- **THEN** the system requests TDX HQ xdxr data for that symbol
- **AND** it writes normalized rows to the xdxr event table
- **AND** it records task run and watermark metadata

#### Scenario: Empty xdxr response
- **WHEN** TDX returns no xdxr rows for a valid supported symbol
- **THEN** the refresh completes without writing event rows
- **AND** it records an operator-visible watermark for that symbol

#### Scenario: Unsupported xdxr event category
- **WHEN** an xdxr row has a category that factor generation does not support
- **THEN** the system persists the normalized event when it can be decoded
- **AND** factor refresh MUST NOT silently use unsupported fields in price factors

### Requirement: Daily adjustment factor refresh
The system SHALL generate daily qfq and hfq adjustment factors from canonical daily bars and persisted xdxr events.

#### Scenario: Ordinary dividend and share event factor
- **WHEN** a symbol has category `1` xdxr events with valid `fenhong`, `peigu`, `peigujia`, and `songzhuangu` values
- **THEN** the factor refresh calculates the event theoretical previous close from the previous valid raw close
- **AND** it derives qfq and hfq factors for affected trading dates
- **AND** it writes rows to the daily adjustment factor table

#### Scenario: Qfq anchor
- **WHEN** qfq factors are refreshed for a symbol
- **THEN** the latest available raw daily bar for that symbol has `qfq_factor = 1.0`
- **AND** older trading dates accumulate later corporate-action ratios

#### Scenario: Hfq anchor
- **WHEN** hfq factors are refreshed for a symbol
- **THEN** the earliest available raw daily bar for that symbol has `hfq_factor = 1.0`
- **AND** later trading dates accumulate corporate-action ratios forward

#### Scenario: Missing or invalid factor inputs
- **WHEN** a factor requires missing, zero, negative, or unsupported inputs
- **THEN** the refresh records a data quality issue
- **AND** the affected factor value is nullable rather than fabricated

#### Scenario: Rebuildable factor history
- **WHEN** raw daily bars or xdxr events are backfilled or corrected
- **THEN** an operator can refresh the affected symbol factor history
- **AND** refreshed factor rows replace stale factor rows for the same market, symbol, and trade_date

### Requirement: Adjusted bar query
The system SHALL support adjusted OHLC queries through the stable `/api/v1/bars` read API.

#### Scenario: Default raw bars
- **WHEN** a client requests `/api/v1/bars` without `adjust`
- **THEN** the query behaves as an unadjusted query
- **AND** response OHLC, volume, and amount values come from canonical raw bar facts

#### Scenario: Qfq daily bars
- **WHEN** a client requests daily bars with `adjust=qfq`
- **THEN** the response OHLC values equal raw OHLC values multiplied by `qfq_factor`
- **AND** the response query echo includes `adjust=qfq`
- **AND** volume and amount remain raw values

#### Scenario: Hfq daily bars
- **WHEN** a client requests daily bars with `adjust=hfq`
- **THEN** the response OHLC values equal raw OHLC values multiplied by `hfq_factor`
- **AND** the response query echo includes `adjust=hfq`
- **AND** volume and amount remain raw values

#### Scenario: Adjusted minute bars
- **WHEN** a client requests 1-minute or 5-minute bars with `adjust=qfq` or `adjust=hfq`
- **THEN** the system joins the minute bar trade_date to the daily adjustment factor for the same market and symbol
- **AND** it adjusts minute OHLC values with that daily factor
- **AND** volume and amount remain raw values

#### Scenario: Invalid adjust value
- **WHEN** a client requests `/api/v1/bars` with an unsupported `adjust` value
- **THEN** the system returns a validation error
- **AND** it MUST NOT run a ClickHouse query

#### Scenario: Missing adjustment factor
- **WHEN** a client requests adjusted bars for rows without required factors
- **THEN** the system returns an error or an explicit missing-factor signal according to the documented API contract
- **AND** it MUST NOT silently mix raw and adjusted OHLC values in one successful adjusted response

### Requirement: No live TDX dependency in adjusted queries
The system SHALL keep adjusted `/api/v1/bars` queries backed by persisted ClickHouse data.

#### Scenario: Adjusted query does not call live TDX
- **WHEN** a client requests `/api/v1/bars?adjust=qfq`
- **THEN** the system reads raw bars and adjustment factors from ClickHouse
- **AND** it MUST NOT request `/api/tdx/hq/xdxr` or connect to a live TDX HQ server inside the query path
