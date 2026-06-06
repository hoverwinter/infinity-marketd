## 1. TDX Quote Protocol

- [x] 1.1 Add realtime quote data types and symbol parsing for `sh` / `sz` A-share requests.
- [x] 1.2 Implement TDX standard HQ TCP setup, request construction, response header handling, optional zlib decompression, and quote response decoding.
- [x] 1.3 Add unit tests for symbol parsing, request bytes, variable-length integer decoding, response decoding, and server time formatting.

## 2. CLI

- [x] 2.1 Add a `quote` command with `--symbol` and `--server` flags.
- [x] 2.2 Emit deterministic JSON quote arrays and return validation/query errors without opening ClickHouse.
- [x] 2.3 Add CLI tests for argument validation, server override wiring, and JSON output behavior.

## 3. Validation

- [x] 3.1 Run `gofmt` and Go tests.
- [x] 3.2 Run `openspec validate --all`.
