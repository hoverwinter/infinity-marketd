# tdx-provider-http-api Specification

## Purpose
TBD - created by archiving change add-tdx-provider-http-api. Update Purpose after archive.
## Requirements
### Requirement: TDX provider API namespace
The system SHALL expose TDX protocol reads under a provider-specific HTTP namespace that is separate from ClickHouse-backed market data APIs.

#### Scenario: TDX APIs use provider namespace
- **WHEN** an HTTP client calls TDX standard行情 or extended行情 reads
- **THEN** the route MUST start with `/api/tdx/`
- **AND** it MUST NOT use `/api/v1/bars` or other ClickHouse-backed market data paths

#### Scenario: ClickHouse query APIs remain stable
- **WHEN** an HTTP client calls existing ClickHouse-backed query APIs
- **THEN** `/api/v1/health`, `/api/v1/bars`, and `/api/v1/resolve-symbol` continue to behave as product/query APIs
- **AND** they MUST NOT initiate live TDX network requests

### Requirement: TDX standard HQ HTTP reads
The system SHALL expose TDX standard行情 (`hq`) reads through `/api/tdx/hq/*` routes.

#### Scenario: Fetch standard realtime quotes
- **WHEN** a client requests `/api/tdx/hq/quotes` with one or more A-share symbols
- **THEN** the system fetches realtime quote snapshots using the existing TDX HQ quote implementation
- **AND** the response includes decoded quote fields, bid levels, ask levels, server intraday time, and optional resolved quote time

#### Scenario: Probe standard HQ servers
- **WHEN** a client requests `/api/tdx/hq/probe`
- **THEN** the system probes configured or default TDX HQ servers
- **AND** the response identifies successful servers and the preferred reachable server

#### Scenario: Read standard HQ auxiliary data
- **WHEN** a client requests supported `/api/tdx/hq/*` read endpoints for security lists, K-lines, minute data, transactions, company/F10, xdxr, finance, or block data
- **THEN** the system maps the request to existing `internal/tdx` HQ read operations
- **AND** it preserves the protocol limits and validation rules of those operations

### Requirement: TDX extended ExHQ HTTP reads
The system SHALL expose TDX extended行情 (`exhq`) reads through `/api/tdx/exhq/*` routes.

#### Scenario: Fetch extended quote
- **WHEN** a client requests `/api/tdx/exhq/quote` with an extended market id and instrument code
- **THEN** the system fetches the quote using the existing TDX ExHQ implementation
- **AND** it returns the decoded quote fields without converting the instrument into an A-share market symbol

#### Scenario: Read extended market metadata and data
- **WHEN** a client requests supported `/api/tdx/exhq/*` read endpoints for markets, instrument counts, instrument lists, K-lines, minute data, transactions, or history ranges
- **THEN** the system maps the request to existing `internal/tdx` ExHQ read operations
- **AND** it keeps extended market ids and instrument codes separate from `sh` / `sz` / `bj` market naming

### Requirement: TDX provider API error model
The system SHALL expose a stable JSON error envelope for TDX provider APIs.

#### Scenario: Validation error
- **WHEN** a TDX provider API request has invalid parameters
- **THEN** the response status MUST be `400`
- **AND** the response body MUST include `{ "error": "<message>" }`

#### Scenario: Upstream TDX unavailable
- **WHEN** all configured TDX upstream servers fail or time out before a valid response is decoded
- **THEN** the response status MUST be `503`
- **AND** the response body MUST include `{ "error": "<message>" }`

#### Scenario: Protocol decode error
- **WHEN** a TDX upstream response is reachable but cannot be decoded according to the supported protocol contract
- **THEN** the response status MUST be `502`
- **AND** the response body MUST include `{ "error": "<message>" }`

### Requirement: No implicit persistence for TDX provider reads
The system SHALL keep TDX provider HTTP reads independent from ClickHouse writes unless a separate storage contract is accepted.

#### Scenario: Realtime quote HTTP request
- **WHEN** a client requests realtime quotes through `/api/tdx/hq/quotes`
- **THEN** the system returns the decoded snapshot
- **AND** it MUST NOT write quote snapshots into ClickHouse

#### Scenario: Provider historical read request
- **WHEN** a client requests online TDX K-lines, minute data, transactions, or extended history data through `/api/tdx/*`
- **THEN** the system returns the upstream read result
- **AND** it MUST NOT import those rows into canonical ClickHouse fact tables

### Requirement: TDX provider API documentation
The system SHALL document the TDX provider API boundary separately from the ClickHouse-backed query API.

#### Scenario: API documentation distinguishes namespaces
- **WHEN** an operator reads API documentation
- **THEN** the documentation identifies `/api/v1/...` as product/query APIs backed by ClickHouse
- **AND** it identifies `/api/tdx/...` as provider/protocol APIs backed by live TDX upstream requests

#### Scenario: Realtime design documentation includes HTTP boundary
- **WHEN** an operator reads the realtime quote design documentation
- **THEN** the documentation describes why TDX provider HTTP APIs are isolated from ClickHouse-backed APIs
- **AND** it records that WebSocket/SSE streaming and snapshot persistence remain out of scope for this change

