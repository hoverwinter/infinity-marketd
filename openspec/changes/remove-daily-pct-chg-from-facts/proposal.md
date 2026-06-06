## Why

`pct_chg` is not a native TongDaXin `.day` field. Local daily records contain date, OHLC, amount, volume, and a reserved field, but not percentage change.

Storing `pct_chg` in `a_share_bars_1d` makes the canonical fact table mix raw facts with cross-row derived data. The value depends on the previous valid close for the same `market + symbol`; if earlier history is backfilled, a close is corrected, or adjusted/unadjusted price policy changes, already stored `pct_chg` values become stale unless explicitly recomputed.

## What Changes

- Remove `pct_chg` from the canonical daily fact table `infinity_market.a_share_bars_1d`.
- Stop inserting `pct_chg` during local TDX daily imports.
- Treat percentage change as a derived metric.
- Add a derived daily table or refresh job for query-heavy derived fields such as:
  - `prev_close`
  - `pct_chg`
  - later daily scan flags or ranking fields
- Make derived daily values rebuildable for a date range or for a full symbol history.

## Non-Goals

- No change to the raw `.day` parser field layout.
- No attempt to use ClickHouse `MATERIALIZED` columns for `pct_chg`.
- No automatic historical recomputation triggered by ClickHouse mutations.
- No frontend changes.

## Impact

- ClickHouse schema changes for `a_share_bars_1d`.
- Daily insert code no longer writes `pct_chg`.
- Existing deployments with `pct_chg` in `a_share_bars_1d` need a migration or table rebuild.
- Query paths that filter by daily percentage change should read a derived table or compute from current close and previous close at query time.
