## ADDED Requirements

### Requirement: Explicit g4day import command
The system SHALL provide an explicit `marketd import-tdx-g4-day` command that imports one finalized broker daily package into canonical A-share daily facts.

#### Scenario: Download official package by date
- **WHEN** an operator runs `marketd import-tdx-g4-day --date 2026-09-04`
- **THEN** marketd downloads the official `g4day/20260904.zip` package
- **AND** it validates and imports that package as trade date `2026-09-04`

#### Scenario: Replay local package
- **WHEN** an operator runs `marketd import-tdx-g4-day --file /path/to/20260904.zip`
- **THEN** marketd reads the local ZIP without contacting the official HTTP source
- **AND** it derives the trade date from the package entries

#### Scenario: Assert local package date
- **WHEN** an operator supplies both `--file` and `--date`
- **THEN** marketd requires the requested date to equal the date encoded by every package entry
- **AND** it rejects a mismatch before writing rows

#### Scenario: Dry run
- **WHEN** the operator adds `--dry-run`
- **THEN** marketd downloads or reads, decodes, filters, and validates the complete package
- **AND** it reports the source, trade date, SHA-256, record counts, selected rows, skipped rows, and quality issue count
- **AND** it MUST NOT write market facts, task runs, watermarks, or quality issues

### Requirement: Atomic g4day package validation
The system SHALL reject an invalid `g4day` archive before writing any daily bars from it.

#### Scenario: Complete market pairs
- **WHEN** a package is parsed
- **THEN** it must contain one `.cod` and one `.md1` entry for each of `sh`, `sz`, and `bj`
- **AND** all six names must encode the same valid trade date

#### Scenario: Fixed-record alignment
- **WHEN** a market pair is parsed
- **THEN** the `.cod` length must be divisible by 150 bytes
- **AND** the `.md1` length must be divisible by 512 bytes
- **AND** the two record counts must be equal and within configured safety limits

#### Scenario: Code directory integrity
- **WHEN** code records are paired with quote records
- **THEN** each code used by the parser must be a six-byte symbol identifier
- **AND** duplicate codes within one market cause the complete package to be rejected

#### Scenario: Bounded input
- **WHEN** a remote response, local ZIP, ZIP entry, or expanded package exceeds the supported safety limit
- **THEN** marketd rejects it without extracting files or writing rows

### Requirement: g4day A-share normalization
The system SHALL map valid traded A-share `g4day` records to the existing raw daily bar model.

#### Scenario: Recognized equity families
- **WHEN** a code is Shanghai `6xxxxx`, Shenzhen `000xxx`, `001xxx`, `002xxx`, `003xxx`, `300xxx`, or `301xxx`, or current Beijing `920xxx`
- **THEN** marketd treats it as an A-share equity candidate
- **AND** all other package codes are counted as skipped without being reported as malformed

#### Scenario: Exclude Beijing indices
- **WHEN** a Beijing package contains index code `899050`, `899601`, or another non-`920xxx` code
- **THEN** marketd MUST NOT write that record as an A-share stock bar
- **AND** it counts the record as skipped

#### Scenario: Normalize a traded record
- **WHEN** an equity candidate has finite positive OHLC and amount values, positive volume, and consistent high/low relationships
- **THEN** marketd maps it to market, symbol, package trade date, open, high, low, close, volume, and amount
- **AND** it uses logical key `(market, symbol, trade_date)`
- **AND** it writes the raw row to `infinity_market.a_share_bars_1d`

#### Scenario: Skip a no-trade equity record
- **WHEN** an equity candidate has no positive traded OHLCV data for the package date
- **THEN** marketd does not create a synthetic daily bar
- **AND** it counts that record as skipped without degrading an otherwise valid package

#### Scenario: Reject corrupted eligible row
- **WHEN** an equity candidate has non-finite numeric data, partial traded values, or inconsistent high/low relationships
- **THEN** marketd rejects the complete package before writing any rows

### Requirement: g4day import lifecycle and boundaries
The system SHALL reuse existing daily fact and ops behavior without coupling realtime reads or adjusted bars to the import.

#### Scenario: Successful write
- **WHEN** a validated non-dry-run package contains daily bars
- **THEN** marketd writes all selected rows through the existing daily batch writer
- **AND** it records a `tdx_g4_daily_import` task run
- **AND** it advances the `a_share_bars_1d` watermark for asset `all` to the package trade date

#### Scenario: Failed package
- **WHEN** download, ZIP validation, or normalization fails in non-dry-run mode
- **THEN** marketd records a failed task run when the ops store is available
- **AND** it writes no daily bars from that package

#### Scenario: Raw facts only
- **WHEN** a g4day package is imported
- **THEN** it MUST NOT fetch XDXR events, refresh adjustment factors, or write derived daily metrics as a side effect

#### Scenario: Realtime reads stay read-only
- **WHEN** a client requests or subscribes to realtime quotes
- **THEN** the g4day importer MUST NOT run as a side effect
- **AND** provisional current-day quote values MUST NOT be written to `a_share_bars_1d` by this capability

#### Scenario: No implicit scheduling
- **WHEN** the command completes
- **THEN** it MUST NOT create a delayed or recurring job
