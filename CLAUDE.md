# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run CLI
go run ./cmd/marketd bootstrap --dry-run
go run ./cmd/marketd bootstrap --config configs/config.yaml
go run ./cmd/marketd status --config configs/config.yaml
go run ./cmd/marketd import-tdx-day --root ~/tdx-data --code 600519 --dry-run
go run ./cmd/marketd import-tdx-1m --root ~/tdx-data --code 600519 --dry-run
go run ./cmd/marketd import-tdx-5m --root ~/tdx-data --code 600519 --dry-run

# Tests
go test ./...
go test ./internal/tdx/... -v
go test -run TestParseDayBytes ./internal/tdx/

# Verification
openspec validate --all
```

## Non-Negotiable Rules

- Do not delete ClickHouse databases or tables. Never run `DROP DATABASE`, `DROP TABLE`, `TRUNCATE TABLE`, `DETACH TABLE`, or destructive table replacement commands from an assistant workflow.
- If a schema rebuild is needed, write a migration plan and ask the operator to execute destructive steps manually. Prefer creating a new table/database and validating data before any manual cutover.
- Do not introduce `source`, `version`, or `updated_at` columns into market fact tables. `marketd` is the fact producer and resolves input conflicts before writing facts.
- Keep canonical fact tables limited to values present in the normalized market data. Cross-row derived metrics such as `pct_chg` belong in derived tables or refresh jobs.

## Architecture

This is a Go market data daemon that parses TDX binary files and writes to ClickHouse.

### Package Structure

- `cmd/marketd/main.go` - Entry point, delegates to `internal/cli`
- `internal/cli` - CLI command routing (bootstrap, status, import-tdx-day/1m/5m)
- `internal/config` - Configuration with precedence: CLI flags > env vars > config file > defaults
- `internal/clickhouse` - ClickHouse connection (`store.go`), schema DDL (`schema.go`)
- `internal/tdx` - TDX binary parsing (`parse.go`), market detection (`market.go`), file discovery (`discovery.go`)
- `internal/model` - Data models: DailyBar, MinuteBar, QualityIssue, TaskRun, Watermark
- `internal/ingest` - Import orchestration, coordinates parsing and writing

### ClickHouse Databases

- `infinity_market` - Market data tables: `a_share_bars_1d`, `a_share_bars_1m`, `a_share_bars_5m`, `a_share_daily_derived`
- `infinity_ops` - Operational tables: `watermarks`, `task_runs`, `data_quality_issues`

All market tables use `ReplacingMergeTree` for idempotent writes by logical key (market + symbol + date/time).

See `docs/storage/clickhouse.md` for the authoritative ClickHouse schema and partitioning rationale.

### TDX Parsing

Binary files are 32 bytes per record:
- `.day` - Daily OHLCV, prices in integer cents → divide by 100
- `.lc1` - 1-minute bars, float32 prices
- `.1` - 1-minute bars, integer cent prices
- `.lc5` / `.5` - 5-minute bars

Market is inferred from file path (`vipdoc/sh/...`, `vipdoc/sz/...`, `vipdoc/bj/...`) or code prefix.

### Configuration

Config file example in `examples/config.example.yaml`. All time handling uses `Asia/Shanghai` timezone.

### Data Flow

```
TDX binary file → tdx.Parse*Bytes → model.DailyBar/MinuteBar → clickhouse.Insert*Bars → ClickHouse
                                 → ParseIssue → model.QualityIssue → clickhouse.InsertQualityIssues
```

Each import creates a TaskRun and updates Watermarks.
