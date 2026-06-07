# infinity-marketd

`infinity-marketd` is a Go market data daemon for importing, normalizing, and writing structured market data into ClickHouse.

The first supported input format is local TongDaXin data:

- `.day` daily bars
- `.lc1` / `.1` 1-minute bars
- `.lc5` / `.5` 5-minute bars

It provides ClickHouse schema bootstrap, idempotent batch writes, watermarks, task run records, and data quality diagnostics.

## Commands

Market data import commands:

```bash
go run ./cmd/marketd bootstrap --dry-run
go run ./cmd/marketd bootstrap --config examples/config.example.yaml

go run ./cmd/marketd status --config examples/config.example.yaml

go run ./cmd/marketd import-tdx-day --root ~/tdx-data --code 600519 --dry-run
go run ./cmd/marketd import-tdx-1m --root ~/tdx-data --code 600519 --dry-run
go run ./cmd/marketd import-tdx-5m --root ~/tdx-data --code 600519 --dry-run
```

On-demand TDX quote commands:

```bash
go run ./cmd/marketd quote --symbol sh:600519 --server 180.153.18.170:7709
go run ./cmd/marketd quote-probe --server 180.153.18.170:7709
go run ./cmd/marketd quote-bestip
go run ./cmd/marketd quote --symbol sh:600519 --bestip
go run ./cmd/marketd quote-sweep --market sh --limit 10 --server 180.153.18.170:7709

go run ./cmd/marketd exquote-markets --server 61.152.107.141:7727
go run ./cmd/marketd exquote --market 47 --code IF1709 --server 61.152.107.141:7727
```

Query service and CLI commands:

```bash
go run ./cmd/infinity querier serve --config examples/config.example.yaml --listen 127.0.0.1:8808

go run ./cmd/infinity querier health --url http://127.0.0.1:8808
go run ./cmd/infinity querier bars --url http://127.0.0.1:8808 --market sh --symbol 600519 --period 1d --since 2024-01-01 --until 2024-12-31
go run ./cmd/infinity querier bars --url http://127.0.0.1:8808 --market sh --symbol 600519 --period 1d --adjust qfq --since 2024-01-01 --until 2024-12-31
go run ./cmd/infinity querier bars --url http://127.0.0.1:8808 --market sh --symbol 600519 --period 1m --since "2026-01-01 09:30:00" --until "2026-01-01 15:00:00"
```

`infinity querier bars` calls the querier HTTP service under `/api/v1`. The CLI does not duplicate ClickHouse SQL logic. `--since` and `--until` are inclusive; for minute bars, a date-only `--until 2026-01-01` includes the whole trading date. `--adjust qfq|hfq` uses precomputed daily adjustment factors and adjusts OHLC only.

Console commands:

```bash
make console-install
make console-dev
make console-build
go run ./cmd/infinity-console --config examples/config.example.yaml \
  --listen 127.0.0.1:8809 --console-dist web/console/dist
```

See `docs/console.md` for the Node.js + Vite workflow, production serving, and safety boundary.

Explicit file imports are also supported:

```bash
go run ./cmd/marketd import-tdx-day --file ~/tdx-data/vipdoc/sh/lday/sh600519.day
go run ./cmd/marketd import-tdx-1m --file ~/tdx-data/vipdoc/sh/minline/sh600519.lc1
go run ./cmd/marketd import-tdx-5m --file ~/tdx-data/vipdoc/sh/fzline/sh600519.lc5
```

## ClickHouse Tables

Market fact tables:

- `infinity_market.a_share_bars_1d`
- `infinity_market.a_share_bars_1m`
- `infinity_market.a_share_bars_5m`
- `infinity_market.a_share_xdxr_events`
- `infinity_market.a_share_adjust_factors_1d`

Operational tables:

- `infinity_ops.watermarks`
- `infinity_ops.task_runs`
- `infinity_ops.data_quality_issues`

Fact tables do not model source or version. `marketd` resolves input conflicts before writing canonical market facts.

## Configuration

Configuration precedence:

```text
CLI flags > environment variables > config file > defaults
```

Use `examples/config.example.yaml` as a starting point. Do not commit real credentials.

## Verification

```bash
go test ./...
openspec validate --all
```
