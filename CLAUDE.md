# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Per-directory `CLAUDE.md` files carry the detail; this file is the map and the rules. Read the `CLAUDE.md` in the directory you're working in.

## 思维方式 (Working Principles)

- **第一性原理 (First principles):** 从真实的数据格式、协议、约束出发推理,不要套用类比或"最佳实践"模板。在下结论前**先读代码/schema 确认系统实际怎么跑**,而不是凭印象推测。
- **奥卡姆剃刀 (Occam's razor):** 选能解决问题的最小方案。除非现有路径确实扛不住,否则**不新增 binary、package、表、依赖或抽象层**。优先复用 `Store`、现有表、现有命令。
- **苏格拉底式提问 (Socratic questioning):** 动手前先问"我们到底在解决什么问题?这需求从哪来?它是真问题吗?"。主动暴露并质疑隐含假设(**包括用户的和 spec 里的**),而不是照单执行。一个写在文档里的 open question,可能本身就是伪命题。
- **费曼技巧 (Feynman):** 能用大白话讲清楚的设计才算想明白了;讲不简单,通常说明它要么是错的、要么过度设计了。描述时**点到具体的 file/function/表**,不要用含糊的"某某层""某某服务"糊弄。
- **不要过度设计 (No over-engineering):** 别把一个内置能力做成架构工程。不要凭空发明"服务""框架""抽象层",也不要纠结现有代码已经回答了的问题。**方案的体量要匹配问题的体量。** 拿不准时,先看现有代码是怎么做同类事情的,照着做。

## Commands

```bash
# Ingest (marketd, write plane)
go run ./cmd/marketd bootstrap --config configs/config.yaml
go run ./cmd/marketd status --config configs/config.yaml
go run ./cmd/marketd import-tdx-day --root ~/tdx-data --code 600519 --dry-run   # also import-tdx-1m / -5m
go run ./cmd/marketd quote --symbol sh:600519 --server 180.153.18.170:7709
go run ./cmd/marketd exquote-count --server 47.112.95.207:7720
go run ./cmd/marketd exquote-instruments --start 0 --count 20 --server 47.112.95.207:7720
go run ./cmd/marketd exquote --market 47 --code TSL8 --server 47.112.95.207:7720
go run ./cmd/marketd exquote-bars --market 47 --code TSL8 --category 4 --start 0 --count 100 --server 47.112.95.207:7720

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
