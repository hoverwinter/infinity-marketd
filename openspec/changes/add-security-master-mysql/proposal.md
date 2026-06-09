## Why

`marketd` currently stores and queries market facts by `market + symbol`, but it has no durable securities master for current names, aliases, history, listing status, or manual corrections. Operators also need Beijing Stock Exchange security-list discovery as part of the same identity layer; current TDX HQ support covers explicit `bj` quotes but not `bj` security discovery.

## What Changes

- Add a MySQL-backed securities master for mutable reference data while keeping行情 and trading facts in ClickHouse.
- Add MySQL configuration and connection plumbing for `marketd` / `infinity` without hard-coded DSNs.
- Add explicit refresh commands for securities master data, with source selection and market selection flags.
- Support TDX HQ security-list discovery for verified `bj` markets, in addition to existing `sh` / `sz` discovery.
- Add two base query APIs:
  - `GET /api/v1/securities?market=sh&symbol=600519`
  - `GET /api/v1/securities/resolve?q=贵州茅台`
- Keep `/api/v1/bars` ClickHouse-only; it must not query MySQL or return joined security names.

## Capabilities

### New Capabilities

- `marketd-security-master`: MySQL-backed securities master storage, refresh commands, and base securities query APIs.

### Modified Capabilities

- `marketd-tdx-hq-data-apis`: standard HQ security discovery expands to verified `bj` security count/list reads.
- `marketd-bj-realtime-quotes`: Beijing online security-list discovery changes from explicitly unsupported to supported after verification.

## Impact

- Adds a MySQL driver dependency and `mysql:` configuration section.
- Adds MySQL schema bootstrap/migration behavior for securities master tables.
- Adds write-plane refresh logic for security master sources and source/market selection flags.
- Adds read-plane repository methods and HTTP routes for securities lookup and resolve.
- Updates TDX HQ security list validation and tests for `bj`.
- Updates API and storage documentation to preserve the boundary: MySQL mutable reference data, ClickHouse market/trading facts.
