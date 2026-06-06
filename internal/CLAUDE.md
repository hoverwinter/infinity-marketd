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
| `querier` | HTTP query service, DTOs, `Repository` interface, HTTP client |
| `clickhouse` | connection, insert (`store.go`), schema DDL (`schema.go`), read SQL (`query.go`) |
| `model` | plain shared data structs |
| `config` | config loading + precedence |

Key invariant: ClickHouse **read** SQL lives only in `clickhouse/query.go` (implements `querier.Repository`). CLIs never open ClickHouse for reads — they call the HTTP querier.
