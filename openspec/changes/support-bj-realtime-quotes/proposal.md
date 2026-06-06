## Why

`marketd` realtime quotes currently support Shanghai and Shenzhen standard行情 snapshots, but Beijing Stock Exchange symbols are explicitly rejected because the TDX market mapping and live response samples have not been verified. Operators need North Exchange realtime quotes handled by the same Go-native quote path once the protocol behavior is proven.

## What Changes

- Verify how TDX standard行情 represents Beijing market realtime quotes, including market byte, symbol list behavior, and quote response shape.
- Add `bj` support to realtime quote request validation only after live samples and parser tests confirm the mapping.
- Extend online security-list discovery and `quote-sweep` workflows to include `bj` when the verified protocol path supports it.
- Update CLI behavior so `marketd quote --symbol bj:920001` and inferred Beijing symbols can return JSON quote snapshots.
- Preserve the current explicit unsupported-market error until the verified implementation is in place.
- Document the verified TDX Beijing market behavior and any limitations.

## Capabilities

### New Capabilities
- `marketd-bj-realtime-quotes`: Fetch and sweep Beijing Stock Exchange realtime quote snapshots through the verified TDX standard行情 path.

### Modified Capabilities

## Impact

- Changes to `internal/tdx` quote market validation, TDX market mapping, request construction, and security-list discovery.
- CLI support for `bj` in `quote` and `quote-sweep`.
- Tests using captured or synthetic Beijing quote/list samples.
- Documentation updates in `docs/design/tdx-realtime-quotes.md`.
- No ClickHouse schema change and no realtime snapshot persistence in this change.
