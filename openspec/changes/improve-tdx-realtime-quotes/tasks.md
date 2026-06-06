## 1. Server Selection

- [x] 1.1 Add a configurable TDX HQ server candidate list.
- [x] 1.2 Implement server probe logic that reports address, success/failure, latency, and error reason.
- [x] 1.3 Add a CLI command or flag path for probing and selecting the fastest successful HQ server.
- [x] 1.4 Add tests for probe success, timeout/failure reporting, and best-server selection.

## 2. Retry and Connection Reuse

- [x] 2.1 Add quote retry across server candidates for connection, setup, and request failures.
- [x] 2.2 Preserve decode/protocol errors clearly so retries do not hide parser regressions.
- [x] 2.3 Add an explicit batch quote client that reuses one setup connection across multiple batches.
- [x] 2.4 Add configurable batch size and tests for batch splitting.

## 3. Symbol Discovery and Quote Sweeps

- [x] 3.1 Implement online `sh` / `sz` security count and security list retrieval from TDX standard行情.
- [x] 3.2 Add tests for security-list response decoding.
- [x] 3.3 Add a full-market quote workflow that uses discovered symbols or an explicit symbol list.
- [x] 3.4 Add tests for quote sweep orchestration without live network dependency.

## 4. Market Coverage

- [x] 4.1 Investigate Beijing market realtime quote support and document whether it belongs in standard `hq` or another endpoint.
- [x] 4.2 Keep `bj` quote requests rejected until market mapping and live sample decoding are verified.
- [x] 4.3 Design `exhq` as a separate extended-market protocol client.
- [x] 4.4 Add tests that keep standard `hq` and `exhq` validation paths separate.

## 5. Timestamp and Storage Decisions

- [x] 5.1 Rename or document the current quote time as intraday server time where API output needs clarity.
- [x] 5.2 Add trade date / `Asia/Shanghai` timestamp handling when the date source is explicit.
- [x] 5.3 Decide whether realtime snapshots should be persisted to ClickHouse.
- [x] 5.4 Document that snapshot persistence requires a separate OpenSpec change if accepted.

## 6. Validation

- [x] 6.1 Update `docs/design/tdx-realtime-quotes.md` as each enhancement is implemented.
- [x] 6.2 Run `go test ./...`.
- [x] 6.3 Run `openspec validate --all`.
