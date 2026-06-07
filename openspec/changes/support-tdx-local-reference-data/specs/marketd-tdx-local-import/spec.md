## ADDED Requirements

### Requirement: Client-local reference-data import command behavior
The system SHALL apply the existing local import safety and ops behavior to client-local reference-data imports.

#### Scenario: Reference import supports explicit client-local file input
- **WHEN** an operator runs a client-local reference-data import command for `gbbq`, block, custom block, or `ex_daily`
- **THEN** the command supports explicit `--file`
- **AND** commands that can safely discover conventional TDX paths support `--root`
- **AND** `ex_daily` imports require explicit `--market` and `--code` until a concrete extension-market path family such as `Lxxx` or `vipdoc/ds/lday` is covered by fixtures and tests

#### Scenario: Reference import reports quality issues
- **WHEN** a client-local reference-data import encounters invalid date, unsupported market, unsupported symbol, unsupported format variant, malformed text encoding, trailing bytes, duplicate logical key, conflicting logical key, or zero valid rows
- **THEN** marketd records a data quality issue with input path and record offset when available
- **AND** the task run is not reported as a clean success

#### Scenario: Reference import remains local-only
- **WHEN** an operator runs a client-local reference-data import command
- **THEN** marketd reads only local files and ClickHouse
- **AND** it MUST NOT connect to remote TDX servers

#### Scenario: Reference import dry-run
- **WHEN** an operator adds `--dry-run` to a local reference-data import command
- **THEN** marketd parses and validates local input
- **AND** it reports the target table, input format, row counts, skipped rows, and quality issues
- **AND** it does not write market tables or ops tables
