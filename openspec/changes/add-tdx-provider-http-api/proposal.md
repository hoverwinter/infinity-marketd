## Why

`marketd` already has Go-native TDX standard行情 and extended行情 read capabilities, but they are currently exposed mainly through CLI commands and internal packages. Operators and future control-console clients need an HTTP surface for these provider/protocol reads without mixing them into the ClickHouse-backed `/api/v1` market data contract.

## What Changes

- Add a TDX provider HTTP API namespace under `/api/tdx/...`.
- Keep `/api/v1/...` as the product/query API for ClickHouse-backed canonical market data.
- Expose TDX standard行情 (`hq`) read operations through `/api/tdx/hq/...`.
- Expose TDX extended行情 (`exhq`) read operations through `/api/tdx/exhq/...`.
- Preserve existing `marketd` CLI commands and `internal/tdx` protocol behavior.
- Update API and realtime quote design docs to describe the provider API boundary, path conventions, error model, and non-persistence contract.
- Do not persist realtime snapshots to ClickHouse in this change.
- Do not add WebSocket or SSE streaming in this change.

## Capabilities

### New Capabilities

- `tdx-provider-http-api`: TDX provider/protocol HTTP API namespace, including standard行情 and extended行情 read endpoints, server probe endpoints, provider-specific validation, and clear separation from ClickHouse-backed `/api/v1` APIs.

### Modified Capabilities

- None.

## Impact

- New HTTP routes in the querier/server layer.
- New request parsing and response mapping from existing `internal/tdx` operations.
- API documentation updates under `docs/api`.
- Realtime quote design documentation updates under `docs/design`.
- No ClickHouse schema change.
- No change to current `/api/v1/bars`, `/api/v1/health`, or `/api/v1/resolve-symbol` behavior.
