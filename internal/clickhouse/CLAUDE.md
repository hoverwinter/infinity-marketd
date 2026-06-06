# internal/clickhouse/

All ClickHouse access for both planes.

- `store.go` — `Open` connection + insert/status helpers (`Insert*Bars`, `InsertQualityIssues`, watermarks). The write plane.
- `schema.go` — `BootstrapDDL(SchemaConfig)` builds CREATE statements; tables are `ReplacingMergeTree` keyed by logical identity.
- `query.go` — read SQL implementing `querier.Repository` (`Health`, `Bars`). Handles inclusive `since`/`until` bounds and date-only `until` covering a whole trading day.

**Non-negotiable:** never emit `DROP`/`TRUNCATE`/`DETACH` or destructive table replacement. Schema rebuilds go through a written migration plan executed manually by the operator. Do not add `source`/`version`/`updated_at` to fact tables. Authoritative schema: `docs/storage/clickhouse.md`.
