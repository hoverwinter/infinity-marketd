## Context

`marketd` already has two separate minute-related data shapes:

```text
local .lc1/.1 files -> a_share_bars_1m -> OHLCV bars
TDX HQ minute-time -> hq-minute/hq-history-minute -> price + volume points
```

The existing online TDX HQ commands and `/api/tdx/*` provider routes are live upstream reads and explicitly do not write ClickHouse. The existing `/api/v1` querier routes are ClickHouse-backed product/query APIs.

This change turns TDX HQ minute-time points into a persisted data product without treating them as 1-minute bars.

## Goals / Non-Goals

**Goals:**

- Add an idempotent ClickHouse table for A-share intraday `price + volume` points.
- Add an explicit write workflow for TDX standard HQ current-day and historical minute-time points.
- Track import runs, watermarks, and quality issues through the existing ops tables.
- Add a stable `/api/v1` query surface for persisted intraday points.
- Keep TDX provider live reads separate from persisted product APIs.

**Non-Goals:**

- Do not derive or write `a_share_intraday_points` from local `a_share_bars_1m` in this change.
- Do not convert TDX minute-time points into `a_share_bars_1m`.
- Do not persist ExHQ minute-time data in this A-share table.
- Do not add a new binary, package, scheduler, queue, or service layer.
- Do not add source, version, or updated-at columns to the market fact table.

## Decisions

### Store Intraday Points As Their Own Fact Table

Create:

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_intraday_points
(
    market LowCardinality(String),
    symbol String,
    trade_date Date,
    point_time DateTime('Asia/Shanghai'),
    point_index UInt16,
    price Float64,
    volume UInt64
)
ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(trade_date)
ORDER BY (market, symbol, trade_date, point_time);
```

Logical key:

```text
market + symbol + trade_date + point_time
```

Rationale: TDX HQ minute-time data contains `price + volume` points, not full OHLCV. A separate table preserves that source shape and avoids polluting `a_share_bars_1m`.

Alternative considered: derive points from `a_share_bars_1m` using `close + volume`. That is useful as a read model for charts, but it is not the same as preserving TDX server minute-time data.

Alternative considered: add `amount Nullable(Float64)`. Current standard HQ minute-time decoding does not provide amount. Adding a nullable column now would create a field with no accepted producer, so this change leaves it out.

Alternative considered: use `Decimal64(4)` for price. Existing market facts use `Float64`, and the storage docs still leave fixed decimal as an open question. This change should not introduce a mixed price representation.

### Use Explicit Import Commands

Add a command shaped around the existing TDX HQ minute-time clients:

```text
marketd import-tdx-intraday-points --market sh --symbol 600519 --date 2026-06-05 --server 180.153.18.170:7709
marketd import-tdx-intraday-points --market sh --symbol 600519 --since 2026-06-01 --until 2026-06-05 --server 180.153.18.170:7709
marketd import-tdx-intraday-points --market sh --symbol 600519 --today --server 180.153.18.170:7709
```

Historical imports call the existing historical minute-time fetch per trade date. Current-day imports call the existing current minute-time fetch and require an explicit trade date decision through `--today` or an operator-provided date if the protocol response does not include one.

Rationale: explicit commands keep imports observable and make network cost/operator intent clear.

Alternative considered: have `/api/tdx/hq/minute` write through as a side effect. That would violate the current provider boundary where `/api/tdx/*` is a live read namespace.

### Keep Import Orchestration Small

Add a small intraday import path that reuses:

- `tdx.FetchHQMinuteTime`
- `tdx.FetchHQHistoryMinuteTime`
- `clickhouse.Store`
- `model.TaskRun`
- `model.Watermark`
- `model.QualityIssue`

Do not add a new long-running worker. Date-range import can loop over calendar dates, accepting empty responses as normal skipped/no-data days with a quality or summary signal.

### Query Persisted Points Through `/api/v1`

Add a ClickHouse-backed endpoint:

```text
GET /api/v1/intraday-points?market=sh&symbol=600519&date=2026-06-05
GET /api/v1/intraday-points?market=sh&symbol=600519&since=2026-06-05T09:30:00&until=2026-06-05T15:00:00&limit=240
```

Response rows expose `market`, `symbol`, `trade_date`, `point_time`, `point_index`, `price`, and `volume`.

Rationale: `/api/v1` is the stable product/query API backed by ClickHouse. `/api/tdx/hq/minute` remains the live upstream read path and should not imply persistence.

## Risks / Trade-offs

- TDX servers can return empty historical minute-time data for valid-looking dates -> Treat empty responses as non-fatal, record rows written as zero, and update task status/message clearly.
- Date-range import over many symbols can be slow and server-sensitive -> Start with explicit single-symbol/date or bounded date-range commands; do not add full-market scheduling in this change.
- `volume` unit semantics can differ from downstream expectations -> Preserve the decoded protocol value as `volume` and document it as the TDX minute-time volume field.
- Re-imports may create physical duplicates before ClickHouse merges -> Use the same logical key and `ReplacingMergeTree`; resolve duplicate/conflicting points in memory before insert when they occur in one response.
- Current-day points may not include a full date in the protocol response -> Require a deterministic trade date from the operator or runtime date logic and record it in task params.

## Migration Plan

1. Add `CREATE TABLE IF NOT EXISTS` bootstrap DDL. This is non-destructive and idempotent.
2. Add model/store insert support and tests.
3. Add the explicit import command behind normal config loading.
4. Add repository/query API and CLI support.
5. Update storage and API docs.

Rollback is non-destructive: stop running the new import command and stop using the new query endpoint. Existing OHLCV fact tables and live TDX provider APIs are unaffected.

## Open Questions

- Should date-range imports skip known non-trading days through a calendar table later, or keep the first version server-driven and tolerate empty responses?
- Should multi-symbol batch import be added after the single-symbol workflow is validated?
- Should `point_index` be part of the order key if a TDX server ever returns duplicate `point_time` rows for a symbol/date?
