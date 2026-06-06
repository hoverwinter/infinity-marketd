## 1. HTTP API Boundary

- [x] 1.1 Add route registration for `/api/tdx/hq/*` and `/api/tdx/exhq/*` without changing existing `/api/v1/*` routes.
- [x] 1.2 Add shared TDX provider API helpers for JSON responses, validation errors, upstream unavailable errors, and protocol decode errors.
- [x] 1.3 Add tests proving `/api/v1/bars` does not initiate live TDX requests.

## 2. Standard HQ Routes

- [x] 2.1 Implement `/api/tdx/hq/quotes` using existing realtime quote request parsing and `tdx.FetchRealtimeQuotes`.
- [x] 2.2 Implement `/api/tdx/hq/probe` using existing HQ server probe behavior.
- [x] 2.3 Implement `/api/tdx/hq/securities` using existing HQ security list reads.
- [x] 2.4 Implement HTTP coverage for supported HQ read APIs: bars, minute data, transactions, company/F10, xdxr, finance, and block reads.
- [x] 2.5 Add request validation and tests for market, symbol, count, date, category, server override, and batch-size parameters.

## 3. Extended ExHQ Routes

- [x] 3.1 Implement `/api/tdx/exhq/markets`, `/api/tdx/exhq/count`, and `/api/tdx/exhq/instruments`.
- [x] 3.2 Implement `/api/tdx/exhq/quote` using existing ExHQ quote behavior.
- [x] 3.3 Implement HTTP coverage for supported ExHQ read APIs: bars, minute data, transactions, and history ranges.
- [x] 3.4 Add request validation and tests for numeric market id, instrument code, date, start, count, category, and server override parameters.

## 4. Non-Persistence And Operational Safety

- [x] 4.1 Add tests proving `/api/tdx/*` routes do not write quote snapshots or online reads to ClickHouse.
- [x] 4.2 Enforce request size limits for quote batches and sweep-style operations exposed through HTTP.
- [x] 4.3 Ensure upstream timeout and all-server-failed errors map to `503`.
- [x] 4.4 Ensure protocol decode errors map to `502`.

## 5. Documentation And Verification

- [x] 5.1 Update `docs/api/README.md` with the `/api/v1` versus `/api/tdx` namespace contract and TDX provider endpoint reference.
- [x] 5.2 Update `docs/design/tdx-realtime-quotes.md` with the HTTP boundary, non-persistence decision, and no WebSocket/SSE decision.
- [x] 5.3 Run `go test ./...`.
- [x] 5.4 Run `openspec validate add-tdx-provider-http-api --strict` and `openspec validate --all`.
