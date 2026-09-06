# Querier API Reference

本文档描述当前 `infinity querier serve` 提供的 HTTP API。

查询服务负责封装行情查询语义和 ClickHouse 读取逻辑。调用方可以是 `infinity querier` CLI、控制台、Agent、研究脚本或其他服务；调用方不应直接依赖 ClickHouse 表结构。

默认服务地址示例：

```text
http://127.0.0.1:8808
```

启动示例：

```bash
go run ./cmd/infinity querier serve --config examples/config.example.yaml --listen 127.0.0.1:8808
```

## 通用约定

所有响应默认使用 JSON：

```http
Content-Type: application/json
```

当前 API 前缀：

```text
/api/v1
```

## API Namespace Model

`/api/v1/...` 是产品级查询 API，面向 ClickHouse-backed canonical market data 和 MySQL-backed mutable reference data。调用方可以把这些接口视为稳定、可重复读取的查询面。

TDX 在线协议能力不放在 `/api/v1` 下。后续 TDX provider/protocol API 设计为：

```text
/api/tdx/hq/...
/api/tdx/exhq/...
```

边界约定：

| Namespace | Owner | Data source | Contract |
| --- | --- | --- | --- |
| `/api/v1/...` | querier product/query API | ClickHouse facts、MySQL securities master 或稳定内部状态 | 稳定查询、可重复读取、不发起 live TDX 请求 |
| `/api/tdx/hq/...` | TDX standard行情 provider API | live TDX HQ upstream | 请求/响应式在线读取，可能超时或受上游影响 |
| `/api/tdx/exhq/...` | TDX extended行情 provider API | live TDX ExHQ upstream | 扩展市场协议读取，使用 numeric market id 和 instrument code |

`/api/tdx/...` 不应隐式写入 ClickHouse。实时快照持久化、保留周期、去重和查询模型需要单独 storage contract。

TDX provider API 的详细 endpoint contract 见 [tdx.md](tdx.md)。

按数据产品统一的多来源在线读取位于 `/api/providers/...`，默认注册 TDX、THS、Eastmoney 三源：TDX 证券/指数 K 线，以及 THS/Eastmoney 板块指数日线、目录和代码解析。该入口显式选择来源，不访问事实存储或隐式导入；接口、来源内身份、能力范围和实际联网验证状态见 [providers.md](providers.md)。

错误响应格式：

```json
{
  "error": "market must be sh, sz, or bj"
}
```

常见状态码：

| Status | Meaning |
| ---: | --- |
| `200` | 请求成功 |
| `400` | 参数错误 |
| `503` | 后端存储或依赖不可用 |
| `500` | 未预期服务错误 |

## 时间语义

日 K 使用交易日期：

```text
trade_date = YYYY-MM-DD
```

分钟 K 使用 `Asia/Shanghai` 时间：

```text
bar_time = Asia/Shanghai datetime
```

分时点使用同一时区：

```text
point_time = Asia/Shanghai datetime
```

`since` 和 `until` 都是包含式。

日 K：

```text
trade_date >= since
trade_date <= until
```

分钟 K：

```text
bar_time >= since
bar_time <= until
```

如果分钟 K 的 `until` 只传日期，例如 `until=2026-01-01`，服务会包含整个交易日期，实际查询边界等价于：

```text
bar_time < 2026-01-02 00:00:00
```

所有 bar 返回都按时间升序：

```text
1d: trade_date ASC
1m/5m: bar_time ASC
```

`limit` 表示最大返回条数，默认 `1000`，最大 `10000`。

当没有 `since` / `until` 时：

```text
limit = 最近 N 根 bar
返回结果仍按时间升序
```

当存在 `since` 或 `until` 时：

```text
limit = 区间内最多返回 N 根 bar
返回结果按时间升序
```

## GET /api/v1/health

健康检查。

### Request

```http
GET /api/v1/health
```

### Response

```json
{
  "status": "ok",
  "version": "0.1.0",
  "schema_version": "2026-09-05"
}
```

调用方应至少保留对 `status` 字段的兼容。`version` 和 `schema_version` 可用于控制台展示和客户端兼容性检查。

### Failure

当 ClickHouse 或底层 repository 不可用时：

```http
HTTP/1.1 503 Service Unavailable
```

```json
{
  "error": "clickhouse: connection refused"
}
```

## GET /api/v1/securities

查询单只证券当前主数据。该接口读取 MySQL securities master。

### Request

```http
GET /api/v1/securities?market=sh&symbol=600519
```

### Query Parameters

| Name | Required | Description |
| --- | --- | --- |
| `market` | yes | 市场，取值 `sh` / `sz` / `bj` |
| `symbol` | yes | 6 位证券代码，例如 `600519` |

### Response

```json
{
  "market": "sh",
  "symbol": "600519",
  "exchange": "SSE",
  "current_name": "贵州茅台",
  "current_name_norm": "贵州茅台",
  "board": "main",
  "status": "listed",
  "listing_date": "2001-08-27",
  "lot_size": 100,
  "price_precision": 2,
  "source": "tdx",
  "manual_locked": false
}
```

### Failure

不存在时返回 `404 Not Found`。MySQL 未配置或不可用时返回 `503 Service Unavailable`。

## GET /api/v1/securities/resolve

按代码、当前名称、历史名称或 alias 搜索证券主数据。该接口只返回候选，不替调用方解决同名歧义。

### Request

```http
GET /api/v1/securities/resolve?q=贵州茅台
```

### Query Parameters

| Name | Required | Description |
| --- | --- | --- |
| `q` | yes | 查询文本，可以是 6 位代码、当前名称、历史名称或 alias |
| `limit` | no | 最大候选数，默认 `20`，最大 `100` |

### Response

```json
{
  "q": "贵州茅台",
  "candidates": [
    {
      "security": {
        "market": "sh",
        "symbol": "600519",
        "exchange": "SSE",
        "current_name": "贵州茅台",
        "board": "main",
        "status": "listed",
        "source": "tdx",
        "manual_locked": false
      },
      "match_type": "current_name",
      "matched_text": "贵州茅台",
      "score": 90
    }
  ]
}
```

如果同名或 alias 匹配多个证券，响应会保留多个候选；业务层自行选择或提示用户确认。

## GET /api/v1/bars

查询单只 A 股 OHLCV K 线。

当前支持：

```text
period=1d
period=1m
period=5m
```

### Request

```http
GET /api/v1/bars?market=sh&symbol=600519&period=1d&since=2024-01-01&until=2024-12-31&limit=1000
```

### Query Parameters

| Name | Required | Description |
| --- | --- | --- |
| `market` | yes | 市场，取值 `sh` / `sz` / `bj` |
| `symbol` | yes | 6 位证券代码，例如 `600519` |
| `period` | no | 周期，取值 `1d` / `1m` / `5m`，默认 `1d` |
| `adjust` | no | 复权模式，取值 `none` / `qfq` / `hfq`，默认 `none` |
| `since` | no | 起始日期或时间，包含式 |
| `until` | no | 截止日期或时间，包含式 |
| `limit` | no | 最大返回条数，默认 `1000`，最大 `10000` |

### Adjustment Semantics

`adjust=none` 返回 canonical raw OHLCV。

`adjust=qfq` / `adjust=hfq` 读取已落库的日级复权因子并调整 OHLC：

```text
adjusted_open  = raw_open  * factor
adjusted_high  = raw_high  * factor
adjusted_low   = raw_low   * factor
adjusted_close = raw_close * factor
```

分钟 K 使用同一 `trade_date` 的日级因子。`volume` 和 `amount` 保持原始值，不随复权因子缩放。复权价格是分析价格，不是交易所真实成交价。

`/api/v1/bars` 不会在请求路径里连接 live TDX server。使用复权查询前，operator 需要先刷新：

`/api/v1/bars` 也不会查询 MySQL securities master，不返回 joined security name。需要名称或 alias 时，调用方应单独请求 `/api/v1/securities` 或 `/api/v1/securities/resolve`。

```bash
marketd refresh-tdx-xdxr --market sh --symbol 600519
marketd refresh-adjust-factors --market sh --symbol 600519
```

如果请求区间内缺少所需复权因子，服务返回 `409 Conflict`，不会在同一个成功响应中混合 raw 和 adjusted OHLC。

### Date And Datetime Format

`period=1d`：

```text
since=YYYY-MM-DD
until=YYYY-MM-DD
```

`period=1m` / `period=5m`：

```text
since=YYYY-MM-DD
since=YYYY-MM-DD HH:mm:ss
since=YYYY-MM-DDTHH:mm:ss
since=RFC3339
```

`until` 支持同样格式。分钟 K 的 date-only `until` 会包含整天。

### Daily Response

```json
{
  "query": {
    "market": "sh",
    "symbol": "600519",
    "period": "1d",
    "adjust": "none",
    "since": "2024-01-01",
    "until": "2024-12-31",
    "limit": 1000
  },
  "bars": [
    {
      "market": "sh",
      "symbol": "600519",
      "period": "1d",
      "trade_date": "2024-01-02",
      "open": 1700.0,
      "high": 1720.0,
      "low": 1688.0,
      "close": 1710.0,
      "volume": 123456,
      "amount": 1234567890.0
    }
  ]
}
```

Daily bar fields:

| Field | Type | Description |
| --- | --- | --- |
| `market` | string | `sh` / `sz` / `bj` |
| `symbol` | string | 6 位代码 |
| `period` | string | `1d` |
| `trade_date` | string | `YYYY-MM-DD` |
| `open` | number | 开盘价 |
| `high` | number | 最高价 |
| `low` | number | 最低价 |
| `close` | number | 收盘价 |
| `volume` | integer | 成交量 |
| `amount` | number | 成交额 |

当前 `/api/v1/bars` 不返回 `pct_chg`。涨跌幅属于派生数据，调用方可按前收自行计算。

### Minute Response

`period=1m` 或 `period=5m` 时，bar 会包含 `bar_time`：

```json
{
  "query": {
    "market": "sh",
    "symbol": "600519",
    "period": "1m",
    "adjust": "none",
    "since": "2026-01-01 09:30:00",
    "until": "2026-01-01 15:00:00",
    "limit": 10000
  },
  "bars": [
    {
      "market": "sh",
      "symbol": "600519",
      "period": "1m",
      "trade_date": "2026-01-01",
      "bar_time": "2026-01-01T09:31:00+08:00",
      "open": 1700.0,
      "high": 1701.0,
      "low": 1699.0,
      "close": 1700.5,
      "volume": 1200,
      "amount": 2040000.0
    }
  ]
}
```

Minute bar fields include all daily fields plus:

| Field | Type | Description |
| --- | --- | --- |
| `bar_time` | string | RFC3339 datetime with timezone |

### Empty Result

合法查询没有数据时返回 `200 OK`：

```json
{
  "query": {
    "market": "sh",
    "symbol": "600519",
    "period": "1d",
    "limit": 1000
  },
  "bars": []
}
```

### Validation Errors

非法 market：

```http
GET /api/v1/bars?market=bad&symbol=600519&period=1d
```

```json
{
  "error": "market must be sh, sz, or bj"
}
```

非法 symbol：

```json
{
  "error": "symbol must be six digits"
}
```

非法 period：

```json
{
  "error": "period must be 1d, 1m, or 5m"
}
```

非法 adjust：

```json
{
  "error": "adjust must be none, qfq, or hfq"
}
```

非法 limit：

```json
{
  "error": "limit must be <= 10000"
}
```

非法日期：

```json
{
  "error": "invalid date \"2024/01/01\", expected YYYY-MM-DD"
}
```

## GET /api/v1/intraday-points

查询已落库的 A 股 TDX 分时点。该接口只读 ClickHouse 中的 `a_share_intraday_points`，不会连接 live TDX server 补数据。

分时点是 `price + volume` point，不是 1 分钟 OHLCV K 线。

### Request

按交易日查询：

```http
GET /api/v1/intraday-points?market=sh&symbol=600519&date=2026-06-05&limit=240
```

按时间范围查询：

```http
GET /api/v1/intraday-points?market=sh&symbol=600519&since=2026-06-05T09:30:00&until=2026-06-05T15:00:00&limit=240
```

### Query Parameters

| Name | Required | Description |
| --- | --- | --- |
| `market` | yes | 市场，取值 `sh` / `sz` / `bj` |
| `symbol` | yes | 6 位证券代码，例如 `600519` |
| `date` | no | 交易日，`YYYY-MM-DD` 或 `YYYYMMDD`；不能和 `since` / `until` 同用 |
| `since` | no | 起始时间；和 `until` 成对使用 |
| `until` | no | 截止时间；和 `since` 成对使用 |
| `limit` | no | 最大返回点数，默认 `1000`，最大 `10000` |

必须提供 `date`，或同时提供 `since` 和 `until`。

### Response

```json
{
  "query": {
    "market": "sh",
    "symbol": "600519",
    "date": "2026-06-05",
    "limit": 240
  },
  "points": [
    {
      "market": "sh",
      "symbol": "600519",
      "trade_date": "2026-06-05",
      "point_time": "2026-06-05T09:30:00+08:00",
      "point_index": 0,
      "price": 12.34,
      "volume": 100
    }
  ]
}
```

合法查询没有数据时返回：

```json
{
  "query": {
    "market": "sh",
    "symbol": "600519",
    "date": "2026-06-05",
    "limit": 240
  },
  "points": []
}
```

## GET /api/v1/resolve-symbol

根据 6 位 A 股代码推导市场。

### Request

```http
GET /api/v1/resolve-symbol?symbol=600519
```

### Response

```json
{
  "symbol": "600519",
  "market": "sh"
}
```

固定推导规则：

```text
920* / 8* / 4* -> bj
6* / 9*        -> sh
其他 A 股      -> sz
```

非法 symbol 返回：

```http
HTTP/1.1 400 Bad Request
```

```json
{
  "error": "symbol must be six digits"
}
```

## 每日复盘 API

以下接口由 ClickHouse 提供，只读，不读取旧 JSON、不临时调用 THS/TDX。
`schema_version` 为 `2026-09-05`。表定义见 [ClickHouse](../storage/clickhouse.md#a-股复盘表约定)，迁移入口见 [设计与迁移](../design/ashare-limit-review-data-layer.md)。

### 列表接口

| 路径 | 内容 | 可选业务过滤 |
| --- | --- | --- |
| `/api/v1/limit-events` | 涨停、炸板、跌停单股事件 | market、symbol、event_type、theme |
| `/api/v1/limit-summary` | 每日摘要 | 无 |
| `/api/v1/limit-relay` | 前日事件样本的当日表现 | market、symbol、sample_group、theme、prev_trade_date |
| `/api/v1/limit-themes` | 题材日聚合 | theme |
| `/api/v1/limit-performance-indices` | 专用表现指数 OHLCV | index_code |
| `/api/v1/market-breadth` | 涨跌家数及强涨强跌累计计数 | 无 |

所有列表使用 GET，日期参数二选一：

- `trade_date=YYYY-MM-DD`：单日。
- `since=YYYY-MM-DD&until=YYYY-MM-DD`：闭区间，两个参数必须同时提供。
- 日期范围不能和 `trade_date` 混用。
- `limit` 默认 1000，允许 1..20000；`offset` 默认 0，允许 0..1000000。
- 单股查询必须同时给 `market=sh|sz|bj` 和六位 `symbol`。
- `event_type=limit_up|open_limit|limit_down`。
- `sample_group=prev_limit_up|prev_ladder|prev_broken|prev_limit_down`。
- `prev_trade_date` 只接受在单日 relay 请求中使用，必须早于 `trade_date`。未提供时取当日摘要中的上一交易日，不会简单减一个自然日。
- `theme` 在事件查询中匹配主题材或 theme_tags；在 relay 中只匹配前日主题材。
- `index_code=prev_limit_up_perf|prev_non_st_limit_up_perf|prev_ladder_perf|prev_limit_down_perf`，未指定则返回全部已保存指数。

日期错误、倒置区间、非法枚举、不适用的业务过滤参数、显式 limit=0 等返回 HTTP 400，错误体为 `{"error":"..."}`。数据库错误返回 HTTP 500。

列表响应统一为：

```json
{
  "query": {"trade_date": "2026-09-04", "event_type": "limit_up", "limit": 1000, "offset": 0},
  "rows": [],
  "has_more": false
}
```

有 `has_more=true` 时按实际返回行数推进 offset。所有结果有确定排序；订正或新导入期间不是跨请求一致性快照，上层批量导出应避开同时写入。没有记录返回空数组，不代表该日确认为零。

事件行字段：

```json
{
  "trade_date": "2026-09-04",
  "market": "sz",
  "symbol": "000001",
  "event_type": "limit_up",
  "close_status": "sealed",
  "board_count": 2,
  "reason_text": "示例归因",
  "theme_primary": "示例题材",
  "theme_tags": ["示例题材"],
  "first_limit_minute": "09:35",
  "last_limit_minute": null,
  "open_count": null,
  "seal_order_amount": null,
  "amount": null,
  "turnover_rate": null,
  "market_value": null
}
```

上述为字段示例，不是真实股票当日数据。事件不返回名称、latest 或 pct_chg；名称使用证券主数据，股价使用 bars。专用表现指数返回 OHLCV，未附加涨幅；展示时按该指数前一交易日 close 计算，不能用个股均值替代。获取一个区间的首日涨幅时，需额外取得前一交易日指数收盘值。

### 订正与派生查询边界

- 事件查询通过 FINAL 读取同键最终值。对 event_type 的修改是另一个逻辑键，不会自动删除原事件。
- 摘要查询会重算已存在摘要日期的基本池计数、板数分布和封板成功率；其余摘要指标仍为导入值。仅补事件不自动创建摘要。
- 单日与区间题材查询在有事件的日期按最终事件主题材重算；无事件日期回退已导入聚合。排名逐日重算后再筛选/分页，不跨日累计排名。
- 单日与区间 relay 在能确定上一交易日且前日有事件时重建四组样本，用当日事件确定状态、用 `a_share_daily_derived.pct_chg` 补涨幅；已保存的 relay 作为结果补源。前日日期来自摘要或已存 relay；显式 prev_trade_date 只用于单日请求。缺前日日期/事件时只返回已保存 relay。部分补录仍是部分样本，不能视为全市场完整池。
- relay/题材范围最长 93 个自然日（含首尾），超出返回 400；先完整重建后筛选和分页。每类输入最多读取 200000 行，超限报错，不静默丢行。十年研究按时间分段。查询重建不会重新写入物化表。
- 摘要 `big_noodle_count/high_level_break_count/strong_theme_count` 为可空计数；新在线采集未计算这些统计时返回 null，只有明确已知零才是 0。其余尚未重算指标继续使用导入值。
- Relay `today_status`：promoted=板数晋级，sealed=涨停但未判定晋级，broken=未封板，open_limit=炸板，limit_down=跌停，suspended=输入明确停牌，unknown=未知。旧“平板”映射 sealed，不等于涨跌幅 0。
- `today_pct_chg` 为百分数，10% 返回 10；unknown/NULL 不自动当作零收益或停牌。数值单位与成功率的 0..1 不同。

### 常用请求

```bash
# 一天涨停股票
curl 'http://127.0.0.1:8808/api/v1/limit-events?trade_date=2026-09-04&event_type=limit_up'

# 前一交易日跌停样本在周一的表现（prev_trade_date 为周五）
curl 'http://127.0.0.1:8808/api/v1/limit-relay?trade_date=2026-09-07&prev_trade_date=2026-09-04&sample_group=prev_limit_down'

# 单股历史涨停日期、板数、原因
curl 'http://127.0.0.1:8808/api/v1/limit-events?market=sz&symbol=000001&event_type=limit_up&since=2016-01-01&until=2026-09-04'

# 软件定义的昨日非 ST 涨停表现指数
curl 'http://127.0.0.1:8808/api/v1/limit-performance-indices?index_code=prev_non_st_limit_up_perf&since=2026-08-01&until=2026-09-04'

# 市场宽度
curl 'http://127.0.0.1:8808/api/v1/market-breadth?trade_date=2026-09-04'
```

### GET /api/v1/limit-review

必填 `trade_date`，只重建一天，不使用客户端分页参数裁剪复盘内容：

```bash
curl 'http://127.0.0.1:8808/api/v1/limit-review?trade_date=2026-09-04'
```

```json
{
  "trade_date": "2026-09-04",
  "summary": null,
  "market_breadth": null,
  "performance_indices": [],
  "limit_up_pool": [],
  "broken": [],
  "limit_down": [],
  "ladder": [],
  "relay": [],
  "theme_overview": [],
  "warnings": [
    "summary_missing",
    "market_breadth_missing",
    "performance_indices_missing",
    "relay_missing",
    "events_missing"
  ]
}
```

有数据时，各字段使用列表接口的行结构；ladder 为 `[{"height":3,"stocks":[事件行]}]`，高度降序。摘要/宽度缺失返回 null，其他集合为空数组，不编造情绪结论、截图、盘中走势或主观备注。题材为空时无额外 warning。部分指数缺失时数组只包含已有记录，不保证四种齐全。

每个集合最多重建 20000 行，超过返回错误，不静默截断。摘要与事件数不符时可出现 `summary_event_count_mismatch`；宽度中已知涨跌停计数与摘要不符时出现 `breadth_limit_count_mismatch`，不自动覆盖双方口径。接口多次读取不是跨表事务快照。

`/api/v1` 保持只读。补录走下述显式启用的 console 写操作，没有新增 infinity CLI 查询子命令。Go HTTPClient 提供 LimitEvents、LimitReview 和 LimitReviewMatrix 方法；其他接口可直接走 HTTP。

### GET /api/v1/limit-review-matrix

历史摘要中的 NULL 封板率保留为未知，不因未记录炸板就计算为 100%。各项计数表示已保存事件的数量；部分历史回放的零炸板不代表真实全市场零炸板，覆盖边界见 `docs/design/ashare-limit-review-migration-20260905.md`。

```bash
curl 'http://127.0.0.1:8808/api/v1/limit-review-matrix?since=2026-08-01&until=2026-09-04&limit=100&offset=0'
```

接受 `trade_date` 或 `since/until`，最长 93 个自然日；可按 market/symbol、theme、event_type 选择股票。limit 默认 100、最大 500，offset 是股票偏移。行按 market/symbol 排序，不按名称排序。

- `query`：归一后的请求。
- `days`：日期升序，每项含 trade_date、summary、market_breadth、performance_indices、themes。
- `rows`：每项为 market、symbol、cells；每个 cell 含 trade_date、events、relay，日期升序。
- `total_rows/has_more`：符合选股条件的总股票数和是否有下一页。
- `warnings`：缺失信息提示。

筛选只决定哪些股票入选，不裁掉其其他日期/题材/事件，所以可以选择区间涨停股，同时看到它之后的跌停。未指定 event_type 时也可从 relay 样本入选。单元格允许同日多个事件和多个样本组，不随意选其中一个。每类数据最多完整加载 200000 行，超限或分页不完整时报错。

`days` 只包含有记录的日期，不宣称是完整交易日历；没有 cell 表示缺记录，不等于未涨停/停牌/收益为零。`missing_cells_are_unknown` 和 `dates_only_include_available_records` 明确这一边界。每个已知日期缺摘要/宽度/全部指数时另有日期前缀 warning；指数数组仍不保证四类齐全。价格、K 线和分时图继续使用 bars API，名称和笔记来自上层，接口不提供生成的图片或主观情绪结论。读取不是跨表事务快照。

### POST /api/console/imports/limit-review-corrections

由现有 `infinity-console` 注入写操作，普通 `infinity querier serve` 不注册此端点。启动 console 前通过进程环境设置非空 `INFINITY_REVIEW_WRITE_TOKEN`；未设置则端点不存在。不把令牌放入前端、URL、仓库或日志。console 默认仅监听 `127.0.0.1:8809`；远程调用必须有 TLS 和受控网络边界。

```bash
# 令牌由部署环境注入，console-dist 必须已构建。
go run ./cmd/infinity-console --config configs/config.yaml --listen 127.0.0.1:8809
```

请求是**单个 JSON 对象**，字段与 `import-limit-review-corrections` 的一行 JSONL 相同。必须携带 `Authorization: Bearer <token>` 和 `Content-Type: application/json`。拒绝 Origin 请求头，供工作台后端/解析任务调用，不供浏览器直连。

```bash
# 默认预览：不写市场表或 ops；correction.json 由工作台导出。
curl --fail-with-body 'http://127.0.0.1:8809/api/console/imports/limit-review-corrections' \
  -H "Authorization: Bearer $INFINITY_REVIEW_WRITE_TOKEN" \
  -H 'Content-Type: application/json' --data-binary @correction.json
# 确认预览后，同一请求加 ?dry_run=false 才正式执行。
```

- 只允许 `dry_run=true|false`，默认 true；不接受其他或重复参数。请求上限 4 MiB，执行超时 30 秒，同一 console 进程串行执行补录；并发请求返回 409 和 Retry-After。CLI、其他进程和在线采集仍由部署方串行调度。
- HTTP 只接受 `mode: "enrich_existing"`，仍提交完整事件行。Go 在预览和写入前读取当前行，只允许补空 `reason_text` 或空/`未分类` 的 `theme_primary`；现有非空归因、事件键、状态、连板、金额、时间、tags 等必须保持一致，缺行或读取失败整批拒绝。遗漏已存可选值也会被拒绝，不会默默清空。
- 操作员确需订正核心事实时，CLI 使用 `mode: "upsert"` 并显式加 `--allow-fact-replacement`，该危险能力不向材料 HTTP 端点开放。更改 event_type 仍产生新键，不删除原键。CLI 默认通过 `--read-url http://127.0.0.1:8808` 读取当前行，不直接查询 ClickHouse。
- 成功返回 `run_id/events/rows_written/rows_skipped/issues/dry_run`。401 鉴权失败，403 浏览器来源请求，400 contract 校验失败，413 超限，415 媒体类型错误；执行失败返回对应错误和可用的 `result`，包括已创建的 run_id。
- 写入无事务、没有恰好一次保证。连接中断/超时不代表没写入，先通过事件查询与任务记录核对，再串行重试完整同一批。不要让 HTTP 客户端自动重试正式 POST，也不要用旧快照覆盖已确认订正。
- 不新增补录表。reason/audit_ref 只进入现有 task_runs.params；预览不会产生任务记录。非 ST 指数和宽度补数继续用已实现 JSON 导入命令，不属于这个事件写接口。

## CLI Mapping

`infinity querier` CLI 是 HTTP API 客户端，不直接连接 ClickHouse。

```bash
go run ./cmd/infinity querier health --url http://127.0.0.1:8808
```

等价于：

```http
GET /api/v1/health
```

```bash
go run ./cmd/infinity querier bars \
  --url http://127.0.0.1:8808 \
  --market sh \
  --symbol 600519 \
  --period 1d \
  --since 2024-01-01 \
  --until 2024-12-31 \
  --limit 1000
```

等价于：

```http
GET /api/v1/bars?market=sh&symbol=600519&period=1d&since=2024-01-01&until=2024-12-31&limit=1000
```

查询最近 120 根日 K：

```bash
go run ./cmd/infinity querier bars \
  --url http://127.0.0.1:8808 \
  --market sh \
  --symbol 600519 \
  --period 1d \
  --limit 120
```

等价于：

```http
GET /api/v1/bars?market=sh&symbol=600519&period=1d&limit=120
```

推导市场：

```bash
go run ./cmd/infinity querier resolve-symbol \
  --url http://127.0.0.1:8808 \
  --symbol 600519
```

等价于：

```http
GET /api/v1/resolve-symbol?symbol=600519
```

## Compatibility Notes

- API 调用方不应依赖 ClickHouse 数据库名或表名。
- API 调用方不应依赖 `infinity-marketd` 内部 Go package。
- API 调用方不应解析服务端日志作为数据接口。
- `/api/v1/bars` 是当前稳定查询入口。
- `/api/v1/resolve-symbol` 是当前稳定市场推导入口。
- 市场查询接口放在 `/api/v1`；显式写操作放在 `/api/console/imports`，不要混入只读市场查询契约。
