# docs/api/

Querier HTTP API reference for `infinity querier serve` (`/api/v1`: `health`, `bars`, `resolve-symbol`). JSON responses; errors as `{"error": "..."}`.

Keep `README.md` in sync with `internal/querier` (routes, DTOs, `Version`/`SchemaVersion`). Callers depend on this contract, not on ClickHouse table shapes.
