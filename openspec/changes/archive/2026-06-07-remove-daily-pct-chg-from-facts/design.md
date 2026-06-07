## Decision

Keep `a_share_bars_1d` as a canonical raw daily OHLCV fact table and remove `pct_chg` from it.

`pct_chg` should be computed from the previous valid close:

```text
pct_chg = (close - prev_close) / prev_close * 100
```

The first record for a symbol, records with missing previous close, and records with non-positive previous close should produce `NULL` derived values.

## Rationale

The TDX `.day` format does not provide `pct_chg`. It is a cross-row calculation, so storing it in the base fact table creates update-order problems:

- importing a later date before an earlier missing date can produce a wrong previous close;
- backfilling old data can change the first valid previous close for later rows;
- correcting a `close` value requires recomputing the next row's `pct_chg`;
- switching price policy, such as adjusted versus unadjusted prices, changes every downstream percentage change.

ClickHouse `MATERIALIZED` columns do not solve this because they calculate on insert and cannot look up a previous row in the same ordered symbol history.

## Schema Shape

Canonical daily facts:

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_bars_1d
(
    market LowCardinality(String),
    symbol String,
    trade_date Date,
    open Float64,
    high Float64,
    low Float64,
    close Float64,
    volume UInt64,
    amount Float64
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(trade_date)
ORDER BY (market, symbol, trade_date);
```

Derived daily metrics can be stored separately:

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_daily_derived
(
    market LowCardinality(String),
    symbol String,
    trade_date Date,
    prev_close Nullable(Float64),
    pct_chg Nullable(Float64),
    computed_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(computed_at)
PARTITION BY toYear(trade_date)
ORDER BY (trade_date, market, symbol);
```

The derived table is ordered by `trade_date` first because the primary high-frequency query is a daily market scan, for example "all stocks with `pct_chg > 8` on a target date".

## Refresh Semantics

Derived values are refreshed by an explicit batch job. A refresh may rebuild:

- one symbol's full history;
- all symbols for a date range;
- all symbols for the full available history.

For date-range rebuilds, the job must include at least one earlier valid close per symbol so the first day in the requested range can derive `prev_close` correctly.

## Query Guidance

Ad hoc queries may compute `pct_chg` directly with a previous-close join. Repeated daily scans should use `a_share_daily_derived` so filters like `pct_chg > 8` do not repeatedly perform full-symbol window calculations.

## Migration Guidance

Existing `a_share_bars_1d` tables that include `pct_chg` or use monthly partitioning should be rebuilt before re-importing canonical daily facts. `CREATE TABLE IF NOT EXISTS` will not change an existing table's columns or partition expression.

For disposable development data, drop and recreate the table:

```sql
DROP TABLE IF EXISTS infinity_market.a_share_bars_1d;
```

For production data, create a replacement table with the new schema, copy validated OHLCV rows into it, swap names during a maintenance window, and rebuild `a_share_daily_derived` afterwards.
