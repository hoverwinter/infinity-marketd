## Context

The repository now has three market data paths:

```text
local/client files        -> internal/ingest -> ClickHouse -> /api/v1
offline packages          -> internal/ingest -> ClickHouse -> /api/v1
online provider live read -> /api/tdx/*     -> JSON only
```

`import-tdx-intraday-points` adds a fourth shape:

```text
online provider -> explicit marketd import -> ClickHouse -> /api/v1
```

That shape is likely to repeat for quote snapshots, online K-line backfill, XDXR refresh, block refresh, and other provider-backed data products. The first implementation should extract only the common orchestration already proven by intraday points.

## Goals / Non-Goals

**Goals:**

- Add a reusable runner for explicit online-provider-to-ClickHouse import jobs.
- Move intraday point import onto the runner without changing the command contract or table schema.
- Keep data product semantics product-owned: normalization, logical key, target model, and write function stay outside the generic runner.
- Preserve ops behavior through existing task runs, watermarks, and quality issues.
- Keep live provider APIs read-only.

**Non-Goals:**

- Do not create a daemon, scheduler, queue, plugin framework, or new binary.
- Do not make a generic persistence switch for every `/api/tdx/*` endpoint.
- Do not merge local/offline imports into the online runner in this change.
- Do not change `a_share_intraday_points` schema or `/api/v1/intraday-points`.
- Do not add new ClickHouse tables.

## Decisions

### Add A Thin Runner Under `internal/ingest`

Create a small runner shape in `internal/ingest`, for example:

```text
OnlineJob
  dataset
  target_table
  asset
  params
  dry_run
  fetch/produce rows
  write rows
  quality issues
  watermark bounds
```

The runner owns:

- run ID and summary lifecycle;
- dry-run behavior;
- common task run recording;
- common quality issue insertion;
- common watermark insertion;
- failure recording.

Rationale: these behaviors are repeated across online imports and already exist in the intraday implementation. Keeping the runner in `internal/ingest` matches the current write-plane boundary.

Alternative considered: create `internal/bridge` or `internal/online`. That adds a new architectural noun without a stronger boundary than `ingest`.

### Use Product Adapters

Each data product supplies a narrow adapter:

```text
intraday adapter
  fetch: FetchHQMinuteTime / FetchHQHistoryMinuteTime
  normalize: HQMinutePoint -> model.IntradayPoint
  key: market + symbol + trade_date + point_time
  write: Store.InsertIntradayPoints
  watermark: min/max point_time
```

The runner must not know TDX packet types, `HQMinutePoint`, or intraday table columns.

Rationale: this prevents a weak generic framework that leaks every product's special cases.

### Keep Date Iteration Product-Owned For Now

The intraday importer currently has date modes:

```text
--date
--since/--until
--today
```

This change can keep date iteration in the intraday adapter and use the runner for the per-job lifecycle. If a second online product needs the same date loop, extract that loop then.

Rationale: not every online import is date-shaped. Quote snapshots and block refreshes have different windows and identities.

### Preserve External Contracts

`marketd import-tdx-intraday-points` must keep its existing flags and output fields. Existing `/api/tdx/*` routes remain live read-only; existing `/api/v1/intraday-points` remains ClickHouse-backed.

Rationale: this change is a refactor plus small abstraction, not a product contract change.

## Risks / Trade-offs

- Runner becomes too generic -> Limit it to lifecycle and ops recording; keep fetch/normalize/write in adapters.
- Intraday behavior changes during migration -> Add regression tests for date, range, today, dry-run, empty response, duplicate handling, task run, watermark, and quality issue behavior.
- Future products do not fit the first runner -> Prefer adding one or two narrow extension points later instead of designing for unknown products now.
- Ops metadata loses product-specific params -> Runner accepts explicit params JSON from adapters and does not invent param schemas.

## Migration Plan

1. Add runner types and unit tests.
2. Refactor intraday import to call the runner for summary, write, task run, watermark, quality issue, and failure behavior.
3. Keep intraday fetch, normalize, duplicate handling, and date mode parsing product-owned.
4. Run focused tests for `internal/ingest` and `internal/cli`.
5. Run `go test ./...` and `openspec validate --all`.

Rollback is a code refactor rollback only. No schema migration or data migration is involved.

## Open Questions

- Should a future second adopter extract a shared date-range iterator, or should each online product own its own range model?
- Should runner summaries eventually use one common JSON shape for all `marketd import-*` commands, or keep command-specific text summaries?
