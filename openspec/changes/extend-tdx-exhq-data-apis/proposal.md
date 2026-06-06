## Why

`marketd` already has a separate TDX extended行情 (`exhq`) path for market metadata and single instrument quote snapshots. Operators now need the rest of the pytdx-compatible read APIs so extended instruments can be inspected without Python:

- instrument count and instrument list
- K-line data
- minute-time data
- transaction data
- historical minute-time and transaction data
- historical K-line date range data

This remains an on-demand read capability. Realtime quote persistence is explicitly out of scope.

## What Changes

- Add Go-native ExHQ packet builders and response decoders for instrument catalog, K-line, minute-time, transaction, and history APIs.
- Add CLI commands:
  - `marketd exquote-count`
  - `marketd exquote-instruments`
  - `marketd exquote-bars`
  - `marketd exquote-minute`
  - `marketd exquote-history-minute`
  - `marketd exquote-transactions`
  - `marketd exquote-history-transactions`
  - `marketd exquote-history-bars`
- Decode ExHQ names with GB18030 fallback because TDX text fields are commonly GBK/GB18030.
- Use current public server behavior: business packets can be sent directly; the older pytdx setup packet may hang on currently reachable servers.
- Update docs and tests for the expanded ExHQ read surface.

## Non-Goals

- Do not persist realtime or historical ExHQ data to ClickHouse.
- Do not define canonical storage schema for futures, options, Hong Kong, or external markets.
- Do not merge extended market IDs with A-share `sh` / `sz` / `bj` markets.
- Do not implement Level-2 authenticated feeds.

## Capabilities

### Modified Capabilities
- `marketd-tdx-extended-quotes`: Expand ExHQ from market list/single quote to the pytdx-compatible read API set listed above.

## Impact

- New protocol code under `internal/tdx`.
- CLI routing and tests under `internal/cli`.
- Documentation updates for command usage, protocol shape, server behavior, and live server caveats.
