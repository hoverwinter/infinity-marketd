## Why

The project now exposes CLI commands and API endpoints for imports, watermarks, realtime quotes, TDX provider access, and bestip selection, but operators still need shell commands to understand system health. A lightweight console will make operational state visible without turning the project into a broad data-management backend.

## What Changes

- Add an `infinity-console` web console for operator-facing visibility into marketd and infinity runtime state.
- Add a Node.js + Vite frontend development workflow for the console.
- Add a standalone `infinity-console` binary that starts the console HTTP server and serves built Vite assets.
- Keep optional console static serving available through the existing Go API server path for compatibility.
- Expose or reuse read-only HTTP endpoints for console state: health, watermarks, task runs, data quality issues, realtime quote service health, and TDX HQ bestip status.
- Allow only narrow, non-destructive console actions in the first scope, such as refreshing bestip or running provider smoke probes.
- Keep destructive ClickHouse operations out of the console.

## Capabilities

### New Capabilities
- `infinity-console`: Operator web console for viewing data-plane health, ops status, realtime quote state, TDX provider smoke checks, and bestip cache state with a Node.js + Vite frontend workflow.

### Modified Capabilities

None.

## Impact

- Frontend: new Vite-based console app using Node.js tooling.
- Backend: Go server route(s) to serve built static assets and JSON endpoints required by the console.
- CLI/build: standalone `infinity-console` binary plus development and production build commands for the console.
- Docs: console development, build, and runtime usage.
- Dependencies: Node.js/Vite frontend dependencies; no new Python runtime or ClickHouse schema changes.
