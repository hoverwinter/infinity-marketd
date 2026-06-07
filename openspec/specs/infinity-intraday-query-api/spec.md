# infinity-intraday-query-api Specification

## Purpose
TBD - created by archiving change add-a-share-intraday-points. Update Purpose after archive.
## Requirements
### Requirement: Persisted intraday point query API
The system SHALL expose persisted A-share intraday points through the ClickHouse-backed querier API.

#### Scenario: Query intraday points for one date
- **WHEN** an HTTP client calls `GET /api/v1/intraday-points` with market, symbol, and date
- **THEN** the querier returns persisted rows from `infinity_market.a_share_intraday_points`
- **AND** rows are ordered by point_time ascending

#### Scenario: Query intraday points by time range
- **WHEN** an HTTP client calls `GET /api/v1/intraday-points` with market, symbol, since, and until
- **THEN** the querier returns persisted points whose point_time values are inside the requested range
- **AND** rows are ordered by point_time ascending

#### Scenario: Intraday point response shape
- **WHEN** the querier returns intraday points
- **THEN** each row includes market, symbol, trade_date, point_time, point_index, price, and volume
- **AND** the response includes the normalized query parameters

#### Scenario: Empty persisted result
- **WHEN** no persisted intraday points match a valid request
- **THEN** the querier returns an empty points array
- **AND** it does not call a live TDX server to fill missing data

### Requirement: Intraday point query validation
The system SHALL validate intraday point query parameters before reading ClickHouse.

#### Scenario: Reject invalid query parameters
- **WHEN** an HTTP client provides an unsupported market, invalid six-digit symbol, invalid date, invalid datetime, inverted time range, or non-positive limit
- **THEN** the querier returns a validation error

#### Scenario: Default bounded query limit
- **WHEN** an HTTP client omits limit for an intraday point query
- **THEN** the querier applies a bounded default limit
- **AND** the applied limit is present in the response query object

### Requirement: Intraday point CLI query
The system SHALL provide an `infinity querier` CLI command for persisted intraday point queries.

#### Scenario: CLI queries persisted intraday points
- **WHEN** an operator runs `infinity querier intraday-points` with API URL, market, symbol, and date or time bounds
- **THEN** the CLI calls `/api/v1/intraday-points`
- **AND** it prints the JSON response without connecting directly to ClickHouse

