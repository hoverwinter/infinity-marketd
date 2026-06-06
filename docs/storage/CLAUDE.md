# docs/storage/

Authoritative ClickHouse storage documentation. `clickhouse.md` is the source of truth for the schema, `ReplacingMergeTree` keys, and partitioning rationale.

Update this alongside any change to `internal/clickhouse/schema.go`. Fact tables exclude `source`/`version`/`updated_at` and cross-row derived metrics by design.
