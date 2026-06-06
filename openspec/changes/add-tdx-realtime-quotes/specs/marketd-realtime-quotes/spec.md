## ADDED Requirements

### Requirement: Fetch realtime A-share quote snapshots
The system SHALL fetch realtime quote snapshots for one or more A-share symbols from a TDX standard行情 server.

#### Scenario: Fetch multiple symbols
- **WHEN** the operator requests realtime quotes for `sh:600519` and `sz:000001`
- **THEN** the system returns one quote snapshot for each requested symbol

#### Scenario: Reject unsupported market
- **WHEN** the operator requests a realtime quote for market `bj`
- **THEN** the system rejects the request with a validation error

### Requirement: Decode standard quote fields
The system SHALL decode known TDX standard quote fields into typed quote data.

#### Scenario: Decode price and depth fields
- **WHEN** a valid TDX standard quote response contains current price, previous close, open, high, low, amount, volume, and five bid/ask levels
- **THEN** the returned quote includes those fields as numeric values with prices converted from integer cents to decimal prices

#### Scenario: Decode server time
- **WHEN** a valid TDX standard quote response contains a quote server time field
- **THEN** the returned quote includes the decoded server time string

### Requirement: Provide quote CLI output
The system SHALL expose realtime quote retrieval through a CLI command that emits JSON.

#### Scenario: Quote command emits JSON
- **WHEN** the operator runs `marketd quote --symbol sh:600519 --symbol 000001`
- **THEN** the command writes a JSON array of quote snapshots to stdout

#### Scenario: Server override
- **WHEN** the operator runs the quote command with `--server 127.0.0.1:7709`
- **THEN** the command uses that TDX server address for the quote request

### Requirement: Keep realtime quotes out of canonical fact tables
The system SHALL NOT write realtime quote snapshots to canonical ClickHouse market fact tables as part of quote retrieval.

#### Scenario: Quote command does not open ClickHouse
- **WHEN** the operator runs the quote command
- **THEN** the command fetches quotes directly from the TDX server without requiring a ClickHouse connection
