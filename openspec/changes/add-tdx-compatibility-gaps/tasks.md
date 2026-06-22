## 1. Protocol Fixtures And Models

- [x] 1.1 Capture local protocol notes for compact batch quotes (`0x054C`), tick chart (`0x0537`), SP/fund probes, fund detail labels, and online adjusted bars.
- [x] 1.2 Add deterministic compact batch quote decoder fixtures and request-header tests.
- [x] 1.3 Add deterministic tick-chart decoder fixtures and request-header tests.
- [x] 1.4 Add SP and fund probe fake-server fixtures covering success, handshake failure, and timeout/error mapping.
- [x] 1.5 Add fund detail label dictionary fixtures for known and unknown item ids.

## 2. Compact Batch Quotes

- [x] 2.1 Add compact batch quote request/response types in `internal/tdx`.
- [x] 2.2 Implement compact batch quote packet builder and decoder.
- [x] 2.3 Add `QuoteSession` and `FetchHQCompactBatchQuotes` helpers with existing HQ server fallback behavior.
- [x] 2.4 Add validation tests for empty symbols, invalid symbols, unsupported markets, and request-size limits.

## 3. Tick Chart

- [x] 3.1 Add tick-chart request/response types in `internal/tdx`.
- [x] 3.2 Implement tick-chart packet builder and decoder.
- [x] 3.3 Add `QuoteSession` and `FetchHQTickChart` helpers with existing HQ server fallback behavior.
- [x] 3.4 Add tests proving tick-chart reads do not change existing minute-time APIs.

## 4. SP And Fund Server Discovery

- [x] 4.1 Add SP and fund server candidate lists in `internal/tdx`.
- [x] 4.2 Implement SP server probe and best-server selection using the SP handshake.
- [x] 4.3 Implement fund server probe and best-server selection using the fund 7727 bootstrap.
- [x] 4.4 Add CLI commands for listing/probing SP and fund candidates.
- [x] 4.5 Add provider functions and HTTP endpoints for SP and fund candidate/probe results.

## 5. Labeled Fund Detail

- [x] 5.1 Add a small fund detail item dictionary with confirmed labels, units, and value decoding rules.
- [x] 5.2 Extend fund detail response models to include decoded item output while preserving raw items.
- [x] 5.3 Update CLI and HTTP fund-detail output/tests to include decoded labels for known ids and raw rows for unknown ids.

## 6. One-Shot Online Adjusted Bars

- [x] 6.1 Add online adjusted bar request/response models for market, symbol, category, start, count, and `adjust=none|qfq|hfq`.
- [x] 6.2 Implement in-memory conversion from live HQ bars and live HQ XDXR rows into adjustment factor inputs.
- [x] 6.3 Implement online adjusted bar helper that returns adjusted OHLC for `qfq/hfq`, raw volume/amount, and explicit missing-input errors.
- [x] 6.4 Add `marketd hq-adjusted-bars-online` command.
- [x] 6.5 Add `/api/tdx/hq/adjusted-bars` provider endpoint.
- [x] 6.6 Add tests proving `/api/v1/bars?adjust=...` remains ClickHouse-backed and does not call live provider paths.

## 7. Documentation And Validation

- [x] 7.1 Update `docs/api/tdx.md` with new CLI/API examples and non-persistence notes.
- [x] 7.2 Update `docs/reference/tdx-python-libraries.md` to move these gaps from missing to covered or partially covered.
- [x] 7.3 Update `docs/reference/tdx-advanced-protocol-notes.md` with packet/probe/fund-label notes.
- [x] 7.4 Run `go test ./...`.
- [x] 7.5 Run `openspec validate --all`.
