## Why

Full-market minute scans have a different access pattern from canonical TDX minute imports.

Local TDX `.lc1/.1` and `.lc5/.5` files are organized by symbol, so the canonical 1-minute and 5-minute fact tables should remain optimized for `market + symbol + time` imports, reimports, and single-symbol time-series queries. Full-market scans, such as "all stocks at 10:30 ordered by amount or minute return", need a time-first layout.

Duplicating complete minute OHLCV history forever would waste storage. The scan layer should therefore be short-retention, narrow-column, and rebuildable from canonical facts.

## What Changes

- Keep local offline minute imports focused on raw canonical data only:
  - `a_share_bars_1m`
  - `a_share_bars_5m`
- Add a design for optional scan-derived tables:
  - `a_share_bars_1m_scan`
  - `a_share_bars_5m_scan`
- Scan tables are time-first:
  - `ORDER BY (trade_date, bar_time, market, symbol)`
- Scan tables store only columns needed for market scans and selected derived metrics.
- Scan tables use explicit refresh commands or jobs.
- Offline imports MUST NOT generate scan rows by default.

## Non-Goals

- No change to TDX local minute binary parsing.
- No automatic materialized view from canonical minute tables in this change.
- No requirement to keep scan data for the full historical retention period.
- No change to canonical table order keys.

## Impact

- New storage design and OpenSpec requirements for a rebuildable minute scan layer.
- Future CLI work will add explicit scan refresh commands.
- Operators can keep canonical minute history long-term while retaining only a shorter scan window.
