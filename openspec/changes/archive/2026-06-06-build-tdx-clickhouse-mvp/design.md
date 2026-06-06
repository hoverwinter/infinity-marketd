## Principles

This change follows the repository reasoning principles:

- Occam's Razor: implement only the current local TDX inputs and the tables needed to store them.
- First Principles: derive tables from actual data shapes and query patterns.
- Socratic Questioning: keep uncertain future needs out of the schema until validated.
- Feynman Principle: if an operator cannot explain a table in one sentence, the design is too complicated.

## Data Model

### Canonical Facts

ClickHouse market tables are canonical fact tables. For one logical key, there is one market fact.

The schema does not model `source`, `source_key`, `version`, or `updated_at` in market fact tables. `infinity-marketd` resolves competing inputs before writing facts.

### Physical Tables By Period

Daily, 1-minute, and 5-minute OHLCV bars use separate physical tables:

```text
infinity_market.a_share_bars_1d
infinity_market.a_share_bars_1m
infinity_market.a_share_bars_5m
```

This keeps queries obvious:

```text
daily query  -> a_share_bars_1d
1m query     -> a_share_bars_1m
5m query     -> a_share_bars_5m
```

Code may share one internal `Bar` model and writer helper, but ClickHouse tables remain separated by period.

## ClickHouse Schema

### Databases

```sql
CREATE DATABASE IF NOT EXISTS infinity_market;
CREATE DATABASE IF NOT EXISTS infinity_ops;
```

### Daily Bars

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

### 1-Minute Bars

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_bars_1m
(
    market LowCardinality(String),
    symbol String,
    bar_time DateTime('Asia/Shanghai'),
    trade_date Date,
    open Float64,
    high Float64,
    low Float64,
    close Float64,
    volume UInt64,
    amount Float64
)
ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(trade_date)
ORDER BY (market, symbol, bar_time);
```

### Daily Derived Metrics

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

`pct_chg` is derived from previous valid close and is not part of the canonical daily fact table.

### 5-Minute Bars

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_bars_5m
(
    market LowCardinality(String),
    symbol String,
    bar_time DateTime('Asia/Shanghai'),
    trade_date Date,
    open Float64,
    high Float64,
    low Float64,
    close Float64,
    volume UInt64,
    amount Float64
)
ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(trade_date)
ORDER BY (market, symbol, bar_time);
```

### Watermarks

```sql
CREATE TABLE IF NOT EXISTS infinity_ops.watermarks
(
    dataset LowCardinality(String),
    asset LowCardinality(String),
    status LowCardinality(String),
    min_watermark Nullable(DateTime64(3)),
    max_watermark Nullable(DateTime64(3)),
    rows_written UInt64,
    message String,
    updated_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (dataset, asset);
```

### Task Runs

```sql
CREATE TABLE IF NOT EXISTS infinity_ops.task_runs
(
    run_id String,
    dataset LowCardinality(String),
    task_type LowCardinality(String),
    status LowCardinality(String),
    target_table LowCardinality(String),
    input_path String,
    input_format LowCardinality(String),
    params String,
    started_at DateTime64(3),
    finished_at Nullable(DateTime64(3)),
    duration_ms Nullable(UInt64),
    rows_written UInt64,
    rows_skipped UInt64,
    error String,
    updated_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(started_at)
ORDER BY (dataset, started_at, run_id);
```

### Data Quality Issues

```sql
CREATE TABLE IF NOT EXISTS infinity_ops.data_quality_issues
(
    issue_id String,
    run_id String,
    dataset LowCardinality(String),
    severity LowCardinality(String),
    issue_type LowCardinality(String),
    market Nullable(String),
    symbol Nullable(String),
    logical_key String,
    input_path String,
    input_record_offset Nullable(UInt64),
    observed_at DateTime64(3),
    message String,
    details String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(observed_at)
ORDER BY (dataset, observed_at, severity, issue_type);
```

## Replacement Semantics

Fact tables use `ReplacingMergeTree` without a version column, matching the existing simple `lightracer.trade_info` pattern.

Marketd must not rely on ClickHouse to choose between competing facts. Before insertion, marketd must:

- normalize records into logical keys,
- detect duplicate logical keys inside a batch,
- record a quality issue when the same logical key has conflicting values,
- write only one resolved fact per logical key.

Repeated imports of the same input should produce the same logical facts.

## TDX Input Formats

### `.day`

Records are 32 bytes, little-endian, unpacked as:

```text
<IIIIIfII
```

Prices are integer cents and are normalized by dividing by `100.0`.

### `.lc1` / `.lc5`

Records are 32 bytes, little-endian, unpacked as:

```text
<HHfffffII
```

The date is 16-bit packed date. The time is minutes since midnight. Prices are float32 values.

### `.1` / `.5`

Records are 32 bytes, little-endian, unpacked as:

```text
<HHIIIIfII
```

Prices are integer cents and are normalized by dividing by `100.0`.

## Time Handling

All local minute records are decoded directly into `Asia/Shanghai` trading time. The implementation must not introduce the +8 hour offset seen in the historical `lightracer.trade_info` data.

## Configuration

Configuration precedence:

```text
CLI flags > environment variables > config file > defaults
```

Required settings:

- ClickHouse address/user/password.
- ClickHouse market database name, default `infinity_market`.
- ClickHouse ops database name, default `infinity_ops`.
- TDX root path.
- Runtime timezone, default `Asia/Shanghai`.
- Batch size.

## Open Questions

- The first implementation stores no `pct_chg` in daily facts. Derived refresh for `a_share_daily_derived` can be added after the canonical import is stable.
- Whether 5-minute import should share parser code with 1-minute import through a period parameter or use a separate command handler that calls shared lower-level code.
