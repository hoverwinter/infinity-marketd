## Why

`marketd` can already read TDX standard HQ daily K-line rows online through `hq-bars`, but operators cannot use that provider to repair gaps in `infinity_market.a_share_bars_1d`. This change adds an explicit, auditable online import path for missing A-share daily OHLCV facts without changing the read-only semantics of `/api/tdx/*`.

## What Changes

- Add a `marketd import-tdx-hq-day` command that fetches standard HQ security daily bars online and writes normalized raw daily bars to `a_share_bars_1d`.
- Add a Console operator action that triggers the same online daily import immediately from the web UI.
- Support single-symbol imports with `--market`, `--symbol`, optional `--since`, `--until`, `--start`, `--count`, server selection, best-IP options, and `--dry-run`.
- Page TDX online K-line reads in batches of at most 800 rows and truncate results to requested date bounds.
- Record task run, watermark, skipped row count, and quality issues through the existing online ingest runner.
- Keep `hq-bars`, `/api/tdx/hq/bars`, and `/api/v1/bars` unchanged: provider reads remain read-only, and stable queries remain ClickHouse-backed.
- Do not fetch or refresh XDXR events or adjustment factors as a hidden side effect. Operators refresh factors explicitly after raw daily backfill.

## Capabilities

### New Capabilities

- `marketd-tdx-online-daily-import`: Explicit online TDX HQ daily K-line import into canonical A-share raw daily bars.

### Modified Capabilities

- None.

## Impact

- Affected code:
  - `internal/cli`: new command parsing and output summary.
  - `internal/ingest`: new online daily bar import adapter built on `RunOnlineJob`.
  - `internal/querier` / Console routes: new explicit `/api/console/imports/tdx-hq-day` operator action.
  - `web/console`: new sync form that submits immediate online daily imports and displays the returned summary.
  - `internal/tdx`: reuse existing HQ K-line request/decoder; add only small helpers if needed.
  - `internal/clickhouse`: reuse existing `InsertDailyBars` and ops writes.
- Affected storage:
  - Writes existing `infinity_market.a_share_bars_1d`.
  - Writes existing `infinity_ops.task_runs`, `watermarks`, and `data_quality_issues`.
  - No new tables.
- Affected docs/tests:
  - Document CLI and Console operator workflows and explain that adjusted bars require explicit `refresh-tdx-xdxr` and `refresh-adjust-factors`.
  - Add deterministic unit tests with fake fetchers; live server access remains optional/manual.
