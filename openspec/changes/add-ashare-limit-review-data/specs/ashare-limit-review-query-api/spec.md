## ADDED Requirements

### Requirement: Focused limit review queries
The system SHALL expose ClickHouse-backed read-only endpoints for limit events, daily summaries, relay events, themes, dedicated performance indices, and market breadth.

#### Scenario: Query a day's event pool
- **WHEN** a client calls `GET /api/v1/limit-events` with a valid trade date and optional event type
- **THEN** the response contains matching normalized event rows in deterministic order

#### Scenario: Query one stock history
- **WHEN** a client calls `GET /api/v1/limit-events` with market, symbol, and date bounds
- **THEN** the response identifies the dates, event types, reasons, and themes for that stock

#### Scenario: Search stored reasons by keyword
- **WHEN** a client calls `GET /api/v1/limit-events` with `reason_keyword` and a valid date or date range
- **THEN** the system matches a literal substring of the final stored `reason_text`, supporting Chinese and ignoring English letter case
- **AND** surrounding whitespace is trimmed, an empty keyword applies no reason filter, and punctuation including `%`, `_`, `+` and quotes is literal rather than a wildcard or search expression
- **AND** the reason condition is combined with other supplied filters using AND before deterministic ordering and pagination
- **AND** unmatched queries return an empty array, and other review endpoints reject a nonempty reason keyword with HTTP 400
- **AND** invalid UTF-8 keywords are rejected with HTTP 400

#### Scenario: Query yesterday-limit-up outcomes
- **WHEN** a client calls `GET /api/v1/limit-relay` with a trade date and sample group
- **THEN** the response contains per-stock next-day statuses and normalized percentage values

#### Scenario: Query breadth or performance series
- **WHEN** a client calls the breadth or performance-index endpoint with valid date bounds
- **THEN** the response contains only persisted rows ordered by trade date
- **AND** the querier does not call a live provider to fill gaps

### Requirement: Daily review reconstruction
The system SHALL reconstruct one trading day's objective review from normalized persisted tables.

#### Scenario: Complete day response
- **WHEN** a client calls `GET /api/v1/limit-review?trade_date=YYYY-MM-DD`
- **THEN** the response contains the date's summary, breadth, performance indices, limit-up pool, broken-board pool, limit-down pool, ladder groups, relay rows, and themes
- **AND** missing optional datasets are represented by null objects or empty arrays rather than request failure

#### Scenario: Objective response only
- **WHEN** the daily review is reconstructed
- **THEN** the API does not fabricate subjective headlines, emotion analysis, or risk conclusions

### Requirement: Limit review query validation
The system SHALL validate dates, markets, symbols, enums, date ranges, and bounded limits before executing ClickHouse queries.

#### Scenario: Invalid request
- **WHEN** a request contains an invalid date, unsupported enum, malformed symbol, inverted range, or non-positive explicit limit
- **THEN** the querier returns HTTP 400 with a validation error

#### Scenario: Bounded defaults
- **WHEN** a list query omits limit
- **THEN** the querier applies a bounded default and returns the normalized query in the response

### Requirement: Correction-sensitive range reconstruction
The system SHALL reconstruct relay and themes from final events for ranges of at most 93 calendar days.

#### Scenario: Previous event corrected
- **WHEN** a previous-day event changes board count, reason or primary theme
- **THEN** range relay reflects the corrected sample membership and values where base events are available
- **AND** range themes aggregate and rank independently for each date before filtering and pagination

#### Scenario: Missing facts or excessive reconstruction
- **WHEN** base dates or events are missing
- **THEN** the query falls back to existing materializations without claiming complete coverage
- **AND** an oversized or incomplete input page fails rather than silently truncating an aggregate

### Requirement: Bounded review matrix API
The system SHALL expose a read-only stock-by-date matrix using the existing review repositories.

#### Scenario: Stock selection and date cells
- **WHEN** a client requests `/api/v1/limit-review-matrix` with a valid bounded range and optional event/theme filters
- **THEN** matching stocks are paginated with default 100 and maximum 500 rows
- **AND** selected stocks retain all their available event and relay cells in the range
- **AND** date headers include available summary, breadth, indices and themes

#### Scenario: Unknown cells
- **WHEN** a date or stock cell has no recorded data
- **THEN** the response does not invent a trading day, zero return, suspension, price chart or subjective narrative
