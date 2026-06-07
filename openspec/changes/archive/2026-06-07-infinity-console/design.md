## Context

`infinity-marketd` currently has two operator surfaces:

- `marketd` CLI for write-plane tasks, TDX probes, realtime quote sweeps, and quote service operations.
- `infinity querier serve` HTTP API for `/api/v1/*` market reads and `/api/tdx/*` live provider reads.

This is enough for scripted operation, but it makes routine status checks hard to scan. Operators need a browser view for health, watermarks, recent runs, data quality issues, realtime quote state, and bestip status. The console should have a standalone `infinity-console` binary while reusing existing Go APIs and ops tables; it should not become a ClickHouse admin UI.

## Goals / Non-Goals

**Goals:**

- Add a Node.js + Vite frontend development workflow for the console.
- Add a standalone `infinity-console` binary that serves the built console and console API.
- Keep optional console asset mounting available from `infinity querier serve` for deployments that prefer one read-plane process.
- Provide read-only operational views for health, watermarks, task runs, data quality issues, quote service runs, and bestip cache.
- Provide limited non-destructive actions for TDX provider smoke checks and bestip refresh.
- Keep all ClickHouse reads for console data behind typed Go repository methods, not ad-hoc frontend SQL.

**Non-Goals:**

- No destructive ClickHouse operations.
- No market fact editing.
- No replacement for CLI import commands.
- No user/account/permission system in the first version.
- No separate long-running Node.js backend in production.

## Decisions

### Use Vite for frontend source and dev server

Place console source under `web/console` with a local `package.json`, Vite config, TypeScript, and application code. Development uses:

```bash
cd web/console
npm install
npm run dev
```

The Vite dev server proxies `/api/*` to `infinity querier serve`, so frontend development does not require CORS changes.

Alternative considered: serve raw HTML from Go templates. That would avoid Node dependencies but would produce a weaker frontend workflow and does not satisfy the explicit Node.js + Vite requirement.

### Start production console through a standalone Go binary

Add `cmd/infinity-console`, backed by `internal/consolecli`, as the primary production entry point. The binary opens the same ClickHouse-backed querier repository, registers the same `/api/v1/*`, `/api/tdx/*`, and `/api/console/*` routes, serves the configured Vite build output at `/console/`, and redirects `/` to `/console/`.

Default command:

```bash
infinity-console --config configs/config.yaml --console-dist web/console/dist
```

Alternative considered: make operators start `infinity querier serve --console-dist`. That minimizes code but makes the console feel like an incidental CLI flag instead of a first-class product surface.

### Keep optional asset serving in the querier process

Add a `--console-dist` flag to `infinity querier serve`. When set, the server mounts the Vite build output at `/console/` and supports SPA fallback to `index.html`.

Default behavior remains API-only if no console dist path is configured. This keeps existing deployments unchanged.

This path is a compatibility option, not the primary operator workflow.

### Add console API endpoints under `/api/console/*`

Use `/api/console/*` for operator dashboard data that is not part of the market data API contract:

- `GET /api/console/summary`
- `GET /api/console/watermarks`
- `GET /api/console/task-runs`
- `GET /api/console/data-quality-issues`
- `GET /api/console/quote-service/runs`
- `GET /api/console/bestip`
- `POST /api/console/bestip/refresh`

Existing `/api/v1/*` and `/api/tdx/*` endpoints remain the provider and market data APIs. The console may call those existing endpoints for smoke checks, but console-specific aggregation should stay under `/api/console/*`.

Alternative considered: make the frontend assemble everything directly from low-level endpoints. That would leak operational composition into the browser and duplicate status logic.

### Extend the repository interface for ops reads

Add typed console repository methods for ops-plane reads. The ClickHouse implementation belongs in `internal/clickhouse/query.go` or another read-only ClickHouse query file, consistent with the existing invariant that querier read SQL lives in `internal/clickhouse`.

Do not reuse `marketd status` CLI internals as the console backend. The console API should be HTTP-native and testable with handler tests.

### Keep actions narrow and non-destructive

First-version console actions are limited to:

- TDX HQ probe / quote smoke checks through existing provider APIs.
- Refreshing and reading the HQ bestip cache.

Imports, bootstrap, table rebuilds, and data deletes stay out of scope.

## Risks / Trade-offs

- Node dependency in a Go repo -> keep it isolated under `web/console` and do not require Node for `go test`.
- Static asset deployment can be missed -> add clear `make console-build` and `--console-dist` docs.
- Console API may grow into a broad backend -> keep endpoints tied to specific operator workflows and specs.
- No auth in first version -> document that the console is intended for local/private operator networks only.
- Bestip refresh performs outbound network calls -> keep refresh explicit and report errors without hiding fallback behavior.

## Migration Plan

1. Add console API DTOs and handler tests.
2. Add read-only repository methods and ClickHouse queries for ops status.
3. Add bestip cache status and refresh endpoints.
4. Add `web/console` Vite app and development proxy.
5. Add static serving support and standalone `infinity-console`.
6. Add build/docs targets.

Rollback is simple: stop `infinity-console`, or run `infinity querier serve` without `--console-dist`; API behavior remains unchanged.

## Open Questions

- Should the first deployed console be local-only, or should a later change add authentication?
- Should production builds embed static assets into the Go binary, or is `--console-dist` sufficient for current deployment?
- Which task-run filters are most useful first: dataset, task type, status, or recent time window?
