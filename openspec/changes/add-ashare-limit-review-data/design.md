## Context

Quman currently produces two compatible daily review shapes: recent THS-backed snapshots and historical rule-replay snapshots. They contain useful objective facts but differ in completeness and percentage units, live as one JSON file per day, and cannot be queried or corrected efficiently across a date range. The existing marketd write plane already owns ClickHouse bootstrap, inserts, task runs, watermarks, and quality issues; the Infinity querier is the only read path.

The detailed field inventory and migration rationale live in `docs/design/ashare-limit-review-data-layer.md`. Repository invariants prohibit provider/source/version columns in market facts and prohibit ClickHouse read SQL outside `internal/clickhouse/query.go`.

## Goals / Non-Goals

**Goals:**

- Persist objective limit-up, broken-board, limit-down, relay, breadth, performance-index, and theme data as normalized ClickHouse rows.
- Import the existing 2016+ snapshot corpus once and accept explicit correction JSONL afterward.
- Reconstruct a trading day's objective review through one read-only API.
- Reuse existing marketd and Infinity binaries, ops tables, and Go package boundaries.

**Non-Goals:**

- Preserve the old JSON file layout as a permanent API contract.
- Store blogger articles, screenshots, OCR text, confidence, or source provenance in market fact tables.
- Generate subjective narrative conclusions in marketd.
- Guess undocumented TDX 880005 fields. Online breadth refresh is enabled only for fields proven by protocol fixtures; normalized JSON import remains available meanwhile.
- Build the spreadsheet matrix UI/export in this change; the API returns the facts needed by that view.

## Decisions

### Authority-first historical integration

Material publication uses `enrich_existing`: Go loads current rows and permits only filling empty reasons and empty/legacy-unclassified primary themes. It rejects missing events, changed core fields, tags, and nonempty attribution conflicts. Unrestricted `upsert` is an explicit CLI-only operator action requiring `--allow-fact-replacement`; it is not exposed by the material HTTP operation. THS refresh retains existing attribution and missing optional fields while updating provider core values. Snapshot replay rejects occupied dates unless the same explicit replacement flag is supplied. Read dependencies are injected; CLI reads go through the existing HTTP querier. Ops params retain operation policy and provider scope; no fact metadata columns are added. Independent writers must still be serialized operationally.

All available evidence dates from 2016 are processed into local, resumable plans. Unsupported images, date conflicts, unverified names, missing events, and inaccessible sources remain explicit review work, not fabricated facts or automatic event additions. Full corpus processing and verified fact completeness are separate outcomes.

### Six purpose-specific tables

Use `a_share_limit_events`, `a_share_limit_daily_summary`, `a_share_limit_relay_events`, `a_share_limit_performance_index_bars_1d`, `a_share_market_breadth_daily`, and `a_share_limit_theme_daily`. Event rows keep stock/date facts; summary, relay, and theme tables are explicitly derived/materialized query tables. This avoids one wide nullable snapshot table and keeps common date/range queries bounded.

Canonical event rows do not store `pct_chg`; callers obtain it from `a_share_daily_derived` or daily bars. Relay rows may store `today_pct_chg` because relay is a derived cross-day table. No table stores source, version, updated-at, or mutable security names.

### One normalized snapshot parser in Go

The migration parser accepts the current quman snapshot shape and flattens persisted objective sections in one pass. Operators explicitly select `--percent-unit=percent|ratio` for retained relay percentages; numeric magnitude is never used to guess units. It normalizes Chinese relay statuses to stable enum values, infers market from the six-digit symbol, validates dates/times/enums, and resolves duplicate logical keys before writing. Legacy `平板` means sealed without board-count promotion, not a zero return; `-` seal times become null. Legacy board counts are retained, but multi-day board labels must not be interpreted as proven consecutive streaks.

Directory import walks `YYYY/MM/DD.json`, applies inclusive `--since` and `--until`, checks path/payload date agreement, and rejects malformed/conflicting rows before fact writes. Count mismatches remain warnings. A single-file path is also accepted for tests and targeted recovery.

### Corrections write final facts

Correction JSONL is not an observation/evidence ledger. Each line names a trade date, mode, operator reason/audit reference, and complete final event rows. Material writes use guarded `enrich_existing`; operator-only `upsert` requires explicit fact-replacement authorization. Unknown modes and fields are rejected. Omitted optional fields in enrichment cannot clear stored values. An explicit operator upsert can replace them and a change of event type is a new logical key, not deletion. Audit text is recorded in task-run params, not facts. Writers must serialize conflicting batches; no cross-table transaction or concurrent last-writer guarantee is implied.

### Opt-in console correction operation

The existing infinity-console write plane optionally installs `POST /api/console/imports/limit-review-corrections` when `INFINITY_REVIEW_WRITE_TOKEN` is configured. It accepts a single correction object and reuses the in-memory CLI parser. Authentication, no browser Origin, a 4 MiB body cap, 30-second execution deadline, process-local serialization, and default dry-run bound the operation. No new service or table is introduced; the ordinary querier does not install it. Storage failures retain available run metadata and require read-back verification before retry, not automatic HTTP replay. Independent writers still require operational serialization.

### Dedicated indices are separate from relay details

Software-defined yesterday-limit-up and consecutive-board performance indices are stored as OHLCV series keyed by internal semantic `index_code`. Per-stock relay rows explain composition/outcomes but are not used as the primary index series.

The online TDX adapter verifies security-directory identities before paging history: 880863 yesterday-limit-up, 880812 yesterday-consecutive-board, and 880751 yesterday-limit-down. 880864 is yesterday-oscillation and must not substitute for the ladder index. Non-ST mapping remains unverified and is rejected. Coverage gaps produce warnings, not fabricated older history. Volumes retain TDX lots and amounts use yuan.

### THS online pools share the normalized write path

`refresh-limit-review` fetches all three pools for an explicitly closed trading day. It validates response date, stable pagination, symbols, timestamps and consecutive board semantics. Multi-day/multi-board labels require a previous-trading-day pool walk; unavailable history fails the batch. The adapter writes events and a basic summary, leaving unavailable optional counts null, and reuses existing task/watermark handling. Online refresh must not blindly overwrite post-import corrections.

### TDX 880005 is a provider adapter, not a schema

Breadth storage uses semantic count columns. The future adapter must map a proven TDX 880005 response into those columns; raw provider field names/codes stay outside facts. The generic HQ multi-record decoder had a shadowed position variable; fixing it restores sequential index/security bar decoding, including a read-only 880005 probe. This does not establish the semantic mapping of breadth buckets. No online breadth writer is enabled yet. Normalized breadth JSON requires explicit up/down/total counts and preserves unknown optional counts as null.

### Read API composes existing tables

Each dataset has a focused query endpoint. `GET /api/v1/limit-review?trade_date=...` performs bounded date queries through the repository and returns summary, breadth, indices, event pools, ladder groups, relay, and themes. It does not call THS/TDX and does not read legacy files.

Basic counts for persisted summary dates are refreshed from final events on reads. Daily and bounded-range relay reconstruct previous-event groups using the saved previous trading date or an explicit single-day date, today's events, and daily-derived returns. Themes are recomputed per date from final primary themes. Missing base facts fall back to saved materializations; this is not a completeness guarantee for partial historical backfills. Ranges are limited to 93 calendar days and each input collection to 200000 rows. Summary promotion/noodle/strong-theme fields remain imported values or null. Query SQL is confined to `internal/clickhouse/query.go`.

The matrix endpoint composes these repositories into date headers and stock/date cells. Filters select stocks but retain all events/outcomes of selected stocks in the range; pagination is by stock (default 100, maximum 500). Missing cells remain unknown, dates come only from available records, and prices/charts continue to use the existing bars API. This is a data API, not a UI/export implementation.

## Risks / Trade-offs

- [ReplacingMergeTree without a version column can expose duplicate physical rows before merges] -> Resolve duplicates before insert and use `FINAL` on correction-sensitive point/date queries.
- [Historical snapshots omit intraday seal data and reasons] -> Preserve null/empty values and let later correction imports enrich final rows.
- [Historical and recent snapshots encode percentages differently] -> Do not persist event `pct_chg`; normalize retained relay percentages with fixture tests.
- [TDX 880005 record fields have unverified semantic meanings] -> Require software/document comparison and captured fixtures before enabling online writes.
- [Legacy board counts may encode N boards within M days] -> Preserve source values and require a separate verified event-chain calculation for strict consecutive-board analysis.
- [A full day reconstruction performs several small queries] -> Keep all predicates on one trade date and add a single repository method so callers make one HTTP request.

## Migration Plan

1. Deploy idempotent bootstrap DDL and the new binary/API code.
2. Dry-run recent THS snapshots and historical rule-replay snapshots; review issue counts and date coverage.
3. Build a date coverage manifest and assign each date to historical or recent snapshots. Reconcile overlapping pools before writing: inserting a newer snapshot does not remove historical events absent from that snapshot. Import corrections after base snapshots, and never blindly replay old snapshots over manual corrections.
4. Import blogger-derived correction JSONL with explicit `upsert` only after dry-run validation.
5. Compare daily event counts with summary counts and record mismatches in `infinity_ops.data_quality_issues`.
6. Switch Infinity consumers to `/api/v1/limit-review`; archive old JSON only after a parallel verification period.

Rollback is operational: stop new imports and route consumers back to the old reader. New tables are additive and are not dropped automatically.

### Blogger evidence integration pilot

Infinity's existing historical-review module now prepares candidate plans from OCR coordinates and publishes explicitly reviewed candidates through the existing console correction operation. Marketd does not interpret blogger documents or gain a Python dependency. The first bounded pilot fills missing reasons and explicit primary themes of existing limit-up events, preserving every other field. Source hashes and full before-images are checked upstream before complete-row submission, with Go dry-run and HTTP readback. This does not make concurrent read/check/write atomic; independent writers must still serialize operations. Pilot results and rejected evidence are documented in `docs/design/ashare-review-evidence-pilot-20260905.md`.

## Open Questions

- Confirm TDX 880005 semantic mapping for directional totals and `>3%`, `>5%`, and `>7%` buckets against independently verified same-day values. A successful SH index-bar probe establishes transport decoding only.
- Confirm the dedicated non-ST-limit-up OHLCV code and missing historical coverage before enabling that online series. The other three TDX identities are verified at run time.
