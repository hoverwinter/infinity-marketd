## Why

TDX standard行情 security-list records store Chinese security names in GBK/GB18030 bytes, but `marketd` currently keeps names only when those bytes happen to be valid UTF-8. This drops normal Chinese names from online security discovery and weakens operator-facing quote sweep diagnostics.

## What Changes

- Decode TDX security-list `name` bytes with GB18030/GBK-compatible decoding.
- Preserve ASCII names and trim null/space padding.
- Keep parser behavior tolerant: malformed name bytes MUST NOT fail the entire security-list response.
- Add tests with Chinese GBK fixtures and invalid-name fallback.
- Update realtime quote design/reference docs to remove the current GBK decoding limitation.

## Capabilities

### New Capabilities
- `marketd-tdx-security-list-names`: Decode TDX standard行情 security-list names from GBK/GB18030 bytes.

### Modified Capabilities

## Impact

- `internal/tdx` security-list decoder.
- `internal/tdx` tests.
- `go.mod` / `go.sum` if `golang.org/x/text` is added for GB18030 decoding.
- Documentation under `docs/design` and `docs/reference`.
- No change to quote price decoding, ClickHouse schemas, or realtime quote snapshot persistence.
