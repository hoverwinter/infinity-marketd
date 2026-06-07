## ADDED Requirements

### Requirement: Online ingest runner boundary
The system SHALL provide a shared runner for explicit online-provider-to-ClickHouse import jobs.

#### Scenario: Runner is not a live provider API
- **WHEN** an HTTP client calls `/api/tdx/*`
- **THEN** the online ingest runner MUST NOT run as a side effect
- **AND** the provider API remains a live read-only path

#### Scenario: Runner is not a query API
- **WHEN** an HTTP client calls `/api/v1/*`
- **THEN** the online ingest runner MUST NOT fetch upstream TDX data
- **AND** `/api/v1/*` continues to read ClickHouse or stable local state only

#### Scenario: Runner stays in the write plane
- **WHEN** an online provider import command persists data
- **THEN** it uses write-plane code under `internal/ingest` and `internal/clickhouse`
- **AND** it MUST NOT add ClickHouse write behavior to `internal/querier`

### Requirement: Online ingest runner lifecycle
The system SHALL centralize common online import lifecycle behavior without owning product-specific data semantics.

#### Scenario: Successful online import
- **WHEN** a product adapter returns normalized rows and no fatal error
- **THEN** the runner writes rows through the adapter-provided Store write function
- **AND** it records task run metadata
- **AND** it records watermark metadata when rows provide bounds
- **AND** it records any adapter-provided quality issues

#### Scenario: Dry run
- **WHEN** an online import runs in dry-run mode
- **THEN** the runner returns row counts and quality issue counts
- **AND** it MUST NOT write market fact rows
- **AND** it MUST NOT write task runs, watermarks, or quality issues

#### Scenario: Failed online import
- **WHEN** fetch, normalize, or write returns a fatal error
- **THEN** the runner records a failed task run when a Store is available
- **AND** it returns the failure to the caller

#### Scenario: Product-owned params
- **WHEN** the runner records a task run
- **THEN** task params come from the product adapter
- **AND** the runner MUST NOT infer product-specific parameter schemas

### Requirement: Product adapter ownership
The system SHALL keep online data product semantics in product-specific adapters.

#### Scenario: Adapter owns normalization
- **WHEN** an online import product receives provider payloads
- **THEN** the product adapter normalizes payloads into that product's model type
- **AND** the runner does not depend on provider packet structs

#### Scenario: Adapter owns logical key handling
- **WHEN** an online import product detects duplicate provider rows
- **THEN** the product adapter applies that product's logical key and conflict rules
- **AND** it passes skipped row counts and quality issues to the runner

#### Scenario: Adapter owns write target
- **WHEN** an online import product is executed through the runner
- **THEN** the product adapter supplies the target table name and Store write function
- **AND** the runner does not choose a ClickHouse table from provider type alone

### Requirement: Intraday import runner adoption
The system SHALL migrate persisted TDX intraday point import onto the online ingest runner without changing its external behavior.

#### Scenario: Intraday command remains stable
- **WHEN** an operator runs `marketd import-tdx-intraday-points`
- **THEN** existing flags for market, symbol, date, since, until, today, server selection, config, and dry-run continue to work
- **AND** the command still writes only `infinity_market.a_share_intraday_points` when not dry-run

#### Scenario: Intraday query remains stable
- **WHEN** an HTTP client calls `/api/v1/intraday-points`
- **THEN** the response contract remains unchanged by the runner refactor

#### Scenario: Intraday provider reads remain read-only
- **WHEN** an operator or HTTP client calls `hq-minute`, `hq-history-minute`, or `/api/tdx/hq/minute`
- **THEN** those read paths MUST NOT invoke the online ingest runner
- **AND** they MUST NOT write `a_share_intraday_points`
