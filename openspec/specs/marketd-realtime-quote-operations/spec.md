# marketd-realtime-quote-operations Specification

## Purpose
TBD - created by archiving change improve-tdx-realtime-quotes. Update Purpose after archive.
## Requirements
### Requirement: TDX HQ server probing
The system SHALL allow operators to probe candidate TDX standard行情 servers and rank reachable servers by observed response behavior.

#### Scenario: Probe reachable server
- **WHEN** the operator probes a reachable TDX HQ server
- **THEN** the system reports the server address, success status, and measured latency

#### Scenario: Probe unreachable server
- **WHEN** the operator probes an unreachable TDX HQ server
- **THEN** the system reports the server address, failure status, and error reason without aborting probes for other candidates

#### Scenario: Select best server
- **WHEN** multiple candidate servers are probed
- **THEN** the system identifies the fastest successful server as the preferred quote server

### Requirement: Quote retry across server candidates
The system SHALL retry realtime quote requests across configured TDX HQ server candidates when the selected server fails before a valid quote response is decoded.

#### Scenario: Fallback after timeout
- **WHEN** the first candidate server times out
- **THEN** the system attempts the quote request against the next configured candidate server

#### Scenario: Stop after successful quote
- **WHEN** a candidate server returns a valid quote response
- **THEN** the system returns the decoded quotes and does not contact remaining candidates

### Requirement: Batch quote connection reuse
The system SHALL support explicit batch quote workflows that reuse an established TDX HQ connection for multiple quote request batches.

#### Scenario: Reuse connection for batches
- **WHEN** a batch quote job fetches multiple batches from the same server
- **THEN** the system performs the setup handshake once and reuses the connection for subsequent batches

#### Scenario: Bound batch size
- **WHEN** a batch quote job contains more symbols than the configured batch size
- **THEN** the system splits the job into multiple quote requests

### Requirement: Online A-share symbol discovery
The system SHALL support online security-list discovery from TDX standard行情 servers for full-market quote workflows.

#### Scenario: Discover market symbols
- **WHEN** the operator requests online symbols for `sh` or `sz`
- **THEN** the system returns discovered six-digit symbols for that market

#### Scenario: Use discovered symbols for quote sweep
- **WHEN** a full-market quote workflow is requested
- **THEN** the system uses online symbol discovery or an explicit symbol list before fetching quotes

### Requirement: Explicit market coverage boundaries
The system SHALL keep Beijing market and `exhq` extended-market quote support behind explicit capabilities and validation.

#### Scenario: Beijing market remains explicit
- **WHEN** Beijing realtime quote support is not implemented
- **THEN** the system rejects `bj` realtime quote requests with a clear unsupported-market error

#### Scenario: Extended market remains separate
- **WHEN** extended-market quote support is implemented
- **THEN** it uses a separate protocol path from standard A-share `hq` quote requests

### Requirement: Quote timestamp semantics
The system SHALL expose realtime quote time semantics clearly.

#### Scenario: Server time without date
- **WHEN** the TDX quote payload only contains intraday server time
- **THEN** the system labels the value as server intraday time rather than a full timestamp

#### Scenario: Full timestamp when trade date is known
- **WHEN** the system can determine the quote trade date under `Asia/Shanghai`
- **THEN** it exposes a full timestamp or separate trade date without changing the raw intraday server time meaning

### Requirement: Snapshot storage decision
The system SHALL require an accepted storage contract before realtime quote snapshots are written to ClickHouse.

#### Scenario: No implicit persistence
- **WHEN** an operator runs realtime quote retrieval without snapshot storage enabled
- **THEN** the system returns quotes without writing to ClickHouse

#### Scenario: Storage proposal required
- **WHEN** realtime quote snapshot persistence is proposed
- **THEN** the proposal defines table schema, logical key, partitioning, retention, and deduplication behavior before implementation

