## Why

`marketd` currently ingests historical bars from local TDX files, but it does not expose a way to query current A-share quote snapshots. Operators need a first-party replacement for pytdx `hq.get_security_quotes` so realtime quote checks can use the same daemon and market-code conventions as the rest of the project.

## What Changes

- Add a TDX standard quote client for A-share realtime snapshots from compatible TDX HQ servers.
- Add a CLI command that fetches one or more realtime A-share quotes and prints structured JSON.
- Decode the quote fields needed for price, previous close, open/high/low, volume, amount, server time, and five-level bid/ask depth.
- Keep realtime quotes as an on-demand query path; do not write realtime snapshots to canonical ClickHouse fact tables in this change.

## Capabilities

### New Capabilities
- `marketd-realtime-quotes`: Fetch realtime A-share quote snapshots from TDX standard行情 servers.

### Modified Capabilities

## Impact

- New internal TDX realtime quote client and parser code.
- CLI routing changes under `cmd/marketd` / `internal/cli`.
- Configuration may gain optional TDX HQ server settings, with sensible defaults if omitted.
- Tests for request construction, response decoding, market inference, and CLI validation.
