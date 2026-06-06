# internal/model/

Plain data structs shared across packages — no behavior, no dependencies on other `internal` packages.

Types: `DailyBar`, `MinuteBar`, `QualityIssue`, `TaskRun`, `Watermark`.

These are the canonical shapes produced by `ingest`/`tdx` and consumed by `clickhouse`. Keep fact-bar structs limited to normalized market values; derived metrics (e.g. `pct_chg`) belong in derived tables, not here.
