# infinity-marketd 项目输入文档

## 项目定位

`infinity-marketd` 是一个独立的 Go 行情数据守护进程，负责把本地或远程市场行情源解析、标准化并写入 ClickHouse。

它不是 Infinity 主应用的一部分，也不应该反向依赖 Infinity 的业务代码。Infinity 通过 ClickHouse 表、HTTP API 或后续可选的控制接口消费它的输出。

一句话定位：

```text
infinity-marketd = market data daemon + ClickHouse writer + watermark/quality control
```

首期目标聚焦 A 股 TDX 数据：

```text
TDX 本地 .day / .lc1 / .lc5 / .1 / .5
  -> Go parser
  -> normalized bars
  -> ClickHouse
  -> watermarks / quality issues
```

## 设计动机

Infinity 当前的数据底座混合了 Python 采集、Parquet、JSON 缓存和业务模块本地文件。这个阶段已经能支撑页面迭代，但不适合作为长期量化研究和全市场扫描底座。

新底座要解决的问题：

- 历史日 K、1 分钟 K、分时、快照必须有统一主存储。
- 全市场分钟级扫描需要数据库查询能力，而不是散读文件。
- 数据写入需要水位、幂等、质量诊断和可追责任务。
- 高频行情采集不应由 Python gateway 或业务 API 请求临时触发。
- TDX / Futu / crypto 等结构化行情源应有统一数据面。

## 与 Infinity 的边界

```text
┌──────────────────────┐
│      front-v2         │
│  charts / tables      │
└──────────┬───────────┘
           │ HTTP / WS
┌──────────▼───────────┐
│  Infinity gateway     │
│  product API layer    │
└──────────┬───────────┘
           │ SQL / API
┌──────────▼───────────┐
│      ClickHouse       │
│ primary market store  │
└──────────▲───────────┘
           │ batch insert / realtime writes
┌──────────┴───────────┐
│  infinity-marketd     │
│ market data plane     │
└──────────────────────┘
```

职责划分：

| 组件 | 职责 |
| --- | --- |
| `infinity-marketd` | 结构化行情采集、解析、写 ClickHouse、水位、质量诊断、后续实时行情推送 |
| Infinity gateway | 产品 API、查询编排、权限、页面数据聚合 |
| Python dagent | 公告、研报、OCR、社区、宏观、复盘、文本/事件/enrichment 数据 |
| ClickHouse | 行情、信号、数据质量、水位和运行状态的主查询层 |
| front-v2 | 展示、操作、图表、监控 |

关键原则：

- `infinity-marketd` 不理解 ST 复盘、策略页面、Agent、业务 UI。
- Infinity 不直接解析 TDX 原始行情文件。
- gateway 不在 HTTP 请求中执行重采集或大文件解析。
- ClickHouse 是结构化行情主存储。

## 范围

### 首期范围

- Go CLI / daemon 项目骨架。
- ClickHouse schema bootstrap。
- TDX 本地 `.day` 日线解析。
- TDX 本地 `.lc1` / `.1` 1 分钟线解析。
- TDX 本地 `.lc5` / `.5` 5 分钟线解析。
- 批量写入 ClickHouse。
- watermark。
- task run 记录。
- data quality issue 记录。
- dry-run / sample import / status 命令。

### 后续范围

- TDX 远程行情 TCP 请求/响应客户端。
- 盘中实时快照、分时、1 分钟 K 增量更新。
- 内部 WebSocket/SSE 推送。
- Futu 行情源。
- crypto OHLCV / ticker / funding rate。
- 回放、补数、校验、重算任务。
- 可选 HTTP control plane。

### 非目标

首期不做：

- Infinity gateway 查询迁移。
- front-v2 页面改造。
- 策略信号计算。
- TDX 远程实时协议。
- Agent runtime 集成。
- 文本、公告、研报、OCR 数据采集。

## 建议项目结构

```text
infinity-marketd/
  cmd/
    marketd/
      main.go
  internal/
    config/
    clickhouse/
    schema/
    model/
    tdx/
      localday/
      localminute/
      market.go
    ingest/
    watermark/
    quality/
    scheduler/
    log/
  docs/
    clickhouse-schema.md
    tdx-local-files.md
    operations.md
  examples/
    docker-compose.yml
    config.example.yaml
  migrations/
  README.md
  LICENSE
  go.mod
```

## 命令设计

首期 CLI：

```bash
marketd bootstrap
marketd status
marketd import-tdx-day --root ~/tdx-data --code 600519
marketd import-tdx-day --file ~/tdx-data/vipdoc/sh/lday/sh600519.day
marketd import-tdx-1m --root ~/tdx-data --code 600519 --since 2026-01-01
marketd import-tdx-1m --file ~/tdx-data/vipdoc/sh/minline/sh600519.lc1
marketd import-tdx-5m --root ~/tdx-data --code 600519 --since 2026-01-01
marketd import-tdx-5m --file ~/tdx-data/vipdoc/sh/fzline/sh600519.lc5
```

建议通用参数：

```text
--clickhouse-url
--clickhouse-market-db
--clickhouse-ops-db
--clickhouse-user
--clickhouse-password
--config
--dry-run
--batch-size
--since
--until
--code
--market
--file
--root
```

## 配置设计

环境变量：

```text
MARKETD_CLICKHOUSE_ADDR
MARKETD_CLICKHOUSE_MARKET_DB
MARKETD_CLICKHOUSE_OPS_DB
MARKETD_CLICKHOUSE_USER
MARKETD_CLICKHOUSE_PASSWORD
MARKETD_TDX_ROOT
MARKETD_BATCH_SIZE
MARKETD_TIMEZONE
```

配置文件示例：

```yaml
clickhouse:
  addr: "127.0.0.1:9000"
  user: "default"
  password: ""
  databases:
    market: "infinity_market"
    ops: "infinity_ops"

tdx:
  root: "~/tdx-data"

runtime:
  timezone: "Asia/Shanghai"
  batch_size: 10000
```

配置优先级：

```text
CLI flags > environment variables > config file > defaults
```

仓库只提交 `config.example.yaml`。真实 `config.yaml`、密码、API key 不提交到仓库；生产环境优先通过环境变量或 secret manager 注入。

## ClickHouse Schema 草案

ClickHouse database 按职责拆分：

```text
infinity_market  # 行情主数据
infinity_ops     # marketd 运行状态、水位、质量问题
```

### ClickHouse 表设计原则

1. 行情表是 canonical fact table。对业务查询来说，同一个逻辑键只能有一份市场事实。
2. ClickHouse schema 不建模 source。`infinity-marketd` 是唯一事实生产者；不同输入之间的冲突由 marketd 内部 resolver 解决，解决后的结果才写入行情表。
3. 不同周期使用不同物理表。即使日线、1 分钟、5 分钟都是 OHLCV，也不要用 `bar_interval` 混在一张 ClickHouse 表里。
4. 代码层可以复用统一的 Bar model、parser output 和 writer helper；ClickHouse 层必须按查询形态和数据生命周期拆表。
5. 日线查询只查 `a_share_bars_1d`，分钟线查询只查 `a_share_bars_1m` / `a_share_bars_5m`。不要让用户为了查一天的数据去过滤分钟线数据。
6. 分时线不是 1 分钟 K。`price + volume` point 写入 `a_share_intraday_points`，不能写入任何 `a_share_bars_*` 表。
7. 重复导入、补数、修正数据通过同一逻辑键和 `ReplacingMergeTree` 收敛，不通过导入前删除实现幂等。

行情表按市场/资产域拆分。首期只支持 A 股，因此只创建 `a_share_*` 表；表内 `market` 字段继续表示 A 股交易所/市场分段，例如 `sh`、`sz`、`bj`。

不同输入进入 marketd 后，必须先按数据质量、时间新鲜度和配置规则收敛成唯一事实，再写入行情表。

后续新增市场时，不把不同市场混写到 A 股表，而是新增独立表：

```text
infinity_market.hk_stock_bars_1d
infinity_market.hk_stock_bars_1m
infinity_market.us_stock_bars_1d
infinity_market.us_stock_bars_1m
infinity_market.crypto_bars_1m
infinity_market.crypto_funding_rates
```

### infinity_market.a_share_bars_1d

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_bars_1d
(
    market Enum8('sh' = 1, 'sz' = 2, 'bj' = 3),
    symbol FixedString(6),
    trade_date Date,
    open Decimal64(4),
    high Decimal64(4),
    low Decimal64(4),
    close Decimal64(4),
    volume UInt64,
    amount Float64
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(trade_date)
ORDER BY (market, symbol, trade_date);
```

承载 A 股日线 OHLCV。TDX `.day` 价格从整数分转为元；`trade_date` 是记录日期。日线事实表不存 `pct_chg`，因为它依赖上一条有效收盘价，是跨行派生指标。

### infinity_market.a_share_daily_derived

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_daily_derived
(
    market Enum8('sh' = 1, 'sz' = 2, 'bj' = 3),
    symbol FixedString(6),
    trade_date Date,
    prev_close Nullable(Decimal64(4)),
    pct_chg Nullable(Float64),
    computed_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(computed_at)
PARTITION BY toYear(trade_date)
ORDER BY (trade_date, market, symbol);
```

承载日线派生指标。`prev_close` / `pct_chg` 可按日期范围或全量历史重算，不进入 canonical OHLCV facts。

### infinity_market.a_share_bars_1m

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_bars_1m
(
    market Enum8('sh' = 1, 'sz' = 2, 'bj' = 3),
    symbol FixedString(6),
    bar_time DateTime('Asia/Shanghai'),
    trade_date Date,
    open Decimal64(4),
    high Decimal64(4),
    low Decimal64(4),
    close Decimal64(4),
    volume UInt64,
    amount Float64
)
ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(trade_date)
ORDER BY (market, symbol, bar_time);
```

承载 A 股 1 分钟 OHLCV。TDX `.lc1` 使用 float32 price；兼容 `.1` 使用整数分价格，写入前统一归一化为 `Decimal64(4)`。

该表是 canonical fact table。它按 `(market, symbol, bar_time)` 排序，优先服务单股时间序列查询、文件级导入/重导、连续性校验和回测读取。全市场分钟扫描不要通过改变该表排序键来解决，应使用短保留 scan 派生表。

### infinity_market.a_share_bars_5m

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_bars_5m
(
    market Enum8('sh' = 1, 'sz' = 2, 'bj' = 3),
    symbol FixedString(6),
    bar_time DateTime('Asia/Shanghai'),
    trade_date Date,
    open Decimal64(4),
    high Decimal64(4),
    low Decimal64(4),
    close Decimal64(4),
    volume UInt64,
    amount Float64
)
ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(trade_date)
ORDER BY (market, symbol, bar_time);
```

承载 A 股 5 分钟 OHLCV。首期支持 `.lc5` / `.5`，但不要把 5 分钟线混入 1 分钟表。

分钟线使用 `toYYYYMM(trade_date)` 月分区。分钟线数据量远大于日线，月分区便于回补、分区维护和冷热管理；按天分区会产生过多分区，按年分区会让 1 分钟单分区过大。

同一表内的逻辑键只允许存在一份事实：

```text
a_share_bars_1d: market + symbol + trade_date
a_share_bars_1m: market + symbol + bar_time
a_share_bars_5m: market + symbol + bar_time
```

重复导入、补数、修正数据时按同一逻辑键写入新事实，由 `ReplacingMergeTree` 收敛。首期不引入显式 version。

### infinity_market.a_share_bars_1m_scan / a_share_bars_5m_scan

分钟 scan 表是短保留、少列、可重建的派生层，不是事实源。它们用于全市场分钟截面扫描，例如某一分钟按成交额、涨速、量比过滤排序。

建议形态：

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_bars_1m_scan
(
    trade_date Date,
    bar_time DateTime('Asia/Shanghai'),
    market Enum8('sh' = 1, 'sz' = 2, 'bj' = 3),
    symbol FixedString(6),
    close Decimal64(4),
    volume UInt64,
    amount Float64,
    prev_close Nullable(Decimal64(4)),
    minute_ret Nullable(Float64),
    volume_ratio Nullable(Float64),
    computed_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(computed_at)
PARTITION BY toYYYYMM(trade_date)
ORDER BY (trade_date, bar_time, market, symbol)
TTL trade_date + INTERVAL 12 MONTH DELETE;
```

`a_share_bars_5m_scan` 使用同样结构，从 `a_share_bars_5m` 重建。

设计约束：

1. 离线原始数据导入默认只写 `a_share_bars_1m` / `a_share_bars_5m`，不生成 scan 数据。
2. scan 表只保存扫描必要列和少量派生指标，不完整复制所有 OHLCV 历史。
3. scan 表保留短窗口，例如 3-12 个月；过期数据可删除，需要时从基础表重建。
4. scan 刷新由显式命令或调度任务触发，例如 `refresh-minute-scan --period 1m --since ... --until ...`。

### infinity_market.a_share_intraday_points

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_intraday_points
(
    market Enum8('sh' = 1, 'sz' = 2, 'bj' = 3),
    symbol FixedString(6),
    trade_date Date,
    point_time DateTime('Asia/Shanghai'),
    point_index UInt16,
    price Decimal64(4),
    volume UInt64,
    amount Nullable(Float64)
)
ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(trade_date)
ORDER BY (market, symbol, trade_date, point_time);
```

用于 TDX 分时线，不等同于 1 分钟 OHLCV。分时线是 `price + volume` point，不能写入任何 `a_share_bars_*` 表。

### infinity_market.a_share_quote_snapshots

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_quote_snapshots
(
    market Enum8('sh' = 1, 'sz' = 2, 'bj' = 3),
    symbol FixedString(6),
    quote_time DateTime64(3, 'Asia/Shanghai'),
    trade_date Date,
    price Decimal64(4),
    prev_close Decimal64(4),
    open Decimal64(4),
    high Decimal64(4),
    low Decimal64(4),
    volume UInt64,
    current_volume UInt64,
    amount Float64,
    bid_prices Array(Decimal64(4)),
    ask_prices Array(Decimal64(4)),
    bid_volumes Array(UInt64),
    ask_volumes Array(UInt64)
)
ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(trade_date)
ORDER BY (market, symbol, quote_time);
```

### infinity_ops.watermarks

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

### infinity_ops.data_quality_issues

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

## TDX 本地文件解析

### 日线 `.day`

TDX `.day` 文件每条记录 32 bytes。

解码目标字段：

```text
date
open
high
low
close
amount
volume
reserved
```

归一化：

- 价格从整数分转为元。
- 日期转为 `Date`。
- market 优先从路径 `sh/sz/bj` 判断。
- 代码从文件名 `sh600519.day` / `sz000001.day` / `bj920002.day` 判断。

### 1 分钟 `.lc1`

TDX `.lc1` 文件每条记录 32 bytes，首期支持已验证布局。

解码目标字段：

```text
date
time
open
high
low
close
amount
volume
```

归一化：

- timestamp 使用 `Asia/Shanghai`。
- `trade_date` 从 timestamp 派生。
- market/code 从路径或文件名判断。

### 市场判断规则

文件位置优先：

```text
vipdoc/sh/... -> sh
vipdoc/sz/... -> sz
vipdoc/bj/... -> bj
```

代码 fallback：

```text
920* / 8* / 4* -> bj
6* / 9*        -> sh
其他常见 A 股  -> sz
```

`920002` 不得因为 `9` 开头被误归为上交所。

## 写入语义

采用逻辑幂等，而不是导入前删除。

```text
a_share_bars_1d 逻辑键：market + symbol + trade_date
a_share_bars_1m 逻辑键：market + symbol + bar_time
a_share_bars_5m 逻辑键：market + symbol + bar_time
分时点逻辑键：market + symbol + trade_date + point_time
快照逻辑键：market + symbol + quote_time
```

ClickHouse 表使用：

```text
ReplacingMergeTree
```

marketd 写入行情表前必须先完成事实决策。同一逻辑键只能写入一份事实，不允许把不同输入的多个版本同时写入行情表。

重复导入、补数或修正时，同一逻辑键的新事实由 `ReplacingMergeTree` 收敛。物理重复行由 ClickHouse 后台合并处理；需要强一致诊断时可以使用 `FINAL`。

首期不引入 source 和 version 作为 schema 概念。marketd 负责在写入前解决输入冲突；运行和质量信息只记录在 `infinity_ops`。

## 数据质量

marketd 必须记录以下问题：

- 文件缺失。
- 不支持的文件格式。
- incomplete trailing bytes。
- invalid date/time。
- zero valid rows。
- duplicate logical key。
- import partial failure。
- ClickHouse write failure。

质量问题进入 `infinity_ops.data_quality_issues`，不要只写日志。

## 水位

每次导入后更新：

```text
dataset
asset
status
min_watermark
max_watermark
rows_written
message
updated_at
```

示例 dataset：

```text
a_share_bars_1d
a_share_bars_1m
a_share_bars_5m
a_share_intraday_points
a_share_quote_snapshots
```

## 后续实时能力设计

TDX 标准行情公开协议更接近请求/响应，不应假设服务端主动推送。

后续实时形态：

```text
TDX TCP server
  -> marketd long-lived polling
  -> ClickHouse writes
  -> optional WebSocket/SSE to clients
```

建议调度：

```text
盘中重点池：
  quotes 每 1-3 秒
  intraday minutes 每 5-10 秒

1 分钟 K：
  每分钟收盘后增量写

盘后：
  15:10 / 15:30 校准全天数据
```

不要一开始全市场每秒轮询。全市场分钟线适合盘后或低频批量补齐。

## 开源边界

`infinity-marketd` 适合作为独立开源项目。

应该包含：

- 通用行情数据模型。
- TDX 本地文件解析。
- TDX 远程行情客户端。
- ClickHouse schema 和 writer。
- 调度、水位、质量诊断。
- 示例配置和部署。

不应该包含：

- Infinity gateway。
- Infinity front-v2。
- ST 复盘语义。
- 具体策略信号。
- Agent runtime。
- 私有数据、账号、API Key。

## MVP 验收标准

第一版可用的最低标准：

1. `marketd bootstrap` 能初始化 ClickHouse schema。
2. `marketd import-tdx-day --code 600519` 能导入日 K。
3. `marketd import-tdx-1m --code 600519` 能导入 1 分钟 K。
4. `marketd import-tdx-5m --code 600519` 能导入 5 分钟 K。
5. 重复导入同一文件后，逻辑结果稳定。
6. `infinity_ops.watermarks` 能看到最新水位。
7. 解析异常能进入 `infinity_ops.data_quality_issues`。
8. 不依赖 pandas / mootdx / Python。
9. Infinity 可以直接用 SQL 查询导入结果。

## 与 Infinity 主仓库的关系

Infinity 主仓库后续只需要：

- 记录 `infinity-marketd` 集成文档。
- gateway 增加 ClickHouse 查询适配。
- front-v2 逐步迁移数据页面。
- dagent 规范里承认 `marketd` 是结构化行情数据 owner。

Infinity 不应该复制 marketd 的解析代码。

## 推荐开发顺序

```text
1. 新建 Go 项目骨架
2. ClickHouse config + bootstrap
3. model 定义
4. TDX .day parser + fixtures
5. daily batch writer
6. watermarks / task_runs / quality_issues
7. TDX .lc1 / .1 parser + fixtures
8. 1m batch writer
9. TDX .lc5 / .5 parser + fixtures
10. 5m batch writer
11. status / dry-run / sample validation
12. 文档和 release
```

## 初始 README 摘要

可以用下面这段作为新仓库 README 开头：

```markdown
# infinity-marketd

`infinity-marketd` is a Go market data daemon for importing, normalizing, and writing structured market data into ClickHouse.

The first supported input format is local TongDaXin data:

- `.day` daily bars
- `.lc1` 1-minute bars
- `.lc5` 5-minute bars

It provides ClickHouse schema bootstrap, idempotent batch writes, watermarks, task run records, and data quality diagnostics.

The project is designed to run independently from Infinity. Infinity consumes the resulting ClickHouse tables through its gateway and research workflows.
```
