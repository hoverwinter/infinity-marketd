## Why

TDX standard HQ minute-time data is currently available only as an online read path, so callers cannot treat it as a persisted market data product. A dedicated intraday point table lets operators store the raw `price + volume` minute-time points without confusing them with local 1-minute OHLCV bars.

## What Changes

- Add `infinity_market.a_share_intraday_points` as the canonical persisted table for A-share TDX minute-time points.
- Add an explicit `marketd` import or refresh workflow that fetches TDX current-day or historical minute-time data and writes points to ClickHouse.
- Preserve TDX minute-time semantics:
  - points are `price + volume`;
  - points are not 1-minute OHLCV bars;
  - writes MUST NOT populate `a_share_bars_1m`;
  - local 1-minute OHLCV imports MUST NOT implicitly populate intraday point rows.
- Add a ClickHouse-backed query API for persisted intraday points under `/api/v1`, separate from live `/api/tdx/*` provider reads.
- Record task runs, watermarks, and data quality issues for intraday point imports.

## Capabilities

### New Capabilities
- `marketd-tdx-intraday-points`: Fetch and persist TDX standard HQ minute-time points for A-share symbols.
- `infinity-intraday-query-api`: Query persisted A-share intraday points through the querier API and CLI.

### Modified Capabilities
- `marketd-clickhouse-data-plane`: Add the `a_share_intraday_points` table contract and operational metadata expectations.
- `marketd-tdx-local-import`: Clarify that local 1-minute OHLCV imports do not write intraday point rows by default.

## Impact

- `internal/clickhouse/schema.go`: bootstrap DDL for `a_share_intraday_points`.
- `internal/clickhouse/store.go` and related write paths: batch insert support for intraday points and task/watermark updates.
- `internal/tdx/hq_data.go`: reuse existing HQ minute-time fetch and decoder behavior.
- `internal/cli`: add explicit intraday point import or refresh command.
- `internal/querier`, `internal/clickhouse/query.go`, `internal/infinitycli`: add ClickHouse-backed intraday point query support.
- `docs/storage/clickhouse.md`, `docs/api/README.md`, and TDX data references: document persisted intraday point semantics.
