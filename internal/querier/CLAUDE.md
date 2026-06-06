# internal/querier/

HTTP **read plane**. Defines query semantics independent of storage.

- `server.go` — `Server` + routes under `/api/v1`: `GET /health`, `/bars`, `/resolve-symbol`.
- `types.go` — `Repository` interface (the read contract), DTOs (`BarQuery`, `Bar`, `BarResult`, `Health`, `SymbolResolution`), and `Version` / `SchemaVersion` constants.
- `validate.go` — `NormalizeQuery` input validation; `ValidationError` maps to HTTP 400.
- `client.go` — `HTTPClient` used by the `infinity` CLI to call this service.

The `Repository` implementation lives in `internal/clickhouse/query.go`. Add query features here (handler + interface + DTO) and there (SQL) — never put ClickHouse SQL in the CLI. Bump `SchemaVersion` when response shapes change.
