## 1. Schema And Model

- [x] 1.1 Add `model.IntradayPoint` with market, symbol, trade_date, point_time, point_index, price, and volume fields.
- [x] 1.2 Add `a_share_intraday_points` bootstrap DDL to `internal/clickhouse/schema.go`.
- [x] 1.3 Add schema tests that assert table columns, partitioning, engine, and order key.
- [x] 1.4 Add `Store.InsertIntradayPoints` with month-partition batching.
- [x] 1.5 Add store tests for intraday point insert SQL and empty-batch behavior.

## 2. TDX Intraday Import

- [x] 2.1 Add an intraday import options/summary type that reuses existing task run, watermark, and quality issue models.
- [x] 2.2 Normalize `tdx.HQMinutePoint` rows into `model.IntradayPoint` using `Asia/Shanghai` trade dates and point times.
- [x] 2.3 Deduplicate identical points within one response and record quality issues for conflicting duplicate logical keys.
- [x] 2.4 Implement single-date historical import using `tdx.FetchHQHistoryMinuteTime`.
- [x] 2.5 Implement current-day import using `tdx.FetchHQMinuteTime` with deterministic trade_date handling.
- [x] 2.6 Implement bounded date-range import by fetching one date at a time.
- [x] 2.7 Preserve empty historical responses as successful zero-row dates without synthetic rows.
- [x] 2.8 Record task runs, watermarks, and quality issues for non-dry-run imports.
- [x] 2.9 Support dry-run summaries without writing ClickHouse market fact rows.

## 3. Marketd CLI

- [x] 3.1 Add `marketd import-tdx-intraday-points` command routing in `internal/cli`.
- [x] 3.2 Add flags for `--market`, `--symbol`, `--date`, `--since`, `--until`, `--today`, `--server`, bestip options, batch/client options, config, and `--dry-run`.
- [x] 3.3 Validate unsupported market, invalid symbol, invalid date, inverted date range, and missing date selection before writes.
- [x] 3.4 Add CLI tests for historical date, date range, current-day, dry-run, and invalid argument cases.
- [x] 3.5 Ensure existing `hq-minute`, `hq-history-minute`, and `/api/tdx/*` provider reads remain read-only.

## 4. Querier API

- [x] 4.1 Add intraday point query/request/result DTOs to `internal/querier`.
- [x] 4.2 Extend the `querier.Repository` interface with an intraday point query method.
- [x] 4.3 Implement ClickHouse read SQL only in `internal/clickhouse/query.go`.
- [x] 4.4 Add `GET /api/v1/intraday-points` with date and time-range query modes.
- [x] 4.5 Validate market, symbol, date, datetime bounds, inverted ranges, and query limit.
- [x] 4.6 Add API tests for successful date query, time-range query, empty result, validation errors, and no live TDX fallback.

## 5. Infinity CLI

- [x] 5.1 Add `infinity querier intraday-points` command as an HTTP client of `/api/v1/intraday-points`.
- [x] 5.2 Add CLI flags for API URL, market, symbol, date, since, until, and limit.
- [x] 5.3 Add CLI tests proving it does not connect directly to ClickHouse.

## 6. Documentation And Verification

- [x] 6.1 Update `docs/storage/clickhouse.md` with the intraday point table contract and import semantics.
- [x] 6.2 Update `docs/api/README.md` with `/api/v1/intraday-points` request and response examples.
- [x] 6.3 Update TDX data/reference docs to distinguish persisted intraday points from local 1-minute OHLCV bars.
- [x] 6.4 Run `gofmt` on touched Go files.
- [x] 6.5 Run focused Go tests for changed packages.
- [x] 6.6 Run `go test ./...`.
- [x] 6.7 Run `openspec validate --all`.
