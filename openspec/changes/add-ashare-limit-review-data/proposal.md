## Why

Daily A-share limit-up review data currently lives in local JSON snapshots and Python workflows, which makes historical correction, range analysis, and reconstruction of a trading-day review expensive and inconsistent. Marketd needs to own the normalized objective facts in ClickHouse so Infinity can query one durable data contract from 2016 onward.

## What Changes

- Bootstrap canonical tables for limit events, daily summaries, next-day relay details, performance-index bars, market breadth, and daily theme aggregates.
- Add Go import commands for existing review snapshots and correction JSONL, with dry-run validation, deterministic conflict resolution, task-run audit, and quality issues.
- Add refresh boundaries for provider-backed limit review, dedicated performance indices, and TDX 880005 breadth data without storing provider metadata in market fact tables.
- Add ClickHouse-backed `/api/v1` queries for events, summaries, relay details, themes, performance indices, breadth, and one-day reconstructed reviews.
- Document the final schema and operational migration path; historical JSON remains migration input rather than a compatibility contract.
- Add opt-in authenticated correction imports to the existing console write operations, keeping `/api/v1` read-only and reusing the CLI validator.

## Capabilities

### New Capabilities

- `ashare-limit-review-storage`: Canonical ClickHouse storage and marketd ingestion/correction behavior for objective A-share daily review facts.
- `ashare-limit-review-query-api`: Read-only Infinity querier APIs that expose normalized review facts and reconstruct a daily review.

### Modified Capabilities

- `marketd-clickhouse-data-plane`: Bootstrap and operational metadata requirements expand to include A-share limit-review datasets.

## Impact

- Affected code: `cmd/marketd`, `internal/cli`, `internal/model`, `internal/ingest`, `internal/clickhouse`, `internal/querier`, and `cmd/infinity` routing.
- Affected storage: new tables in `infinity_market`; existing `infinity_ops.task_runs`, `watermarks`, and `data_quality_issues` are reused.
- Affected APIs: new read-only endpoints under `/api/v1`; existing endpoints remain unchanged.
- Dependencies: no Python runtime or new service is introduced; provider-specific mappings remain Go configuration/import logic.
