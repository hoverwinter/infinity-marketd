# internal/ingest/

Import orchestration — wires `tdx` parsing to `clickhouse` writes.

- `import.go` — `Import(ctx, ImportOptions) (Summary, error)`: single file/code import. Parses, writes bars + quality issues, records a `TaskRun`, advances the `Watermark`.
- `bulk.go` — `ImportDailyBulk(ctx, BulkOptions) (BulkSummary, error)`: many codes under a root.

`--dry-run` runs parse + validation without writing. This layer owns the parse→write→bookkeeping sequence; it should not contain binary decoding (that's `tdx`) or SQL (that's `clickhouse`).
