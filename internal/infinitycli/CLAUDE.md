# internal/infinitycli/

Command routing for the **infinity** binary. `Run(ctx, args, stdout, stderr) int` dispatches the `querier` subcommands.

- `serve` — boots the ClickHouse `Store` and serves `querier.Server.Handler()` (the only command that touches ClickHouse directly).
- `health` / `bars` / `resolve-symbol` — thin clients that call the running HTTP service via `querier.HTTPClient`. They must **not** open a ClickHouse connection or build SQL.

When adding a client command, extend the querier HTTP API first, then call it from here.

`providers.go` adds `providers`, `provider-bars`, `provider-boards`, `provider-board` as thin `/api/providers` clients with an explicit source. `serve` alone reads the optional `INFINITY_THS_COOKIE` startup setting; do not expose it as a query parameter or print it.
