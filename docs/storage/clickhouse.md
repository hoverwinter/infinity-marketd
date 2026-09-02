# ClickHouse Storage

本文档是 `infinity-marketd` 的 ClickHouse 存储设计说明。实现以 `internal/clickhouse/schema.go` 为准，本文档用于解释数据库职责、表结构、分区选择和操作约束。

## Safety Rules

`infinity-marketd` 的 assistant workflow 不允许删除 ClickHouse 数据库或表。

禁止执行：

```sql
DROP DATABASE ...
DROP TABLE ...
TRUNCATE TABLE ...
DETACH TABLE ...
```

如果需要重建 schema，只能先给出迁移计划，由人工 operator 决定并手动执行破坏性步骤。优先选择创建新表、导入验证、再人工切换，而不是由 assistant 自动删除旧库表。

## Databases

```text
infinity_market  # market facts and derived market data
infinity_ops     # marketd watermarks, task runs, and data quality issues
```

不要使用 `infinity` 作为 marketd 的目标 database。`configs/config.yaml` 应指向 `infinity_market`。

Mutable securities reference data does not live in ClickHouse. Current security names, aliases, historical names, listing status, and manual corrections are stored in MySQL; see [MySQL Securities Master](security-master-mysql.md). ClickHouse tables remain limited to行情 facts, derived market data, and ops records.

## Configuration

当前配置文件格式：

```yaml
clickhouse:
  user: "default"
  host: "192.168.28.210"
  port: 9000
  passwd: ""
  database: "infinity_market"

mysql:
  enabled: false
  host: "127.0.0.1"
  port: 3306
  database: "infinity_market"
  user: "marketd"
  password: ""
  max_open_conns: 5
  max_idle_conns: 2
  conn_max_lifetime: "5m"

tdx:
  root: "data"

runtime:
  timezone: "Asia/Shanghai"
  batch_size: 10000

logging:
  level: "info"
  encoding: "console"
  output_paths: ["stderr", "file"]
  error_output_paths: ["stderr"]
  file:
    path: "logs/marketd.log"
    max_size_mb: 100
    max_backups: 7
    max_age_days: 30
    compress: true
```

`database` 映射到 market database。ops database 固定默认是 `infinity_ops`，除非通过配置或环境变量显式覆盖。

## Fact Table Rules

Market fact tables are canonical facts:

- No `source`, `source_key`, `source_file`, `version`, or `updated_at` columns.
- No security names, aliases, listing status, or other mutable securities-master fields.
- No cross-row derived metrics in fact tables.
- `marketd` resolves duplicate or conflicting inputs before writing facts.
- Re-imports write by the same logical key and rely on `ReplacingMergeTree` to merge physical duplicates.
- Idempotency must not be implemented by deleting tables or databases.

`pct_chg` is not a `.day` fact. It depends on previous valid close, so it belongs in a derived table or refresh job.

Minute scan data follows the same rule. Local TDX 1-minute and 5-minute imports write only canonical OHLCV facts by default. Time-first scan tables are derived, short-retention, and rebuilt by explicit refresh jobs.

## Tables

### infinity_market.a_share_bars_1d

Canonical A-share daily OHLCV facts.

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

Logical key:

```text
market + symbol + trade_date
```

Partition rationale:

- Daily data is relatively small for ClickHouse.
- A full A-share daily import is about tens of millions of rows, not billions.
- Monthly partitions caused too many active parts during file-by-file imports.
- Year partitions reduce part count while preserving useful historical data management.

Development import verification on 2026-06-06:

```text
rows: 29,024,981
active_parts: 166
has_pct_chg: false
partition: toYear(trade_date)
```

### infinity_market.a_share_bars_1m

Canonical A-share 1-minute OHLCV facts.

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

Logical key:

```text
market + symbol + bar_time
```

Partition rationale:

- 1-minute data is much larger than daily data.
- Month partitioning is still the current working decision for minute bars.
- Canonical imports and single-symbol time-series queries are symbol-first, so the order key remains `(market, symbol, bar_time)`.
- Full-market minute scans use a separate scan-derived table instead of changing this canonical order key.
- The import path must batch by partition and avoid small per-symbol inserts.

### infinity_market.a_share_bars_5m

Canonical A-share 5-minute OHLCV facts.

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

Logical key:

```text
market + symbol + bar_time
```

5-minute bars stay separate from 1-minute bars. Do not combine them into one table with `bar_interval`.

### infinity_market.a_share_financial_raw_items

Canonical raw TDX professional financial item facts from `tdxfin.zip`.

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_financial_raw_items
(
    market LowCardinality(String),
    symbol String,
    report_date Date,
    item_id UInt16,
    value Float64
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(report_date)
ORDER BY (market, symbol, report_date, item_id);
```

Logical key:

```text
market + symbol + report_date + item_id
```

This table stores normalized raw facts only. It does not store `source`, `version`, `updated_at`, announcement-date interpretation, or a derived wide financial row. Dictionary metadata for `item_id` lives in repo metadata and is synchronized to `tdx_financial_item_dictionary`.

### infinity_market.a_share_gp_metric_values

Canonical raw TDX stock trading/metric values from `tdxgp.zip`.

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_gp_metric_values
(
    market LowCardinality(String),
    symbol String,
    metric_type UInt16,
    event_date Date,
    value1 Float64,
    value2 Float64
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(event_date)
ORDER BY (market, symbol, metric_type, event_date);
```

Logical key:

```text
market + symbol + metric_type + event_date
```

This table stores raw `GP01..GP46` series values. It intentionally keeps `value1/value2` narrow and defers semantic widening to explicit downstream jobs.

### infinity_market.tdx_financial_item_dictionary

ClickHouse lookup copy of `internal/tdx/finance/metadata/financial_items.csv`.

```sql
CREATE TABLE IF NOT EXISTS infinity_market.tdx_financial_item_dictionary
(
    item_id UInt16,
    name String,
    title String,
    category LowCardinality(String),
    unit LowCardinality(String),
    value_kind LowCardinality(String),
    source_ref String,
    status LowCardinality(String)
)
ENGINE = ReplacingMergeTree
ORDER BY (item_id);
```

### infinity_market.tdx_gp_metric_dictionary

ClickHouse lookup copy of `internal/tdx/finance/metadata/gp_metrics.csv`.

```sql
CREATE TABLE IF NOT EXISTS infinity_market.tdx_gp_metric_dictionary
(
    metric_type UInt16,
    name String,
    title String,
    value1_meaning String,
    value2_meaning String,
    source_ref String,
    status LowCardinality(String)
)
ENGINE = ReplacingMergeTree
ORDER BY (metric_type);
```

### infinity_market.a_share_intraday_points

Canonical persisted A-share TDX standard HQ minute-time points.

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

This table stores TDX minute-time `price + volume` points. It is not a 1-minute OHLCV table and does not store `open`, `high`, `low`, `close`, or `amount`.

Import commands:

```text
marketd import-tdx-intraday-points --market sh --symbol 600519 --date 2026-06-05
marketd import-tdx-intraday-points --market sh --symbol 600519 --since 2026-06-01 --until 2026-06-05
marketd import-tdx-intraday-points --market sh --symbol 600519 --today
```

Local `import-tdx-1m` does not populate this table. Live `/api/tdx/*` provider reads also remain read-only.

### infinity_market.a_share_bars_1m_scan

Short-retention 1-minute scan data rebuilt from canonical 1-minute facts.

This table is for full-market minute scans such as "all stocks at 10:30 ordered by amount or minute return". It is not the source of truth and should not be written by offline TDX imports.

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

Design constraints:

- Keep the schema narrow. Do not copy full OHLCV unless a measured scan use case needs those columns.
- Keep retention short, for example 3, 6, or 12 months.
- Treat old scan rows as disposable. Rebuild them from `a_share_bars_1m` when needed.
- Generate rows only through an explicit refresh job, never as a hidden side effect of `import-tdx-1m`.

### infinity_market.a_share_bars_5m_scan

Short-retention 5-minute scan data rebuilt from canonical 5-minute facts.

The schema mirrors `a_share_bars_1m_scan`, but rows are rebuilt from `a_share_bars_5m`. It uses:

```sql
PARTITION BY toYYYYMM(trade_date)
ORDER BY (trade_date, bar_time, market, symbol)
```

Generate rows only through an explicit refresh job, never as a hidden side effect of `import-tdx-5m`.

### infinity_market.a_share_daily_derived

Daily derived metrics that are rebuilt from canonical daily facts.

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

`pct_chg` is computed as:

```text
(close - prev_close) / prev_close * 100
```

Use `NULL` when previous close is missing or non-positive.

### infinity_market.a_share_xdxr_events

Normalized TDX xdxr corporate-action events. These rows are refreshed explicitly from TDX HQ and are not OHLCV facts.

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_xdxr_events
(
    market LowCardinality(String),
    symbol String,
    event_date Date,
    category UInt8,
    category_name String,
    fenhong Nullable(Float64),
    peigujia Nullable(Float64),
    songzhuangu Nullable(Float64),
    peigu Nullable(Float64),
    suogu Nullable(Float64),
    panqianliutong Nullable(Float64),
    panhouliutong Nullable(Float64),
    qianzongguben Nullable(Float64),
    houzongguben Nullable(Float64),
    fenshu Nullable(Float64),
    xingquanjia Nullable(Float64)
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(event_date)
ORDER BY (market, symbol, event_date, category);
```

Logical key:

```text
market + symbol + event_date + category
```

### infinity_market.a_share_adjust_factors_1d

Daily qfq/hfq adjustment factors rebuilt from canonical daily bars and `a_share_xdxr_events`.

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_adjust_factors_1d
(
    market LowCardinality(String),
    symbol String,
    trade_date Date,
    qfq_factor Nullable(Float64),
    hfq_factor Nullable(Float64),
    computed_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(computed_at)
PARTITION BY toYear(trade_date)
ORDER BY (market, symbol, trade_date);
```

Adjusted `/api/v1/bars` queries join this table and multiply OHLC by the selected factor. Volume and amount remain raw values. Full adjusted K-line tables are intentionally not created.

### infinity_market.a_share_capital_change_events

Client-local TDX `gbbq` capital-change and corporate-action events.

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_capital_change_events
(
    market LowCardinality(String),
    symbol String,
    event_date Date,
    category UInt8,
    event_seq UInt16,
    event_name LowCardinality(String),
    cash_dividend Nullable(Float64),
    allotment_price Nullable(Float64),
    bonus_shares Nullable(Float64),
    allotment_shares Nullable(Float64),
    shrink_shares Nullable(Float64),
    pre_float_shares Nullable(Float64),
    post_float_shares Nullable(Float64),
    pre_total_shares Nullable(Float64),
    post_total_shares Nullable(Float64),
    ratio_denominator Nullable(Float64),
    exercise_price Nullable(Float64)
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(event_date)
ORDER BY (market, symbol, event_date, category, event_seq);
```

Logical key:

```text
market + symbol + event_date + category + event_seq
```

This table is event input for later adjustment-factor, turnover, and market-cap derived jobs. It is not a bar table and does not store file source columns.

### infinity_market.tdx_block_snapshots

Client-local TDX block snapshot metadata. One snapshot represents normalized block content from one import.

```sql
CREATE TABLE IF NOT EXISTS infinity_market.tdx_block_snapshots
(
    snapshot_id String,
    block_scope LowCardinality(String),
    snapshot_time DateTime64(3, 'Asia/Shanghai'),
    content_hash String,
    block_count UInt32,
    member_count UInt32
)
ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(snapshot_time)
ORDER BY (block_scope, snapshot_time, snapshot_id);
```

Use the latest snapshot per `block_scope` for current membership queries:

```sql
SELECT snapshot_id
FROM infinity_market.tdx_block_snapshots
WHERE block_scope = 'custom'
ORDER BY snapshot_time DESC
LIMIT 1;
```

### infinity_market.tdx_block_definitions

Block definitions for a specific snapshot.

```sql
CREATE TABLE IF NOT EXISTS infinity_market.tdx_block_definitions
(
    snapshot_id String,
    block_scope LowCardinality(String),
    block_kind LowCardinality(String),
    block_id String,
    block_name String,
    block_type UInt16,
    display_order UInt32,
    member_count UInt32
)
ENGINE = ReplacingMergeTree
ORDER BY (snapshot_id, block_scope, block_id);
```

### infinity_market.tdx_block_memberships

Block memberships for a specific snapshot.

```sql
CREATE TABLE IF NOT EXISTS infinity_market.tdx_block_memberships
(
    snapshot_id String,
    block_scope LowCardinality(String),
    block_id String,
    member_order UInt32,
    code String,
    market LowCardinality(String),
    symbol String
)
ENGINE = ReplacingMergeTree
ORDER BY (snapshot_id, block_scope, block_id, market, symbol, member_order);
```

Snapshot storage avoids destructive replacement when a later client-local import removes a member. Consumers choose the snapshot they want.

### infinity_market.tdx_ex_bars_1d

Client-local TDX extension-market daily bars. These use numeric `ex_market` and instrument `code`, not A-share `sh` / `sz` / `bj` symbols.

```sql
CREATE TABLE IF NOT EXISTS infinity_market.tdx_ex_bars_1d
(
    ex_market UInt16,
    code String,
    trade_date Date,
    open Float64,
    high Float64,
    low Float64,
    close Float64,
    position Int64,
    trade Int64,
    price Nullable(Float64),
    amount Nullable(Float64),
    settlement_price Nullable(Float64)
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(trade_date)
ORDER BY (ex_market, code, trade_date);
```

Logical key:

```text
ex_market + code + trade_date
```

### infinity_ops.watermarks

Latest import status for each dataset asset.

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

### infinity_ops.task_runs

Import task history.

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

### infinity_ops.quote_service_runs

Realtime quote sweep run state for the long-running quote service (`marketd quote-serve`). One row per sweep run, replaced in place as the run progresses. Keyed by `run_id`; reads use `FINAL` to collapse versions. Inspect with `marketd quote-status`.

```sql
CREATE TABLE IF NOT EXISTS infinity_ops.quote_service_runs
(
    run_id String,
    status LowCardinality(String),
    markets Array(LowCardinality(String)),
    symbol_source LowCardinality(String),
    batch_size UInt32,
    planned_symbols UInt32,
    planned_batches UInt32,
    succeeded_batches UInt32,
    failed_batches UInt32,
    skipped_batches UInt32,
    rows_fetched UInt64,
    started_at DateTime64(3),
    finished_at Nullable(DateTime64(3)),
    duration_ms Nullable(UInt64),
    error String,
    updated_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(started_at)
ORDER BY (run_id);
```

### infinity_ops.quote_service_batches

Per-batch progress within a sweep run, used for operator visibility and resume. Keyed by `(run_id, batch_no)`; reads use `FINAL`. Not partitioned — batch counts per run are bounded (full-market sweep ≈ a few hundred batches).

```sql
CREATE TABLE IF NOT EXISTS infinity_ops.quote_service_batches
(
    run_id String,
    batch_no UInt32,
    status LowCardinality(String),
    symbol_count UInt32,
    first_symbol String,
    last_symbol String,
    attempts UInt32,
    rows_fetched UInt64,
    started_at Nullable(DateTime64(3)),
    finished_at Nullable(DateTime64(3)),
    duration_ms Nullable(UInt64),
    failure_kind LowCardinality(String),
    error String,
    updated_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (run_id, batch_no);
```

These are **ops records, not market facts** — the quote service never writes realtime snapshots to canonical fact tables. `marketd` reads them back through the shared `Store` (like `marketd status` reads watermarks), not through the `infinity` querier. Both tables are created by `bootstrap` via `CREATE TABLE IF NOT EXISTS`; introducing them is non-destructive and does not touch existing tables.

### infinity_ops.data_quality_issues

Parser and import quality issues.

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

## Import Guidance

Daily bulk imports should group rows by year partition and insert sufficiently large batches. This avoids the two common ClickHouse failure modes seen during development:

- too many partitions in one INSERT block;
- too many active parts caused by many tiny per-symbol inserts.

Minute imports should group by month partition until real 1m/5m volume proves a different strategy is better.

Offline imports are raw data ingestion only:

```text
import-tdx-day -> writes a_share_bars_1d only
import-tdx-1m -> writes a_share_bars_1m only
import-tdx-5m -> writes a_share_bars_5m only
import-tdx-intraday-points -> writes a_share_intraday_points only
import-tdx-fin -> syncs tdx_financial_item_dictionary, writes a_share_financial_raw_items only
import-tdx-gp -> syncs tdx_gp_metric_dictionary, writes a_share_gp_metric_values only
```

Online provider imports are also explicit write-plane operations:

```text
import-tdx-hq-day -> fetches TDX HQ daily K-line pages and writes a_share_bars_1d only
POST /api/console/imports/tdx-hq-day -> immediately runs the same single-symbol import from Console
```

Scan tables are refreshed separately, for example:

```text
marketd refresh-minute-scan --period 1m --since 2026-06-01 --until 2026-06-07
marketd refresh-minute-scan --period 5m --since 2026-06-01 --until 2026-06-07
```

This keeps large offline backfills deterministic and prevents hidden write amplification. Operators decide when to pay the scan refresh cost.

Adjusted bars follow the same explicit refresh rule:

```text
marketd refresh-tdx-xdxr --market sh --symbol 600519
marketd refresh-adjust-factors --market sh --symbol 600519
```

`import-tdx-day`, `import-tdx-hq-day`, `import-tdx-1m`, and `import-tdx-5m` do not fetch xdxr events and do not refresh qfq/hfq factors as hidden side effects.

Financial wide tables follow the same explicit refresh rule. `import-tdx-fin` and `import-tdx-gp` do not generate wide financial snapshots, factors, or scan tables as hidden side effects.

Existing deployments can add the two new tables non-destructively by running bootstrap. Then backfill in order: raw daily bars, xdxr events, adjustment factors. Rollback is non-destructive: stop using `adjust=qfq|hfq`; canonical OHLCV tables are unchanged.

## Current Open Questions

- Whether derived daily refresh should be a standalone CLI command or scheduled job.
- Which scan metrics justify storage beyond `close`, `volume`, `amount`, `minute_ret`, and `volume_ratio`.
- Whether market fact numeric prices should remain `Float64` or move to fixed decimal types after downstream query needs are clearer.
