## 1. Schema Contract

- [x] 1.1 Remove `pct_chg` from the `a_share_bars_1d` bootstrap DDL.
- [x] 1.2 Add bootstrap DDL for `a_share_daily_derived`.
- [x] 1.3 Add migration guidance for existing ClickHouse deployments, including `ALTER TABLE ... DROP COLUMN pct_chg` or table rebuild.

## 2. Import Path

- [x] 2.1 Remove `PctChg` from the daily bar model or leave it unused outside derived calculations.
- [x] 2.2 Remove `pct_chg` from daily insert column lists and batch append arguments.
- [x] 2.3 Update tests that assert daily table DDL and insert shape.

## 3. Derived Metrics

- [ ] 3.1 Add a refresh command or job for daily derived metrics.
- [ ] 3.2 Compute `prev_close` using the previous valid row for the same `market + symbol`.
- [ ] 3.3 Compute `pct_chg` as `(close - prev_close) / prev_close * 100`.
- [ ] 3.4 Store `NULL` when previous close is missing or non-positive.
- [ ] 3.5 Support date-range refresh with enough lookback to calculate the first requested date.

## 4. Verification

- [ ] 4.1 Add tests for first-row `NULL` derived values.
- [ ] 4.2 Add tests for missing calendar days and suspended stocks where previous valid close is not the previous natural day.
- [ ] 4.3 Add tests for correcting a close and recomputing the affected derived rows.
- [ ] 4.4 Verify a daily scan query for `pct_chg > 8` uses the derived table.
