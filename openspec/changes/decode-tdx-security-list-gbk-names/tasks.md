## 1. Decoder

- [x] 1.1 Add `golang.org/x/text/encoding/simplifiedchinese` dependency for GB18030 decoding.
- [x] 1.2 Add a helper that trims fixed-field null/space padding and decodes security names with GB18030.
- [x] 1.3 Update `DecodeSecurityListResponse` to use the helper for `record[8:16]`.
- [x] 1.4 Preserve current tolerant behavior by returning an empty name when name bytes cannot be decoded.

## 2. Tests

- [x] 2.1 Add a test fixture for a Chinese GBK security name such as `贵州茅台`.
- [x] 2.2 Add tests for ASCII names, null padding, and space padding.
- [x] 2.3 Add a malformed-name test proving the record is still returned with an empty name.

## 3. Documentation and Validation

- [x] 3.1 Update `docs/design/tdx-realtime-quotes.md` to describe GB18030 security-list name decoding.
- [x] 3.2 Update `docs/reference/tdx-server-capabilities.md` to remove the GBK decoding limitation.
- [x] 3.3 Run `go test ./...`.
- [x] 3.4 Run `openspec validate --all`.
