# internal/

All application code (private to this module). Two planes share `internal/clickhouse`:

**Write plane** (marketd): `cli` → `ingest` → `tdx` (parse) + `model` + `clickhouse` (insert).

**Read plane** (infinity): `infinitycli` → `querier` (HTTP) → `clickhouse/query.go`.

| Package | Role |
|---------|------|
| `cli` | marketd command routing |
| `infinitycli` | infinity command routing |
| `ingest` | import orchestration (parse → write, TaskRun, Watermark) |
| `tdx` | TDX binary parsing, market inference, file discovery |
| `marketdata` | online data product DTOs, optional provider capabilities, validation and registry; no source/storage dependencies |
| `ths` | THS HTTP transport, current board catalogs/resolution and annual index daily history |
| `eastmoney` | Eastmoney HTTP transport, fully paginated board catalogs/resolution and board index daily history |
| `quotesvc` | long-running realtime quote service: connection pools, rate limiting, sweep scheduling, retry/resume, durable ops progress |
| `querier` | HTTP query service, DTOs, `Repository` interface, HTTP client |
| `clickhouse` | connection, insert (`store.go`), schema DDL (`schema.go`), read SQL (`query.go`) |
| `model` | plain shared data structs |
| `config` | config loading + precedence |

Key invariant: ClickHouse **read** SQL lives only in `clickhouse/query.go` (implements `querier.Repository`). CLIs never open ClickHouse for reads — they call the HTTP querier.

Online common products: `querier` → `marketdata` capability → `ths`, `eastmoney` or `tdx.MarketDataProvider`. All three are registered by default. Source adapters implement contracts without importing querier/ingest/storage. Existing TDX wire APIs remain independent compatibility endpoints. Extension guide: `docs/api/providers.md`.
