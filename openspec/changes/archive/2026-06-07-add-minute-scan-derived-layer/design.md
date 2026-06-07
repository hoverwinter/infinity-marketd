## Decision

Use two layers for minute bars:

```text
canonical layer
  a_share_bars_1m
  a_share_bars_5m
  long-retention raw OHLCV facts
  ORDER BY (market, symbol, bar_time)

scan layer
  a_share_bars_1m_scan
  a_share_bars_5m_scan
  short-retention derived scan data
  ORDER BY (trade_date, bar_time, market, symbol)
```

Local offline imports write only the canonical layer. Scan data is generated only by an explicit refresh command or scheduled job.

## Canonical Tables Stay Symbol-First

TDX minute files are per symbol:

```text
vipdoc/sh/minline/sh600519.lc1
vipdoc/sh/fzline/sh600519.lc5
```

The canonical logical key is therefore:

```text
market + symbol + bar_time
```

Keeping `ORDER BY (market, symbol, bar_time)` makes imports, reimports, single-symbol queries, continuity checks, and per-symbol calculations straightforward.

## Scan Tables Are Time-First

Full-market scan queries need a different shape:

```sql
WHERE trade_date = toDate('2026-06-05')
  AND bar_time = toDateTime('2026-06-05 10:30:00', 'Asia/Shanghai')
ORDER BY amount DESC
```

These queries should use a scan table ordered by:

```text
trade_date + bar_time + market + symbol
```

## Narrow Schema

The scan layer should not copy full minute OHLCV by default. It stores the fields needed for scanning and selected derived metrics:

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_bars_1m_scan
(
    trade_date Date,
    bar_time DateTime('Asia/Shanghai'),
    market LowCardinality(String),
    symbol String,
    close Float64,
    volume UInt64,
    amount Float64,
    prev_close Nullable(Float64),
    minute_ret Nullable(Float64),
    volume_ratio Nullable(Float64),
    computed_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(computed_at)
PARTITION BY toYYYYMM(trade_date)
ORDER BY (trade_date, bar_time, market, symbol)
TTL trade_date + INTERVAL 12 MONTH DELETE;
```

`a_share_bars_5m_scan` uses the same shape and is built from `a_share_bars_5m`.

If a scan needs full open/high/low values later, add those columns deliberately after measuring query needs. Do not pre-copy every canonical field.

## Retention And Rebuild

Scan tables are disposable derived data:

- keep a short rolling window, such as 3, 6, or 12 months;
- delete older scan data with TTL or explicit partition maintenance;
- rebuild old scan windows from canonical minute tables when needed.

Canonical minute tables remain the long-retention source of truth.

## Refresh Semantics

Refresh is explicit:

```text
marketd refresh-minute-scan --period 1m --since 2026-06-01 --until 2026-06-07
marketd refresh-minute-scan --period 5m --since 2026-06-01 --until 2026-06-07
```

Offline import commands do not refresh scan rows by default:

```text
marketd import-tdx-1m -> writes a_share_bars_1m only
marketd import-tdx-5m -> writes a_share_bars_5m only
```

This keeps imports deterministic and prevents hidden write amplification during large backfills. Operators decide when scan refresh cost is paid.
