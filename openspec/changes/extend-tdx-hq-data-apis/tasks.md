## 1. Protocol Baseline

- [x] 1.1 Audit existing HQ quote, probe, retry, session, compression, and security discovery code against the new capability spec.
- [x] 1.2 Add shared HQ request/response helpers only where they reduce duplication across the new API families.
- [x] 1.3 Add or reuse GB18030/GBK text decoding helpers for HQ names, F10 content, and block fields.
- [x] 1.4 Add common validation for HQ markets, six-digit symbols, `YYYYMMDD` dates, non-negative starts, and bounded counts.
- [x] 1.5 Add scripted local HQ server test helpers for packet/response workflows that cannot rely on live public servers.

## 2. K-Line APIs

- [x] 2.1 Add standard HQ security K-line request packet builder for `get_security_bars`.
- [x] 2.2 Add standard HQ index K-line request packet builder for `get_index_bars`.
- [x] 2.3 Add K-line response decoder for timestamp, OHLC, volume, and amount fields.
- [x] 2.4 Enforce K-line category validation and `count <= 800` before network calls.
- [x] 2.5 Add client/session methods for security and index K-line fetches.
- [x] 2.6 Add CLI commands for security and index K-line reads with `--market`, `--symbol`, `--category`, `--start`, `--count`, and `--server`.

## 3. Minute-Time APIs

- [x] 3.1 Add current-day minute-time request packet builder for `get_minute_time_data`.
- [x] 3.2 Add historical minute-time request packet builder for `get_history_minute_time_data`.
- [x] 3.3 Add minute-time response decoder for `price + volume` points.
- [x] 3.4 Preserve empty historical minute-time responses as empty JSON arrays.
- [x] 3.5 Add client/session methods for current-day and historical minute-time fetches.
- [x] 3.6 Add CLI commands for current-day and historical minute-time reads.

## 4. Transaction APIs

- [x] 4.1 Add current-day transaction request packet builder for `get_transaction_data`.
- [x] 4.2 Add historical transaction request packet builder for `get_history_transaction_data`.
- [x] 4.3 Add transaction response decoder for time, price, volume, and direction fields where decodable.
- [x] 4.4 Enforce transaction start/count validation before network calls.
- [x] 4.5 Add client/session methods for current-day and historical transaction fetches.
- [x] 4.6 Add CLI commands for current-day and historical transaction reads.

## 5. Company, XDXR, and Finance APIs

- [x] 5.1 Add company info category request packet builder and decoder.
- [x] 5.2 Add company info content request packet builder and decoder.
- [x] 5.3 Add xdxr request packet builder and corporate-action decoder.
- [x] 5.4 Add finance info request packet builder and decoder.
- [x] 5.5 Add client/session methods for company info, xdxr, and finance fetches.
- [x] 5.6 Add CLI commands for company info categories, company info content, xdxr, and finance info.

## 6. Block APIs

- [x] 6.1 Add block metadata request packet builder and decoder.
- [x] 6.2 Add block membership request packet builder and decoder.
- [x] 6.3 Decode block names and member text fields with GB18030/GBK fallback.
- [x] 6.4 Add client/session methods for block metadata and block membership fetches.
- [x] 6.5 Add CLI commands for block metadata and block membership reads.

## 7. Tests

- [x] 7.1 Add packet construction tests for each new HQ API family.
- [x] 7.2 Add response decoder tests using captured or synthetic fixture payloads.
- [x] 7.3 Add validation tests for unsupported markets, invalid symbols, invalid dates, invalid pages, and K-line count over 800.
- [x] 7.4 Add scripted local server tests for client/session fetch wiring.
- [x] 7.5 Add CLI JSON and argument tests for each new command.
- [x] 7.6 Add regression tests proving existing `quote`, `quote-probe`, and security discovery behavior remains unchanged.

## 8. Documentation and Validation

- [x] 8.1 Update `docs/design/tdx-server-capabilities.md` with implemented HQ command coverage and caveats.
- [x] 8.2 Update `docs/reference/tdx-python-libraries.md` with marketd replacement status for each standard HQ API.
- [x] 8.3 Update `docs/tdx-data/通达信数据格式.md` with decoded field semantics for implemented online data shapes.
- [x] 8.4 Document that all commands are read-only and do not write ClickHouse.
- [x] 8.5 Run `gofmt` on changed Go files.
- [x] 8.6 Run `go test ./...`.
- [x] 8.7 Run `openspec validate --all`.
