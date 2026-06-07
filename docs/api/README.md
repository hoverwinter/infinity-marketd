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

`/api/v1/...` 是产品级查询 API，当前主要面向 ClickHouse-backed canonical market data。调用方可以把这些接口视为稳定、可重复读取的查询面。

TDX 在线协议能力不放在 `/api/v1` 下。后续 TDX provider/protocol API 设计为：

```text
/api/tdx/hq/...
/api/tdx/exhq/...
```

边界约定：

| Namespace | Owner | Data source | Contract |
| --- | --- | --- | --- |
| `/api/v1/...` | querier product/query API | ClickHouse 或稳定内部状态 | 稳定查询、可重复读取、不发起 live TDX 请求 |
| `/api/tdx/hq/...` | TDX standard行情 provider API | live TDX HQ upstream | 请求/响应式在线读取，可能超时或受上游影响 |
| `/api/tdx/exhq/...` | TDX extended行情 provider API | live TDX ExHQ upstream | 扩展市场协议读取，使用 numeric market id 和 instrument code |

`/api/tdx/...` 不应隐式写入 ClickHouse。实时快照持久化、保留周期、去重和查询模型需要单独 storage contract。

TDX provider API 的详细 endpoint contract 见 [tdx.md](tdx.md)。

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
  "schema_version": "2026-06-06"
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
- 后续管控控制台接口应继续放在 `/api/v1` 下，按资源拆分路径。
