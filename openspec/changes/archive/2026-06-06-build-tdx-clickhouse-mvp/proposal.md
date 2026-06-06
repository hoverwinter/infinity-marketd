## Why

`infinity-marketd` needs a small, verifiable first implementation that turns local TongDaXin A-share files into canonical ClickHouse facts.

Current Infinity market data is spread across Python loaders, Parquet, JSON caches, and business modules. That is workable for UI iteration, but it is not a stable base for full-market minute scans, repeatable backtests, and operational data quality checks.

The first implementation should not over-model future sources. `infinity-marketd` is the only fact producer for this project; ClickHouse facts should not contain source/version concepts. Input conflicts must be resolved inside marketd before facts are written.

## What Changes

- Add a Go `marketd` CLI and project skeleton in this repository.
- Add configuration for ClickHouse, TDX root, timezone, batch size, and dry-run mode.
- Bootstrap ClickHouse databases:
  - `infinity_market`
  - `infinity_ops`
- Bootstrap A-share canonical fact tables:
  - `infinity_market.a_share_bars_1d`
  - `infinity_market.a_share_bars_1m`
  - `infinity_market.a_share_bars_5m`
- Bootstrap operational tables:
  - `infinity_ops.watermarks`
  - `infinity_ops.task_runs`
  - `infinity_ops.data_quality_issues`
- Parse and import local TDX files:
  - `.day` daily bars
  - `.lc1` and `.1` 1-minute bars
  - `.lc5` and `.5` 5-minute bars
- Provide operator commands:
  - `marketd bootstrap`
  - `marketd status`
  - `marketd import-tdx-day`
  - `marketd import-tdx-1m`
  - `marketd import-tdx-5m`

## Non-Goals

- No remote TDX TCP client.
- No realtime quote polling.
- No intraday point table implementation.
- No quote snapshot table implementation.
- No Infinity gateway migration.
- No frontend changes.
- No strategy signal calculation.
- No source/version modeling in ClickHouse fact tables.
- No pandas, mootdx, or Python dependency in marketd parsing.

## Capabilities

### New Capabilities

- `marketd-clickhouse-data-plane`: schema bootstrap, canonical fact table writes, watermarks, task runs, quality issues, and status command.
- `marketd-tdx-local-import`: local TDX `.day`, `.lc1/.1`, and `.lc5/.5` parsing and import.

## Impact

- New Go code in this repository.
- New ClickHouse dependency.
- New ClickHouse databases and tables.
- New fixtures and tests for TDX binary formats.
- Existing Infinity applications remain unchanged until later integration work.
