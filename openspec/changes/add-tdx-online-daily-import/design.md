## Context

`marketd` already has three relevant pieces:

- `tdx.FetchHQSecurityBars` and the `hq-bars` CLI can read standard HQ K-line rows from TDX servers.
- `clickhouse.Store.InsertDailyBars` writes canonical raw daily OHLCV facts to `a_share_bars_1d`.
- `ingest.RunOnlineJob` centralizes lifecycle recording for explicit online-provider-to-ClickHouse imports.

The missing piece is a product adapter that turns online HQ daily K-line pages into `model.DailyBar` rows and invokes the runner. Existing live provider commands and `/api/tdx/*` endpoints must remain read-only.

## Goals / Non-Goals

**Goals:**

- Add a single-symbol online daily import command for A-share securities.
- Add a Console action that immediately triggers the same single-symbol import.
- Persist raw daily OHLCV rows into the existing `a_share_bars_1d` table.
- Support bounded repair workflows with `--since` and `--until`.
- Keep provider reads, stable query APIs, and adjustment refreshes explicit and separate.
- Reuse existing TDX decoding, ClickHouse insert, best-IP/server selection, and online runner infrastructure.

**Non-Goals:**

- No automatic full-market scheduler in this change.
- No scheduled/cron import execution in this change.
- No online 1m/5m OHLCV import yet.
- No index, fund, or ExHQ daily import in this command.
- No new ClickHouse tables.
- No precomputed adjusted OHLCV tables.
- No implicit XDXR or adjustment factor refresh during daily import.

## Decisions

### 1. Add `import-tdx-hq-day`, not write behavior to `hq-bars`

`hq-bars` remains a diagnostic/provider read command that does not need ClickHouse. The new command is explicitly write-plane and can require ClickHouse config when `--dry-run` is false.

Alternative considered: add `--write` to `hq-bars`. Rejected because it mixes read-only provider tooling with persistent import semantics and weakens the current `/api/tdx/*` boundary.

### 2. Use `RunOnlineJob` with a daily-bar adapter

Create an `ingest.ImportHQDailyBars` adapter that owns fetch paging, date filtering, logical-key handling, quality issues, target table selection, and watermark bounds. The shared runner records lifecycle metadata and performs dry-run/write behavior.

Alternative considered: copy local `Import` orchestration. Rejected because online imports already have a dedicated runner and should share task/watermark behavior with `import-tdx-intraday-points`.

### 3. Normalize online rows into canonical raw daily bars

The adapter maps `tdx.HQBar` to `model.DailyBar` using:

```text
market, symbol, trade_date, open, high, low, close, volume, amount
```

Only raw OHLCV goes into `a_share_bars_1d`. The import does not persist provider category, source server, version, or updated timestamp in the fact table.

Alternative considered: add source metadata columns to daily bars. Rejected because project rules explicitly keep market fact tables to normalized market values only.

### 4. Page by provider window and filter by date bounds

TDX standard HQ K-line pages are limited to 800 rows. The adapter fetches `category=HQKLineDayAlt`, starting from the requested `--start` and advancing by page size until one of these conditions is reached:

- requested `--count` rows have been fetched before date filtering;
- a page returns fewer than the page size;
- date filtering proves no older requested rows can be included;
- a defensive maximum page count is reached.

Rows are then filtered by `--since` and `--until` before writing. `--start` and `--count` remain provider-window controls; `--since` and `--until` are import correctness controls.

Alternative considered: ignore `start/count` and fetch until the date range is satisfied. Rejected for the first version because it can create unexpectedly large upstream reads. Operators can explicitly widen `--count` when they need deeper repair.

### 5. Treat duplicate/conflicting provider rows like import quality issues

The logical key is `(market, symbol, trade_date)`. Identical duplicates are skipped with a quality issue; conflicting duplicates are skipped with a higher-severity quality issue. The command must never fabricate or average OHLCV values.

Alternative considered: rely only on ClickHouse `ReplacingMergeTree` to deduplicate. Rejected because conflicting provider rows should be visible to operators before storage merges hide the import shape.

### 6. Keep adjustment refresh explicit

After backfilling raw daily bars, operators run:

```text
marketd refresh-tdx-xdxr --market sh --symbol 600519
marketd refresh-adjust-factors --market sh --symbol 600519
```

The online daily import may print a short reminder when rows are written, but it must not call those refreshes automatically.

Alternative considered: refresh factors automatically after every daily import. Rejected because factor refresh has its own data requirements and quality issues; tying it to every bar import makes failures harder to reason about.

### 7. Add a Console operator POST endpoint for immediate imports only

Expose `POST /api/console/imports/tdx-hq-day` as an operator action that validates query parameters, invokes the same `ingest.ImportHQDailyBars` adapter, and returns the import summary as JSON. This route belongs to `/api/console`, not `/api/v1` or `/api/tdx/*`, because it is an explicit operational write.

The first version runs one bounded import request and returns when that import finishes. This keeps the implementation small and matches the single-symbol repair workflow. A future scheduler/job queue can wrap the same importer for delayed or recurring work.

Alternative considered: create a generic scheduled job system now. Rejected for this change because scheduling needs durable pending/running/canceled state and would be reusable infrastructure for XDXR, factor refresh, derived metrics, and future minute imports.

## Risks / Trade-offs

- [TDX server retention varies] -> Document that `--count`/`--start` control how much provider history is requested and return zero rows as a degraded import, not fabricated data.
- [Provider ordering assumptions] -> Normalize by decoded `trade_date`, sort before writing if needed, and compute watermarks from actual rows.
- [Silent stale adjusted queries] -> Document the explicit XDXR/factor refresh step after raw daily backfills.
- [Large repair runs create many small inserts] -> First version is single-symbol only; full-market/bulk online repair can be designed later with batching and concurrency limits.
- [Current day may be incomplete before market close] -> The command imports whatever the provider returns. Operators should use date bounds for historical repair and avoid treating in-session current-day bars as final.
- [Long web request during large imports] -> Keep Console imports single-symbol and bounded by `count` in this version; durable background scheduling remains a separate capability.

## Migration Plan

No schema migration is required. Deploying the command is non-destructive because it writes the existing canonical daily table through the existing `ReplacingMergeTree` logical key.

Rollback is operational: stop running `import-tdx-hq-day`. Existing raw bars remain in `a_share_bars_1d`; if an operator imported unwanted rows, correction should follow the project’s non-destructive migration policy rather than table truncation or replacement.

## Open Questions

- Should `--until` default to the previous Shanghai trading day instead of allowing current-day import by default?
- Should the command offer `--refresh-adjust-factors` later as an explicit opt-in wrapper, or keep refresh orchestration outside this command permanently?
- Should a later scheduler use a new ops table or reuse task runs with a pending status?
