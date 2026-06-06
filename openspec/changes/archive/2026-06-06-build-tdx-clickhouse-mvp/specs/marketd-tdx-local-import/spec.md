## ADDED Requirements

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
