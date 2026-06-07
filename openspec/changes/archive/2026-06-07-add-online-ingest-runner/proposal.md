## Why

`import-tdx-intraday-points` proved that online TDX provider data can become a persisted ClickHouse data product, but its orchestration is currently product-specific. Before adding more online-to-ClickHouse products, marketd should have a small shared runner for the repeated path: fetch online data, normalize it, deduplicate it, write facts, and record ops metadata.

## What Changes

- Add a thin online ingest runner inside `internal/ingest`.
- Move the intraday point import workflow onto that runner as the first implementation.
- Keep product-specific semantics outside the runner:
  - source fetch function;
  - normalization;
  - logical key;
  - target model/table/write function;
  - dataset-specific validation.
- Reuse existing `TaskRun`, `Watermark`, and `QualityIssue` tables.
- Preserve existing boundaries:
  - `/api/tdx/*` remains live read-only provider access;
  - `/api/v1/*` remains ClickHouse-backed query access;
  - local/offline imports remain separate from online provider imports.

## Capabilities

### New Capabilities
- `marketd-online-ingest-runner`: Shared runner for explicit online-provider-to-ClickHouse import jobs.

### Modified Capabilities
None.

## Impact

- `internal/ingest`: add a small online ingest runner and migrate intraday import orchestration onto it.
- `internal/cli`: keep `marketd import-tdx-intraday-points` behavior stable while routing through the migrated import path.
- `internal/clickhouse`: no schema change expected; runner uses existing Store write methods.
- `openspec/changes/add-a-share-intraday-points`: implementation remains semantically valid; this change only factors shared orchestration.
- Tests: add runner-level unit tests and intraday regression tests to prove behavior is unchanged.
