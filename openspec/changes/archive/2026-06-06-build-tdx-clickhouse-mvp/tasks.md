## 1. Project Skeleton

- [x] 1.1 Create Go module and project layout for `cmd/marketd` and `internal/*`.
- [x] 1.2 Add CLI command routing for `bootstrap`, `status`, `import-tdx-day`, `import-tdx-1m`, and `import-tdx-5m`.
- [x] 1.3 Add configuration loading with CLI/env/config/default precedence.
- [x] 1.4 Add structured command errors and non-zero exits for failed operations.

## 2. ClickHouse Schema

- [x] 2.1 Add ClickHouse connection helper.
- [x] 2.2 Implement idempotent bootstrap for `infinity_market` and `infinity_ops`.
- [x] 2.3 Implement DDL for `a_share_bars_1d`, `a_share_bars_1m`, and `a_share_bars_5m`.
- [x] 2.4 Implement DDL for `watermarks`, `task_runs`, and `data_quality_issues`.
- [x] 2.5 Add tests that validate emitted DDL names, engines, partition keys, and order keys.
- [x] 2.6 Implement batch insert helpers for each market table and ops table.

## 3. TDX Discovery And Market Rules

- [x] 3.1 Implement market/code inference from file path and filename.
- [x] 3.2 Implement code fallback rules: `920*`, `8*`, `4*` -> `bj`; `6*`, `9*` -> `sh`; common remaining A-share codes -> `sz`.
- [x] 3.3 Add tests that ensure `920002` resolves to `bj`.
- [x] 3.4 Implement root+code discovery for `.day`, `.lc1/.1`, and `.lc5/.5`.

## 4. Daily Import

- [x] 4.1 Implement `.day` parser for 32-byte `<IIIIIfII` records.
- [x] 4.2 Normalize prices from cents to yuan.
- [x] 4.3 Validate dates, `high >= low`, incomplete trailing bytes, zero valid rows, and duplicate logical keys.
- [x] 4.4 Add fixture tests for valid Shanghai/Shenzhen/Beijing files and invalid records.
- [x] 4.5 Wire parser output to `a_share_bars_1d` writes.
- [x] 4.6 Record task run, watermark, and quality issue rows.

## 5. 1-Minute Import

- [x] 5.1 Implement `.lc1` parser for 32-byte `<HHfffffII` records.
- [x] 5.2 Implement `.1` parser for 32-byte `<HHIIIIfII` records.
- [x] 5.3 Decode packed date and minute-of-day into `Asia/Shanghai` `bar_time`.
- [x] 5.4 Validate time, `high >= low`, incomplete trailing bytes, zero valid rows, and duplicate logical keys.
- [x] 5.5 Add fixture tests for LC float-price format and integer-cent compatible format.
- [x] 5.6 Wire parser output to `a_share_bars_1m` writes.
- [x] 5.7 Record task run, watermark, and quality issue rows.

## 6. 5-Minute Import

- [x] 6.1 Reuse minute parser support for `.lc5` and `.5` files.
- [x] 6.2 Ensure `.lc5/.5` writes only to `a_share_bars_5m`.
- [x] 6.3 Add fixture tests for 5-minute float-price and integer-cent compatible formats.
- [x] 6.4 Wire parser output to `a_share_bars_5m` writes.
- [x] 6.5 Record task run, watermark, and quality issue rows.

## 7. CLI Operations

- [x] 7.1 Implement `marketd bootstrap`.
- [x] 7.2 Implement `marketd status`.
- [x] 7.3 Implement `marketd import-tdx-day` with `--file`, `--root`, `--code`, `--market`, `--since`, `--until`, and `--dry-run`.
- [x] 7.4 Implement `marketd import-tdx-1m` with `--file`, `--root`, `--code`, `--market`, `--since`, `--until`, and `--dry-run`.
- [x] 7.5 Implement `marketd import-tdx-5m` with `--file`, `--root`, `--code`, `--market`, `--since`, `--until`, and `--dry-run`.
- [x] 7.6 Ensure local import commands never connect to remote TDX servers.

## 8. Verification

- [x] 8.1 Run Go unit tests.
- [x] 8.2 Run bootstrap dry-run and verify DDL output.
- [x] 8.3 Run daily fixture import dry-run and verify row count, logical keys, and quality issues.
- [x] 8.4 Run 1-minute fixture import dry-run and verify timestamps are not shifted by +8 hours.
- [x] 8.5 Run 5-minute fixture import dry-run and verify rows go to `a_share_bars_5m`.
- [x] 8.6 If a development ClickHouse instance is available, run bootstrap and sample imports for one stock.
