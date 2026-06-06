# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Per-directory `CLAUDE.md` files carry the detail; this file is the map and the rules. Read the `CLAUDE.md` in the directory you're working in.

## Commands

```bash
# Ingest (marketd, write plane)
go run ./cmd/marketd bootstrap --config configs/config.yaml
go run ./cmd/marketd status --config configs/config.yaml
go run ./cmd/marketd import-tdx-day --root ~/tdx-data --code 600519 --dry-run   # also import-tdx-1m / -5m
go run ./cmd/marketd quote --symbol sh:600519 --server 180.153.18.170:7709
go run ./cmd/marketd exquote-count --server 112.74.214.43:7727
go run ./cmd/marketd exquote-instruments --start 0 --count 20 --server 47.102.108.214:7727
go run ./cmd/marketd exquote --market 47 --code ICL0 --server 47.102.108.214:7727
go run ./cmd/marketd exquote-bars --market 47 --code ICL0 --category 4 --start 0 --count 100 --server 47.102.108.214:7727

# Query service + client (infinity, read plane)
make serve                 # go run ./cmd/infinity querier serve (CONFIG/LISTEN overridable)
make querier-health
go run ./cmd/infinity querier bars --url http://127.0.0.1:8808 \
  --market sh --symbol 600519 --period 1d --since 2024-01-01 --until 2024-12-31

# Build / test / verify
make build                 # go build ./cmd/infinity ./cmd/marketd
make test                  # go test ./...
go test -run TestParseDayBytes ./internal/tdx/   # single test
openspec validate --all
```

## Non-Negotiable Rules

- Never run destructive ClickHouse commands from an assistant workflow: no `DROP DATABASE`, `DROP TABLE`, `TRUNCATE TABLE`, `DETACH TABLE`, or destructive table replacement. Schema rebuilds get a written migration plan executed manually by the operator (prefer new table → validate → manual cutover).
- Do not add `source`, `version`, or `updated_at` columns to market fact tables. `marketd` resolves input conflicts before writing facts.
- Keep canonical fact tables to normalized market values only. Cross-row derived metrics (e.g. `pct_chg`) belong in derived tables/refresh jobs.

## Architecture

Go market data daemon. Two binaries share `internal/clickhouse`:

- **`marketd`** (write plane): TDX binary files → `internal/cli` → `internal/ingest` → `internal/tdx` parse + `internal/model` → `internal/clickhouse` insert. Each import records a `TaskRun` and advances a `Watermark`.
- **`infinity`** (read plane): client/HTTP → `internal/infinitycli` → `internal/querier` (`/api/v1`) → `internal/clickhouse/query.go`.

**Key invariant:** ClickHouse *read* SQL lives only in `internal/clickhouse/query.go` (implementing `querier.Repository`). CLIs never open ClickHouse for reads — they call the HTTP querier. To add a query feature: extend the querier handler + `Repository` interface + the SQL impl; never add ad-hoc reads to the CLI.

Storage: `infinity_market` (fact bars `a_share_bars_1d/1m/5m`, `a_share_daily_derived`) and `infinity_ops` (`watermarks`, `task_runs`, `data_quality_issues`); all `ReplacingMergeTree` keyed by logical identity. All time handling is `Asia/Shanghai`.

Authoritative references: schema → `docs/storage/clickhouse.md`; HTTP API → `docs/api/README.md`; TDX formats → `docs/tdx-data/`; specs/changes → `openspec/`.
