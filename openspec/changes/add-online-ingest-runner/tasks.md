## 1. Runner Core

- [x] 1.1 Add online ingest runner types under `internal/ingest` for job metadata, result counts, watermark bounds, quality issues, and dry-run mode.
- [x] 1.2 Add runner execution logic for Store-backed writes, task run recording, watermark recording, quality issue insertion, and failed task run recording.
- [x] 1.3 Keep the runner generic over row type and Store write function without depending on TDX provider payload structs.
- [x] 1.4 Add unit tests for successful write, dry-run no-write, empty-row success, quality issue insertion, watermark recording, and failure recording.

## 2. Intraday Migration

- [x] 2.1 Refactor `ImportIntradayPoints` to use the online ingest runner for lifecycle and ops metadata.
- [x] 2.2 Keep intraday date mode parsing, TDX fetch calls, normalization, duplicate handling, and logical key handling product-owned.
- [x] 2.3 Preserve existing `marketd import-tdx-intraday-points` flags, summary output, target table, and dry-run behavior.
- [x] 2.4 Preserve `a_share_intraday_points` schema and `/api/v1/intraday-points` query behavior.

## 3. Boundary Regression Tests

- [x] 3.1 Add regression tests proving `hq-minute`, `hq-history-minute`, and `/api/tdx/hq/minute` do not invoke the online ingest runner or write ClickHouse rows.
- [x] 3.2 Add regression tests for intraday single-date, range, today, empty response, duplicate identical point, and duplicate conflicting point behavior after migration.
- [x] 3.3 Add tests proving `/api/v1/intraday-points` remains ClickHouse-backed and does not fetch upstream provider data.

## 4. Documentation And Validation

- [x] 4.1 Document the online ingest runner boundary in the relevant design/storage or TDX reference docs.
- [x] 4.2 Run `gofmt` on touched Go files.
- [x] 4.3 Run focused Go tests for `internal/ingest`, `internal/cli`, `internal/querier`, and `internal/clickhouse`.
- [x] 4.4 Run `go test ./...`.
- [x] 4.5 Run `openspec validate --all`.
