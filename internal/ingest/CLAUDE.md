# internal/ingest/

Import orchestration — wires `tdx` parsing to `clickhouse` writes.

- `import.go` — `Import(ctx, ImportOptions) (Summary, error)`: single file/code import. Parses, writes bars + quality issues, records a `TaskRun`, advances the `Watermark`.
- `bulk.go` — `ImportDailyBulk(ctx, BulkOptions) (BulkSummary, error)`: many codes under a root.
- `online_runner.go` — `RunOnlineJob[T](ctx, OnlineJob[T])`: shared runner for **explicit online-provider→ClickHouse** import jobs. Owns lifecycle + ops recording only (task run, watermark, quality issues, dry-run, failure). The product adapter owns fetch/normalize/dedup/logical-key/write-target/watermark-bounds and supplies a `Write` closure + an `OnlineOps` (satisfied by `*clickhouse.Store`). `intraday.go` (`ImportIntradayPoints`) is the first adopter.
- `g4_daily.go` — explicit post-close `g4day` package import into raw `a_share_bars_1d`; local replay and official HTTP mode share the TDX package validator and `RunOnlineJob` lifecycle.

Boundaries: this runner is write-plane only. Live provider reads (`/api/tdx/*`, `hq-minute`, …) MUST NOT invoke it; query reads (`/api/v1/*`) MUST NOT fetch upstream. Local/offline imports stay on `Import`/`ImportDailyBulk`, not the online runner.

`--dry-run` runs parse + validation without writing. This layer owns the parse→write→bookkeeping sequence; it should not contain binary decoding (that's `tdx`) or SQL (that's `clickhouse`).
