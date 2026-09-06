## ADDED Requirements

### Requirement: Optional online capabilities and explicit source identity
The system SHALL register TDX, THS and Eastmoney by default and expose optional bars and board capabilities, with provider-scoped instruments and explicit supported markets, kinds and periods. Unknown sources and unsupported capabilities SHALL fail without invoking a different provider. Configuring THS credentials SHALL preserve other registered providers.

#### Scenario: Three sources are available
- **WHEN** the normal querier starts, with or without a configured THS cookie
- **THEN** `/api/providers` lists `eastmoney`, `tdx` and `ths` and all three are reachable through the common bars route

#### Scenario: Unsupported request
- **WHEN** a caller requests THS minute bars or a TDX board catalog through the common API
- **THEN** the system returns an unsupported-capability error without an upstream request or fallback

### Requirement: Common bar range semantics
Bars SHALL accept explicit instrument identity, inclusive date bounds and unadjusted periods, reject invalid/future/over-ten-year ranges, and return chronological unique rows with timezone and volume/amount units. Invalid OHLC, non-finite numbers and conflicting duplicates SHALL cause an error. Scan limits or failed pages SHALL NOT become successful truncated results.

#### Scenario: Invalid range
- **WHEN** a caller supplies reversed dates, an invalid date or unsupported adjustment
- **THEN** validation fails before network access

#### Scenario: Conflicting observations
- **WHEN** upstream rows repeat a timestamp with different values
- **THEN** the adapter rejects the response rather than selecting an arbitrary value

### Requirement: THS board and historical daily data
The THS adapter SHALL fetch current industry/concept catalogs, resolve page IDs to quotation instruments using the detail page, and parse annual daily K-line files only for requested years. It SHALL bound transport resources and distinguish failed/challenged/malformed responses from valid empty data.

#### Scenario: Concept code resolution
- **WHEN** concept page `301558` declares quotation code `885611`
- **THEN** resolution returns the `board` index instrument with symbol `885611` and bars use that quotation symbol

#### Scenario: A year fails
- **WHEN** one annual request fails or returns challenge HTML
- **THEN** the entire range request fails and does not report the remaining years as a complete success

### Requirement: TDX bars use the shared contract
The TDX adapter SHALL reuse existing security/index wire clients, map supported periods, page backwards to the requested date range, validate progress and disclose exhausted history boundaries.

#### Scenario: Multi-page range
- **WHEN** the requested range spans multiple TDX pages
- **THEN** the adapter advances the reverse offset, filters inclusive bounds and returns ascending bars with native volume units

#### Scenario: History unavailable
- **WHEN** TDX exhausts available history before reaching the lower date bound
- **THEN** the result includes an explicit history-boundary warning

### Requirement: Live API and CLI isolation
The system SHALL provide provider discovery, bars, board catalog and board resolution through `/api/providers` and thin `infinity querier` commands. These requests SHALL NOT query or mutate database facts or invoke imports. Existing `/api/v1` and `/api/tdx` behavior SHALL remain compatible.

#### Scenario: Source failure
- **WHEN** an online source is unavailable
- **THEN** the common API returns a classified upstream error and does not invoke the canonical repository

#### Scenario: CLI uses HTTP
- **WHEN** the user runs `infinity querier provider-bars`
- **THEN** the CLI sends the selected provider and normalized query fields to the HTTP service

### Requirement: Eastmoney catalogs and category-checked board identity
The Eastmoney adapter SHALL fetch industry/concept catalogs with stable code ordering, determine the effective page size, validate unchanged total and exact unique row counts, and fail atomically on incomplete or inconsistent scans. Resolution SHALL confirm the requested category before mapping the board code to an index instrument in market `board`.

#### Scenario: Upstream caps page size
- **WHEN** a request for 100 items returns 20 items with a declared total of 45
- **THEN** the adapter fetches three pages with 20, 20 and 5 distinct items and returns all 45

#### Scenario: Inconsistent pagination
- **WHEN** a later page repeats a code, changes total, fails, or has an unexpected length
- **THEN** the entire catalog request fails without returning a partial list

#### Scenario: Wrong board category
- **WHEN** a concept board is resolved under the industry category
- **THEN** the adapter rejects the resolution rather than returning an unchecked quotation identity

### Requirement: Eastmoney daily index data
The Eastmoney adapter SHALL implement common board-index daily bars using `secid=90.BK...`, `klt=101` and `fqt=0`, bounded date chunks, validated response identity and the documented open/close/high/low field order. It SHALL distinguish empty arrays from missing/null data and failed responses; any failed chunk SHALL fail the whole request. It SHALL disclose unverified native volume units and historical coverage limitations.

#### Scenario: Distinct field order
- **WHEN** a K-line row contains date, open=10, close=11, high=12 and low=9
- **THEN** the common result returns open=10, high=12, low=9 and close=11

#### Scenario: Failed year or wrong instrument
- **WHEN** any requested year fails, returns null data or reports another market/code
- **THEN** the adapter returns a classified error and no partial history
