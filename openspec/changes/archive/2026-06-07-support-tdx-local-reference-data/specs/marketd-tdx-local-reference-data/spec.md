## ADDED Requirements

### Requirement: Client-local GBBQ capital-change import
The system SHALL parse client-local TDX `gbbq` files and import A-share capital-change and corporate-action events.

#### Scenario: Decode GBBQ events
- **WHEN** marketd parses a supported client-local `gbbq` file
- **THEN** it decodes each valid event to market, symbol, event_date, category, event_seq, event_name, and known numeric event fields
- **AND** it normalizes text fields to UTF-8
- **AND** it records unsupported event categories or malformed records as data quality issues

#### Scenario: Import GBBQ events
- **WHEN** an operator runs `marketd import-tdx-gbbq --file <path>`
- **THEN** marketd writes valid events to `infinity_market.a_share_capital_change_events`
- **AND** the logical key is `(market, symbol, event_date, category, event_seq)`
- **AND** it records task runs, watermarks, row counts, skipped rows, and quality issues through existing ops tables

#### Scenario: Dry-run GBBQ import
- **WHEN** an operator runs `marketd import-tdx-gbbq --file <path> --dry-run`
- **THEN** marketd parses and validates the file
- **AND** it reports rows, skipped rows, date range, and quality issues
- **AND** it does not write market rows, watermarks, task runs, or quality issue rows

### Requirement: Client-local TDX block import
The system SHALL parse client-local TDX vendor/system block files and import block definitions and memberships as deterministic snapshots.

#### Scenario: Decode system block file
- **WHEN** marketd parses a supported client-local system block file such as `block.dat`, `block_zs.dat`, `block_fg.dat`, or `block_gn.dat`
- **THEN** it decodes block names, block type, display order, member order, member code, inferred market, and inferred symbol
- **AND** it decodes GBK or GB18030 text fields to UTF-8

#### Scenario: Import system block snapshot
- **WHEN** an operator runs `marketd import-tdx-block --file <path> --scope system`
- **THEN** marketd computes a deterministic `snapshot_id` from normalized block content
- **AND** it writes one row to `infinity_market.tdx_block_snapshots`
- **AND** it writes block rows to `infinity_market.tdx_block_definitions`
- **AND** it writes member rows to `infinity_market.tdx_block_memberships`

#### Scenario: Preserve membership removals through snapshots
- **WHEN** a later block import omits a member that existed in an older snapshot
- **THEN** marketd creates or reuses a new snapshot for the later normalized content
- **AND** it MUST NOT delete or destructively replace old snapshot rows
- **AND** consumers can distinguish old and new memberships by `snapshot_id`

### Requirement: Client-local custom block import
The system SHALL parse client-local TDX user custom block files and import them as custom block snapshots.

#### Scenario: Decode custom block file
- **WHEN** marketd parses a supported client-local custom block file
- **THEN** it decodes custom block identifiers, custom block names, member order, member code, inferred market, and inferred symbol
- **AND** it records unsupported custom block variants as data quality issues instead of guessing the format

#### Scenario: Import custom block snapshot
- **WHEN** an operator runs `marketd import-tdx-block --file <path> --scope custom`
- **THEN** marketd computes a deterministic `snapshot_id` from normalized custom block content
- **AND** it writes snapshot, definition, and membership rows using `block_scope = 'custom'`
- **AND** it records task runs, watermarks, row counts, skipped rows, and quality issues through existing ops tables

### Requirement: Guarded custom block write
The system SHALL modify client-local TDX custom block files only through explicit guarded write commands.

#### Scenario: Dry-run custom block write
- **WHEN** an operator runs `marketd write-tdx-custom-block --file <path> --block-id <id> --add <symbol> --dry-run`
- **THEN** marketd parses and validates the existing custom block file
- **AND** it prints the planned normalized block content
- **AND** it does not modify the local file

#### Scenario: Atomic custom block write
- **WHEN** an operator runs `marketd write-tdx-custom-block --file <path> --block-id <id> --add <symbol>`
- **THEN** marketd validates the existing file before writing
- **AND** it writes a backup of the original file
- **AND** it writes the new content to a temporary file
- **AND** it atomically replaces the target file
- **AND** it re-reads the final file and verifies the normalized result

#### Scenario: Reject unsafe custom block write
- **WHEN** the target custom block file cannot be parsed, the requested symbol is unsupported, backup creation fails, or post-write validation fails
- **THEN** marketd fails the command
- **AND** it MUST NOT leave a partially written target file as the successful result

### Requirement: Client-local extension-market daily import
The system SHALL parse client-local TDX extension-market daily files and import extension-market OHLCV bars separately from A-share bars.

#### Scenario: Decode ex_daily bars
- **WHEN** marketd parses a supported client-local `ex_daily` file
- **THEN** it decodes each valid daily record to ex_market, code, trade_date, open, high, low, close, position, trade, price, amount, and settlement_price when those fields are present
- **AND** it records invalid dates, invalid prices, trailing bytes, and unsupported format variants as quality issues

#### Scenario: Import ex_daily bars
- **WHEN** an operator runs `marketd import-tdx-ex-daily --file <path> --market <id> --code <code>`
- **THEN** marketd writes valid bars to `infinity_market.tdx_ex_bars_1d`
- **AND** the logical key is `(ex_market, code, trade_date)`
- **AND** it MUST NOT write extension-market rows to `infinity_market.a_share_bars_1d`
- **AND** it MUST NOT assume the input file lives under A-share `vipdoc/sh`, `vipdoc/sz`, or `vipdoc/bj` directories

#### Scenario: Dry-run ex_daily import
- **WHEN** an operator runs `marketd import-tdx-ex-daily --file <path> --market <id> --code <code> --dry-run`
- **THEN** marketd parses and validates the file
- **AND** it reports rows, skipped rows, date range, and quality issues
- **AND** it does not write market rows, watermarks, task runs, or quality issue rows

### Requirement: Source classes remain separate
The system SHALL keep client-local imports, offline-package imports, and online-provider reads separate.

#### Scenario: Client-local reference import does not use online provider reads
- **WHEN** an operator runs a client-local `gbbq`, block, custom block, or `ex_daily` import command
- **THEN** marketd reads only local files and ClickHouse
- **AND** it MUST NOT connect to remote TDX HQ or ExHQ servers

#### Scenario: Provider APIs do not persist reference data
- **WHEN** an operator calls `hq-xdxr`, `hq-block`, `exquote-bars`, or the matching `/api/tdx/*` endpoints
- **THEN** marketd returns provider data as JSON
- **AND** it MUST NOT write rows to the local reference-data tables

#### Scenario: Offline packages remain outside client-local imports
- **WHEN** an operator provides a downloaded package such as `hsjday.zip`, `tdxfin.zip`, or `tdxgp.zip`
- **THEN** marketd MUST route it through an explicit offline-package importer
- **AND** it MUST NOT treat the package as a client-local TDX directory reader
