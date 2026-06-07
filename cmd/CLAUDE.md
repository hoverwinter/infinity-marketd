# cmd/

Binary entry points (`package main`). Each subdirectory builds one executable and stays thin: parse `os.Args`, call into an `internal/*` `Run` function, exit with its return code. No business logic lives here.

- `marketd/` — ingest/write-plane binary (TDX → ClickHouse).
- `infinity/` — querier/read-plane binary (HTTP service + client CLI).
- `infinity-console/` — standalone operator console binary (Go API + built Vite assets).

Build all binaries: `make build` (`go build ./cmd/infinity ./cmd/marketd ./cmd/infinity-console`).
