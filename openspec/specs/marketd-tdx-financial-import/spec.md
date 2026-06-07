# marketd-tdx-financial-import Specification

## Purpose
TBD - created by archiving change support-tdx-financial-data-import. Update Purpose after archive.
## Requirements
### Requirement: TDX financial dictionary metadata
The system SHALL provide version-controlled dictionary metadata for TDX professional financial fields and stock metric types.

#### Scenario: Load financial item dictionary
- **WHEN** marketd starts a `tdxfin` import
- **THEN** it loads the financial item dictionary from repository metadata
- **AND** each dictionary entry includes an item id, stable field name, display title, unit or value kind, and confirmation status

#### Scenario: Reject missing financial dictionary ids
- **WHEN** a parsed `tdxfin` record references an item id that is absent from the loaded dictionary
- **THEN** marketd records a data quality issue
- **AND** the import is not reported as a clean success

#### Scenario: Load stock metric dictionary
- **WHEN** marketd starts a `tdxgp` import
- **THEN** it loads the stock metric dictionary from repository metadata
- **AND** each dictionary entry includes a metric type, stable field name, display title, value meanings, and confirmation status

### Requirement: TDX professional financial package import
The system SHALL parse local TDX professional financial packages and import raw financial item facts.

#### Scenario: Import tdxfin zip
- **WHEN** an operator runs `marketd import-tdx-fin --file data/tdxfin.zip`
- **THEN** marketd reads the local ZIP file
- **AND** it discovers `gpcwYYYYMMDD.dat` or matching compressed report files inside the package
- **AND** it MUST NOT connect to remote TDX servers

#### Scenario: Decode financial report records
- **WHEN** marketd parses a valid `gpcwYYYYMMDD.dat` report file
- **THEN** it normalizes each financial value to market, symbol, report_date, item_id, and value
- **AND** it maps the records to `infinity_market.a_share_financial_raw_items`
- **AND** the logical key is `(market, symbol, report_date, item_id)`

#### Scenario: Validate tdxfin manifest
- **WHEN** a `tdxfin` package contains `gpcw.txt`
- **THEN** marketd validates available file names, sizes, and checksums against the manifest
- **AND** it records quality issues for mismatches

#### Scenario: Dry-run tdxfin import
- **WHEN** an operator runs `marketd import-tdx-fin --file data/tdxfin.zip --dry-run`
- **THEN** marketd parses and validates the package
- **AND** it reports discovered files, rows, skipped rows, and quality issues
- **AND** it does not write raw facts, dictionary tables, watermarks, or task runs

### Requirement: TDX stock metric package import
The system SHALL parse local TDX stock metric packages and import raw dated metric facts.

#### Scenario: Import tdxgp zip
- **WHEN** an operator runs `marketd import-tdx-gp --file data/tdxgp.zip`
- **THEN** marketd reads the local ZIP file
- **AND** it discovers `gp{market}{symbol}.dat` files inside the package
- **AND** it MUST NOT connect to remote TDX servers

#### Scenario: Decode stock metric records
- **WHEN** marketd parses a valid `gp{market}{symbol}.dat` file
- **THEN** it reads fixed-length metric records
- **AND** it normalizes each record to market, symbol, metric_type, event_date, value1, and value2
- **AND** it maps the records to `infinity_market.a_share_gp_metric_values`
- **AND** the logical key is `(market, symbol, metric_type, event_date)`

#### Scenario: Validate tdxgp manifest
- **WHEN** a `tdxgp` package contains `gpszsh.txt` or `gpszsh.local`
- **THEN** marketd validates available file names, sizes, and checksums against the manifest data it can interpret
- **AND** it records quality issues for mismatches

#### Scenario: Dry-run tdxgp import
- **WHEN** an operator runs `marketd import-tdx-gp --file data/tdxgp.zip --dry-run`
- **THEN** marketd parses and validates the package
- **AND** it reports discovered files, rows, skipped rows, and quality issues
- **AND** it does not write raw facts, dictionary tables, watermarks, or task runs

### Requirement: Financial import ops records
The system SHALL report TDX financial imports through existing marketd ops tables.

#### Scenario: Record successful financial import
- **WHEN** a financial package import writes rows
- **THEN** marketd records a task run in `infinity_ops.task_runs`
- **AND** it records watermarks in `infinity_ops.watermarks`
- **AND** it records data quality issues in `infinity_ops.data_quality_issues` when applicable

#### Scenario: No derived wide table side effects
- **WHEN** a financial package import completes
- **THEN** marketd writes only raw financial facts, dictionary lookup rows, and ops records
- **AND** it MUST NOT create or refresh financial derived wide tables

### Requirement: Trading remains out of scope
The system SHALL keep broker trading capability outside `marketd`.

#### Scenario: Financial import does not add trading integration
- **WHEN** marketd adds TDX financial package import support
- **THEN** it MUST NOT wrap `pytdx.trade` or `trade.dll`
- **AND** it MUST NOT add order submission, account query, position, fill, broker login, or transaction command surfaces

