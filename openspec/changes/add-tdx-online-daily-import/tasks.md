## 1. Ingest Adapter

- [x] 1.1 Add `ImportHQDailyBars` options and summary types under `internal/ingest`.
- [x] 1.2 Implement provider paging with `tdx.FetchHQSecurityBars`, `HQKLineDayAlt`, max-800 page size, `--start`, and `--count`.
- [x] 1.3 Normalize `tdx.HQBar` rows into `model.DailyBar` with Asia/Shanghai trade dates.
- [x] 1.4 Apply inclusive `--since` and `--until` date filtering and count filtered rows as skipped.
- [x] 1.5 Detect invalid rows, identical duplicates, and conflicting duplicates using `(market, symbol, trade_date)` and emit quality issues.
- [x] 1.6 Execute the adapter through `RunOnlineJob` with target table `a_share_bars_1d`, task type `tdx_hq_daily_import`, and `Store.InsertDailyBars`.

## 2. CLI

- [x] 2.1 Add `marketd import-tdx-hq-day` routing and help output.
- [x] 2.2 Parse flags for `--market`, `--symbol`, `--since`, `--until`, `--start`, `--count`, `--server`, best-IP options, config, and `--dry-run`.
- [x] 2.3 Reuse existing HQ server configuration and best-IP option wiring.
- [x] 2.4 Print an import summary consistent with existing import commands, including dataset, target table, row counts, skipped rows, quality issue count, and dry-run mode.

## 3. Console Immediate Trigger

- [x] 3.1 Add a `/api/console/imports/tdx-hq-day` POST route that validates request parameters and calls `ImportHQDailyBars`.
- [x] 3.2 Return the online daily import summary as JSON and map validation errors to HTTP 400.
- [x] 3.3 Add a Sync view form for market, symbol, bounds, provider window, servers, and dry-run, then submit the Console POST action.
- [x] 3.4 Refresh console summary/watermark/task data after a successful import action.

## 4. Tests

- [x] 4.1 Add ingest unit tests with fake fetchers for successful single-page import, dry-run, bounds filtering, paging, empty results, invalid rows, and duplicate/conflicting rows.
- [x] 4.2 Add CLI tests for valid dry-run invocation, missing required flags, invalid date bounds, and server-option wiring.
- [x] 4.3 Add Console API tests for successful dry-run, invalid request handling, and server-option wiring.
- [x] 4.4 Add a test proving `hq-bars` and `/api/tdx/*` provider reads remain read-only and do not invoke the online import path.
- [x] 4.5 Run targeted package tests, console type-check, and `openspec validate --all`.

## 5. Documentation

- [x] 5.1 Document `import-tdx-hq-day` and the Console immediate trigger in TDX data/source documentation as explicit online-provider-to-ClickHouse imports.
- [x] 5.2 Document that online daily import writes raw bars only and that operators refresh XDXR/factors explicitly after raw daily backfill.
- [x] 5.3 Update command lists and examples in repository guidance where import commands are enumerated.
