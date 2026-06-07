# marketd-tdx-local-import Specification

## Purpose
TBD - created by archiving change build-tdx-clickhouse-mvp. Update Purpose after archive.
## Requirements
### Requirement: Local TDX market and symbol discovery
The system SHALL infer A-share market and symbol from local TDX paths, filenames, and code fallback rules.

#### Scenario: Path-based market inference
- **WHEN** a file path contains `vipdoc/sh`, `vipdoc/sz`, or `vipdoc/bj`
- **THEN** marketd uses that path segment as the market

#### Scenario: Filename-based symbol inference
- **WHEN** a filename is `sh600519.day`, `sz000001.lc1`, or `bj920002.lc5`
- **THEN** marketd extracts the six-digit symbol after the two-letter market prefix

#### Scenario: Beijing code fallback
- **WHEN** marketd must infer market from symbol without path context
- **AND** the symbol starts with `920`, `8`, or `4`
- **THEN** marketd resolves the market to `bj`

#### Scenario: Shanghai and Shenzhen fallback
- **WHEN** marketd must infer market from symbol without path context
- **AND** the symbol starts with `6` or `9` but not `920`
- **THEN** marketd resolves the market to `sh`
- **AND** common remaining A-share symbols resolve to `sz`

### Requirement: Daily `.day` import
The system SHALL parse local TDX `.day` files and import daily OHLCV bars.

#### Scenario: Daily record decoding
- **WHEN** marketd parses a `.day` file
- **THEN** it reads 32-byte little-endian records using `<IIIIIfII`
- **AND** it decodes date, open, high, low, close, amount, volume, and reserved

#### Scenario: Daily normalization
- **WHEN** a valid `.day` record is normalized
- **THEN** prices are divided by `100.0`
- **AND** the record is mapped to `infinity_market.a_share_bars_1d`
- **AND** the logical key is `(market, symbol, trade_date)`

#### Scenario: Daily command
- **WHEN** an operator runs `marketd import-tdx-day`
- **THEN** the command supports an explicit `--file`
- **AND** it supports `--root` plus `--code`
- **AND** it supports optional `--market`, `--since`, `--until`, and `--dry-run`

### Requirement: Local 1-minute import
The system SHALL parse local TDX `.lc1` and `.1` files and import 1-minute OHLCV bars.

#### Scenario: LC1 record decoding
- **WHEN** marketd parses a `.lc1` file
- **THEN** it reads 32-byte little-endian records using `<HHfffffII`
- **AND** it decodes packed date, minute-of-day, float OHLC, amount, volume, and reserved

#### Scenario: Compatible `.1` record decoding
- **WHEN** marketd parses a `.1` file
- **THEN** it reads 32-byte little-endian records using `<HHIIIIfII`
- **AND** it decodes packed date, minute-of-day, integer-cent OHLC, amount, volume, and reserved

#### Scenario: One-minute normalization
- **WHEN** a valid 1-minute record is normalized
- **THEN** bar_time is decoded in `Asia/Shanghai` trading time
- **AND** trade_date is derived from bar_time
- **AND** `.1` prices are divided by `100.0`
- **AND** the record is mapped to `infinity_market.a_share_bars_1m`
- **AND** the logical key is `(market, symbol, bar_time)`

#### Scenario: One-minute command
- **WHEN** an operator runs `marketd import-tdx-1m`
- **THEN** the command supports an explicit `--file`
- **AND** it supports `--root` plus `--code`
- **AND** it supports optional `--market`, `--since`, `--until`, and `--dry-run`

### Requirement: Local 5-minute import
The system SHALL parse local TDX `.lc5` and `.5` files and import 5-minute OHLCV bars.

#### Scenario: LC5 record decoding
- **WHEN** marketd parses a `.lc5` file
- **THEN** it reads 32-byte little-endian records using `<HHfffffII`
- **AND** it decodes packed date, minute-of-day, float OHLC, amount, volume, and reserved

#### Scenario: Compatible `.5` record decoding
- **WHEN** marketd parses a `.5` file
- **THEN** it reads 32-byte little-endian records using `<HHIIIIfII`
- **AND** it decodes packed date, minute-of-day, integer-cent OHLC, amount, volume, and reserved

#### Scenario: Five-minute normalization
- **WHEN** a valid 5-minute record is normalized
- **THEN** bar_time is decoded in `Asia/Shanghai` trading time
- **AND** trade_date is derived from bar_time
- **AND** `.5` prices are divided by `100.0`
- **AND** the record is mapped to `infinity_market.a_share_bars_5m`
- **AND** the logical key is `(market, symbol, bar_time)`

#### Scenario: Five-minute command
- **WHEN** an operator runs `marketd import-tdx-5m`
- **THEN** the command supports an explicit `--file`
- **AND** it supports `--root` plus `--code`
- **AND** it supports optional `--market`, `--since`, `--until`, and `--dry-run`

### Requirement: Quality handling for local imports
The system SHALL record parse and import quality issues without requiring shell log access.

#### Scenario: Incomplete trailing bytes
- **WHEN** a local TDX file length is not a multiple of 32 bytes
- **THEN** marketd parses complete records
- **AND** records a quality issue for the trailing bytes

#### Scenario: Invalid record
- **WHEN** a record has invalid date, invalid time, unsupported market, unsupported code format, or `high < low`
- **THEN** marketd skips or quarantines that record according to parser rules
- **AND** records a quality issue with input path and record offset

#### Scenario: Zero valid rows
- **WHEN** an import completes parsing but produces zero valid rows
- **THEN** marketd records a quality issue
- **AND** the task run is not reported as a clean success

### Requirement: No remote dependency during local import
The system SHALL keep local TDX imports independent from remote TDX services.

#### Scenario: Local import does not connect remotely
- **WHEN** an operator runs `import-tdx-day`, `import-tdx-1m`, or `import-tdx-5m`
- **THEN** marketd reads only local files and ClickHouse
- **AND** it MUST NOT connect to remote TDX servers

### Requirement: Local minute imports do not populate intraday points
The system SHALL keep local TDX 1-minute OHLCV imports separate from persisted TDX standard HQ intraday point imports.

#### Scenario: One-minute local import writes only bars
- **WHEN** an operator runs `marketd import-tdx-1m`
- **THEN** marketd writes valid records to `infinity_market.a_share_bars_1m`
- **AND** it MUST NOT write rows to `infinity_market.a_share_intraday_points`

#### Scenario: Five-minute local import does not write intraday points
- **WHEN** an operator runs `marketd import-tdx-5m`
- **THEN** marketd writes valid records to `infinity_market.a_share_bars_5m`
- **AND** it MUST NOT write rows to `infinity_market.a_share_intraday_points`

#### Scenario: No implicit point derivation from bars
- **WHEN** local minute OHLCV bars are imported
- **THEN** marketd MUST NOT derive intraday points from bar close and volume as an import side effect

### Requirement: Local OHLCV imports remain raw-only
The system SHALL keep local TDX OHLCV imports independent from adjusted bar factor refreshes.

#### Scenario: Daily import does not refresh adjustment factors
- **WHEN** an operator runs `marketd import-tdx-day`
- **THEN** the command writes only canonical raw daily OHLCV facts and import quality metadata
- **AND** it MUST NOT fetch xdxr events
- **AND** it MUST NOT refresh qfq or hfq adjustment factors as a hidden side effect

#### Scenario: Minute imports do not refresh adjustment factors
- **WHEN** an operator runs `marketd import-tdx-1m` or `marketd import-tdx-5m`
- **THEN** the command writes only canonical raw minute OHLCV facts and import quality metadata
- **AND** it MUST NOT fetch xdxr events
- **AND** it MUST NOT refresh qfq or hfq adjustment factors as a hidden side effect

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

