## ADDED Requirements

### Requirement: Authority-first material writes
Material corrections SHALL fill only missing attribution on existing events, with current-row validation enforced by Go before any fact write.

#### Scenario: Existing fact protection
- **WHEN** enrichment changes event membership, price fields, board count, times, tags, or a nonempty attribution
- **THEN** the entire payload is rejected without fact writes
- **AND** absent current-event reads fail closed

#### Scenario: Provider and replay protection
- **WHEN** THS refresh updates an existing event
- **THEN** existing attribution and unavailable optional values are preserved
- **AND** an offline snapshot cannot overwrite an occupied date without explicit operator replacement

### Requirement: Canonical limit event storage
The system SHALL bootstrap `infinity_market.a_share_limit_events` for one objective stock event per trade date, event type, market, and symbol.

#### Scenario: Event table contract
- **WHEN** `marketd bootstrap` completes
- **THEN** the event table stores trade date, market, symbol, event type, close status, board count, reason, themes, seal times, open count, seal amount, traded amount, turnover rate, and market value
- **AND** it MUST NOT store provider/source, version, updated-at, mutable security name, or cross-row percentage-change columns

#### Scenario: Supported event types
- **WHEN** a normalized snapshot is imported
- **THEN** event type is one of `limit_up`, `open_limit`, or `limit_down`
- **AND** close status is one of `sealed`, `broken`, `broken_reseal`, or `limit_down`

### Requirement: Daily review aggregate storage
The system SHALL bootstrap daily summary, relay, performance-index, market-breadth, and theme aggregate tables with logical keys documented in the storage schema.

#### Scenario: Aggregate tables exist
- **WHEN** `marketd bootstrap` completes
- **THEN** `a_share_limit_daily_summary`, `a_share_limit_relay_events`, `a_share_limit_performance_index_bars_1d`, `a_share_market_breadth_daily`, and `a_share_limit_theme_daily` exist
- **AND** all tables use `ReplacingMergeTree` and date-based partitions where applicable

#### Scenario: Performance indices remain provider-independent
- **WHEN** a performance-index bar is stored
- **THEN** it is keyed by semantic index code and trade date
- **AND** the fact row MUST NOT contain a provider code or provider name

### Requirement: Legacy review snapshot import
The system SHALL import the existing quman daily review JSON shape through a Go marketd command without requiring Python at runtime.

#### Scenario: Directory migration
- **WHEN** an operator runs `marketd import-limit-review-json` with a root directory and inclusive date bounds
- **THEN** matching `YYYY/MM/DD.json` snapshots are parsed, normalized, deduplicated, and written to the review tables
- **AND** the command records task runs, watermarks, and data-quality issues through existing ops tables

#### Scenario: Dry run
- **WHEN** the import command uses `--dry-run`
- **THEN** it performs discovery, parsing, normalization, duplicate detection, and validation
- **AND** it writes no market or ops rows

#### Scenario: Mixed historical percentage units
- **WHEN** a historical snapshot contains a fractional percentage and a recent snapshot contains a percentage-point value
- **THEN** retained relay percentage values are normalized to percentage points according to the operator's explicit `--percent-unit=ratio|percent`
- **AND** the importer does not infer units from numeric magnitude
- **AND** event rows do not persist the input percentage field

#### Scenario: Verified legacy placeholder values
- **WHEN** an operator explicitly selects `historical-replay` or `ths` as the snapshot kind
- **THEN** known zero placeholders from that legacy writer are normalized to null while nonzero corrections are retained
- **AND** original snapshot warnings are preserved as operational quality issues
- **AND** generic imports do not infer a profile from values or paths

#### Scenario: Production migration verification
- **WHEN** a frozen one-snapshot-per-date corpus is migrated
- **THEN** inputs and batch results are retained and all normalized event fields can be compared with HTTP FINAL reads
- **AND** successful migration does not claim that incomplete historical source data became complete

### Requirement: Final-fact correction import
The system SHALL accept line-delimited correction payloads that upsert final normalized event facts.

#### Scenario: Correction dry run
- **WHEN** an operator runs `marketd import-limit-review-corrections --dry-run`
- **THEN** every line and event is validated and normalized
- **AND** reason and audit reference are reported as operational metadata rather than fact columns

#### Scenario: Correction upsert
- **WHEN** a valid correction line uses mode `upsert` and the operator explicitly enables fact replacement
- **THEN** its normalized events are inserted using the event logical key
- **AND** re-running the same payload does not create distinct logical facts

#### Scenario: Unsupported partial correction mode
- **WHEN** a correction line requests a mode other than `enrich_existing` or explicitly authorized `upsert`
- **THEN** the importer rejects the line with a validation error rather than guessing missing values

### Requirement: Opt-in authenticated HTTP corrections
The existing console write plane SHALL accept a single final-event correction object only when a write token is explicitly configured, reusing the CLI validation and ingestion path.

#### Scenario: Read-only default
- **WHEN** the ordinary querier is used or no console write token is configured
- **THEN** the correction write route is not registered
- **AND** `/api/v1` remains read-only

#### Scenario: Authenticated preview and execution
- **WHEN** an authenticated server-side caller submits a valid correction object
- **THEN** the operation defaults to dry-run without market or ops writes
- **AND** only an explicit `dry_run=false` performs the final-row upsert and returns available task metadata

#### Scenario: Bounded serialized execution
- **WHEN** a request has an invalid token, browser Origin, unsupported media type, excessive body, malformed contract or conflicting concurrent execution
- **THEN** the request is rejected without starting fact writes
- **AND** accepted imports have an execution deadline and reuse the existing no-partial-patch contract

### Requirement: Provider-backed breadth safety
The system SHALL write TDX 880005 breadth data only after the response layout and field meanings pass fixture-backed validation.

#### Scenario: Unknown 880005 layout
- **WHEN** the provider response does not match the proven 880005 layout or produces impossible dates/counts
- **THEN** marketd writes no breadth fact rows
- **AND** it reports a data-quality issue or command error

#### Scenario: Normalized breadth import
- **WHEN** an operator supplies valid normalized breadth JSON
- **THEN** marketd can populate the same semantic breadth table independently of provider packet decoding
- **AND** missing optional counts remain null, while up/down/total counts must be explicitly supplied

### Requirement: Validated online review refresh
The system SHALL fetch THS event pools through Go for an explicitly closed trading date and validate complete input before writing facts.

#### Scenario: Multi-page pools and ambiguous board labels
- **WHEN** the provider returns multiple pages or a multi-day/multi-board label
- **THEN** the adapter reads every page and resolves the actual consecutive suffix using previous trading-day pools
- **AND** wrong dates, incomplete pagination, duplicate symbols or unprovable suffixes fail before fact writes

#### Scenario: Unavailable summary statistics
- **WHEN** online pools do not provide noodle, high-level-break or strong-theme counts
- **THEN** their stored values are null instead of zero

### Requirement: Verified dedicated index import
The system SHALL verify TDX directory identities before importing the three supported dedicated index histories.

#### Scenario: Supported index and bounded history
- **WHEN** a supported semantic index matches its expected TDX code and name
- **THEN** marketd fetches bounded pages, validates dates and OHLCV, and stores the requested range
- **AND** incomplete earlier coverage is reported explicitly without fabricated rows

#### Scenario: Unverified non-ST or changed identity
- **WHEN** a requested index has no verified mapping or its directory name differs
- **THEN** marketd rejects the import rather than substituting another index
