## Why

`marketd` already supports TDX standard `hq` realtime snapshots for A-share `sh` / `sz` symbols. TDX extended行情 (`exhq`) is a separate protocol surface for futures, options, Hong Kong, foreign markets, and other extended instruments. Operators need a Go-native way to inspect extended market IDs and fetch a single extended instrument quote without relying on Python `pytdx`.

The implementation must keep `exhq` separate from the standard A-share quote path. Extended instruments use numeric market IDs and instrument codes, not `sh` / `sz` / `bj` six-digit stock symbols.

## What Changes

- Add a TDX `exhq` client using the extended行情 setup packet, market-list request, and single instrument quote request.
- Add typed extended market and extended quote models.
- Decode extended quote fields: pre-close, open, high, low, last price, open-interest style fields, volume fields, and five bid/ask levels.
- Add CLI commands:
  - `marketd exquote-markets`
  - `marketd exquote --market <id> --code <instrument>`
- Keep extended quote retrieval on demand only; do not write extended quotes to ClickHouse.

## Non-Goals

- Do not implement extended K-line, minute-time, transaction, history, or instrument-list APIs in this change.
- Do not add extended行情 ClickHouse tables or persistence.
- Do not define canonical futures/options/Hong Kong instrument storage semantics.
- Do not mix extended market IDs with A-share `market` values in existing fact tables.

## Capabilities

### New Capabilities
- `marketd-tdx-extended-quotes`: Fetch TDX extended行情 market metadata and single instrument quote snapshots.

### Modified Capabilities
- `marketd-realtime-quotes`: Standard A-share quote validation remains separate from extended行情 validation.

## Impact

- New code under `internal/tdx` for `exhq` packets, sessions, parsers, and default servers.
- CLI routing changes under `internal/cli`.
- Tests for request validation, packet construction, market-list decoding, quote decoding, CLI validation, and JSON output.
- Documentation updates describing `hq` versus `exhq`, supported commands, and current limits.
