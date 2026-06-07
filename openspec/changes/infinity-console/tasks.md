## 1. Console API Contracts

- [x] 1.1 Add console DTOs for summary, watermarks, task runs, data quality issues, quote service runs, bestip status, and smoke-check responses.
- [x] 1.2 Add console repository interface methods for bounded read-only ops-plane queries.
- [x] 1.3 Add request validation for console API limits, symbols, server candidates, and bestip refresh parameters.

## 2. ClickHouse Read Implementation

- [x] 2.1 Implement read-only ClickHouse queries for recent watermarks, task runs, and data quality issues.
- [x] 2.2 Implement read-only ClickHouse queries for recent quote service runs.
- [x] 2.3 Add ClickHouse query tests for ordering, bounded limits, and empty-result behavior.

## 3. Console HTTP Handlers

- [x] 3.1 Register `/api/console/summary`, `/api/console/watermarks`, `/api/console/task-runs`, and `/api/console/data-quality-issues`.
- [x] 3.2 Register `/api/console/quote-service/runs`, TDX HQ probe smoke-check, and quote smoke-check endpoints.
- [x] 3.3 Register `/api/console/bestip` and `/api/console/bestip/refresh`.
- [x] 3.4 Add handler tests for success, validation errors, upstream errors, and non-destructive behavior.

## 4. Static Console Serving

- [x] 4.1 Add standalone `cmd/infinity-console` binary and `internal/consolecli` server entry point.
- [x] 4.2 Add `--console-dist` support to `infinity querier serve`.
- [x] 4.3 Mount configured assets at `/console/` with SPA fallback to `index.html` and `/` redirect.
- [x] 4.4 Add tests proving API routes still win over console static routes.

## 5. Vite Frontend Setup

- [x] 5.1 Create `web/console` with Node.js package metadata, Vite config, TypeScript config, and dev proxy for `/api/*`.
- [x] 5.2 Add frontend build scripts for `dev`, `build`, `test` or `check`, and `preview`.
- [x] 5.3 Add repository-level Make targets for console development and production build.

## 6. Console UI

- [x] 6.1 Build the console shell with navigation for Overview, Ops, Realtime, BestIP, and TDX Smoke views.
- [x] 6.2 Build Overview cards/tables for health, schema version, watermarks, task runs, quality issue counts, and quote service status.
- [x] 6.3 Build Ops views for watermarks, task runs, and data quality issues with bounded limit controls.
- [x] 6.4 Build Realtime and BestIP views including quote service runs, cache status, probe results, and explicit refresh.
- [x] 6.5 Build TDX smoke-check views for HQ probe and quote checks without fact-table writes.
- [x] 6.6 Add loading, empty, error, and stale-data states for each view.

## 7. Documentation And Validation

- [x] 7.1 Document Node.js version expectations, install, dev server, API proxy, production build, and `--console-dist` usage.
- [x] 7.2 Document first-version safety boundaries and lack of authentication.
- [x] 7.3 Run Go tests, frontend checks/build, and `openspec validate --all`.
