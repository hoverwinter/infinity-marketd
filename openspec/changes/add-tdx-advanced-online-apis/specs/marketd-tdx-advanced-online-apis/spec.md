## ADDED Requirements

### Requirement: Sorted market quote list

`marketd` SHALL support TDX `hq` sorted quote list reads for scanner-style market lists.

#### Scenario: Fetch sorted quote list

- **WHEN** an operator requests a quote list with market category, sort key, start, count, and order
- **THEN** `marketd` SHALL send the corresponding TDX online request and return structured rows with market, symbol, name when available, price fields, volume, amount, turnover-related values when available, sort key metadata, and raw protocol identifiers

#### Scenario: Validate quote list pagination

- **WHEN** the caller provides `start < 0`, `count <= 0`, or a count above the configured maximum
- **THEN** `marketd` SHALL reject the request before contacting a TDX server

#### Scenario: Exclude stock types

- **WHEN** the caller provides an exclude bitmask or named exclude filters
- **THEN** `marketd` SHALL encode the filter in the request and include the effective exclude value in the response metadata

### Requirement: Top board rankings

`marketd` SHALL support TDX `hq` top board reads that return multiple ranking groups in one response.

#### Scenario: Fetch top board

- **WHEN** an operator requests top boards for a category and size
- **THEN** `marketd` SHALL return grouped ranking lists for supported groups such as gainers, losers, amplitude, speed, volume ratio, commission ratio, and turnover

#### Scenario: Preserve ranking identity

- **WHEN** a top board response contains a ranking group
- **THEN** each returned row SHALL include the group name, raw group id, rank order, market, symbol, and decoded quote metrics available in the protocol row

#### Scenario: Validate top board size

- **WHEN** the caller provides an unsupported size
- **THEN** `marketd` SHALL reject the request before contacting a TDX server

### Requirement: SP live board members

`marketd` SHALL support SP/MAC protocol live board member reads separately from static HQ block-file and client-local block imports.

#### Scenario: Fetch SP board members

- **WHEN** an operator requests members for a board id with sort type, count, and order
- **THEN** `marketd` SHALL perform the SP/MAC online request and return member rows with board id, market, symbol, rank order, decoded known fields, raw bitmap fields, sort type, and sort order

#### Scenario: Auto paginate board members

- **WHEN** the requested board member count exceeds one upstream response page
- **THEN** `marketd` SHALL fetch additional pages until it has the requested count, receives an empty page, or reaches the configured maximum

#### Scenario: Keep static block APIs unchanged

- **WHEN** callers use existing HQ block-file reads or client-local block imports
- **THEN** those APIs SHALL continue to return static block definitions/memberships and SHALL NOT use the new SP board member protocol implicitly

### Requirement: LHB records from F10

`marketd` SHALL support Dragon-Tiger list extraction from TDX F10 company information.

#### Scenario: Parse LHB from F10

- **WHEN** an operator requests LHB records for a symbol
- **THEN** `marketd` SHALL fetch F10 categories, locate the `资金动向` section or configured aliases, fetch the corresponding content, and return structured LHB records

#### Scenario: Missing LHB section

- **WHEN** the F10 category list does not contain an LHB-compatible section
- **THEN** `marketd` SHALL return an empty result with metadata explaining that the section was not found

#### Scenario: Parser fixture coverage

- **WHEN** the LHB parser receives stored F10 text fixtures with known Dragon-Tiger records
- **THEN** it SHALL decode dates, reasons, buy/sell seats, amounts, and net amounts when those fields are present

### Requirement: Fund-specific 7727 reads

`marketd` SHALL support fund-specialized TDX 7727 reads for fund K-line and raw fund detail.

#### Scenario: Fetch fund K-line

- **WHEN** an operator requests a fund K-line by code, period, and count
- **THEN** `marketd` SHALL perform the fund 7727 bootstrap/request path and return normalized bars with fund code, period, time, OHLC, volume, amount when available, and raw category metadata

#### Scenario: Fetch fund detail

- **WHEN** an operator requests fund detail by code and optional mode
- **THEN** `marketd` SHALL return fund detail rows with item id and raw value array, plus any confirmed decoded labels

#### Scenario: Keep generic ExHQ bars separate

- **WHEN** callers use existing `exquote-bars` or `/api/tdx/exhq/bars`
- **THEN** those APIs SHALL continue using the generic ExHQ path and SHALL NOT silently switch to the fund-specialized path

### Requirement: CLI and provider API productization

`marketd` SHALL expose advanced online reads through CLI commands and `infinity` TDX provider endpoints.

#### Scenario: CLI commands exist

- **WHEN** an operator runs `marketd help`
- **THEN** the advanced online commands SHALL be discoverable for quote lists, top boards, SP board members, LHB, fund K-line, and fund detail

#### Scenario: Provider endpoints exist

- **WHEN** `infinity querier serve` is running
- **THEN** it SHALL expose `/api/tdx/*` endpoints for advanced quote lists, top boards, SP board members, LHB, fund K-line, and fund detail

#### Scenario: Provider boundary

- **WHEN** advanced online endpoints are called
- **THEN** they SHALL return live upstream reads and SHALL NOT query or write ClickHouse-backed `/api/v1` data

### Requirement: Server selection and reliability

`marketd` SHALL reuse existing operational patterns for server selection, timeouts, fallback, and testability.

#### Scenario: Explicit server override

- **WHEN** a caller provides one or more servers
- **THEN** `marketd` SHALL use those servers in order and SHALL NOT silently replace them with defaults

#### Scenario: BestIP support where applicable

- **WHEN** an advanced `hq` command supports BestIP and the caller enables it
- **THEN** `marketd` SHALL use the existing HQ BestIP cache workflow

#### Scenario: Decoder tests do not require public servers

- **WHEN** tests validate advanced protocol decoders
- **THEN** they SHALL use fixtures or fake TDX servers and SHALL NOT require public TDX server availability

#### Scenario: No hidden persistence

- **WHEN** any advanced online read succeeds
- **THEN** `marketd` SHALL NOT write market fact tables, derived tables, or ops records unless a command explicitly documents an ops-only side effect
