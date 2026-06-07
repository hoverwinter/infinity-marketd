## Why

`marketd` already covers the core TDX `hq` and `exhq` online reads, but it still lacks several higher-level online capabilities that are visible in `millken/tdx`: sorted market quote lists, top rankings, SP/MAC live board members, LHB parsing from F10, and fund-specific 7727 requests. These APIs are useful for scanner, console, and research workflows because they avoid rebuilding every ranking from raw per-symbol reads.

## What Changes

- Add TDX `hq` sorted quote list support similar to `GetQuotesList`, including category, sort type, start/count pagination, reverse order, and optional stock-type exclude bitmask.
- Add TDX `hq` top board support similar to `GetTopBoard`, returning the standard ranking groups such as gainers, losers, amplitude, speed, volume ratio, commission ratio, and turnover.
- Add SP/MAC protocol board member support similar to `GetBoardMembers`, separate from existing static HQ block-file reads and local TDX block imports.
- Add LHB parsing from existing F10 company category/content reads by locating the `资金动向` section and extracting Dragon-Tiger list records into structured JSON.
- Add fund-specialized 7727 protocol support for fund K-line and raw fund detail responses, separate from generic ExHQ bars.
- Productize only the SDK surface that `marketd` actually needs:
  - server selection/probe/cache hooks for these new online reads;
  - reusable provider wiring through existing CLI and `/api/tdx/*` boundaries;
  - no broad public SDK abstraction that duplicates `internal/quotesvc` or existing short-read helpers.
- Keep all new capabilities as live upstream reads by default; do not write ClickHouse facts or derived tables in this change.

## Capabilities

### New Capabilities

- `marketd-tdx-advanced-online-apis`: Advanced TDX online provider APIs for sorted market lists, top rankings, SP board members, F10 LHB parsing, and 7727 fund-specific reads.

### Modified Capabilities

- None.

## Impact

- `internal/tdx`: add protocol packets, decoders, request validation, and fake-server/fixture tests for advanced HQ/SP/fund reads.
- `internal/cli`: add CLI commands for sorted quote lists, top boards, SP board members, LHB, fund K-line, and fund detail.
- `internal/querier`: expose matching `/api/tdx/*` provider endpoints without mixing them into `/api/v1` ClickHouse-backed queries.
- `docs/api/tdx.md`, `docs/reference/tdx-server-capabilities.md`, and `docs/design/tdx-server-capabilities.md`: document contracts, known protocol limits, source references, and implementation status.
- No ClickHouse schema changes are expected unless implementation discovers a narrow ops-plane observability need; market fact persistence remains out of scope.
