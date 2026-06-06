# TDX Provider API Reference

本文档描述 `infinity querier serve` 下的 TDX provider/protocol HTTP API。

TDX API 与 `/api/v1` 的 ClickHouse-backed 查询 API 隔离：

```text
/api/v1/...
  产品级查询 API，查询已落库、可重复读取的数据。

/api/tdx/hq/...
  TDX 标准行情 provider API，实时连接 TDX HQ upstream。

/api/tdx/exhq/...
  TDX 扩展行情 provider API，实时连接 TDX ExHQ upstream。
```

`/api/tdx/*` 是请求/响应式 live upstream read，不隐式写入 ClickHouse，不提供 WebSocket/SSE。

## 通用约定

所有响应默认使用 JSON：

```http
Content-Type: application/json
```

错误响应格式：

```json
{
  "error": "unsupported symbol \"abc\""
}
```

状态码：

| Status | Meaning |
| ---: | --- |
| `200` | 请求成功 |
| `400` | 参数错误 |
| `502` | 上游响应可达，但协议解码失败 |
| `503` | TDX upstream 不可用、超时或所有候选 server 失败 |
| `500` | 未预期服务错误 |

通用 query 参数：

| Name | Description |
| --- | --- |
| `server` | 可重复或逗号分隔的 TDX server 地址；不传则使用内置候选 server |
| `servers` | `server` 的复数别名，支持逗号分隔 |

## Standard HQ

标准行情路径使用：

```text
/api/tdx/hq/...
```

标准行情市场：

```text
sh
sz
bj
```

### GET /api/tdx/hq/quotes

查询 A 股实时 quote snapshot。

Request:

```http
GET /api/tdx/hq/quotes?symbol=sh:600519&symbol=000001&batch-size=80&trade_date=2026-06-05
```

Query parameters:

| Name | Required | Description |
| --- | --- | --- |
| `symbol` | yes | 可重复。支持 `market:symbol` 或 6 位代码自动推导市场 |
| `symbols` | no | 逗号分隔 symbol 列表，和 `symbol` 合并 |
| `server` / `servers` | no | TDX HQ server override |
| `batch-size` / `batch_size` | no | quote batch size，最大 `200` |
| `trade_date` | no | `YYYY-MM-DD`，用于生成 `quote_time` |

最多一次请求 `200` 个 symbol。

Response:

```json
[
  {
    "market": "sh",
    "symbol": "600519",
    "price": 1272.86,
    "last_close": 1268.0,
    "open": 1278.0,
    "high": 1283.0,
    "low": 1267.74,
    "server_time": "14:52:22.494",
    "server_intraday_time": "14:52:22.494",
    "trade_date": "2026-06-05",
    "quote_time": "2026-06-05 14:52:22.494",
    "volume": 31303,
    "current_volume": 560,
    "amount": 3984001792.0,
    "sell_volume": 17408,
    "buy_volume": 13896,
    "bids": [{ "price": 1271.0, "volume": 1 }],
    "asks": [{ "price": 1272.86, "volume": 7 }]
  }
]
```

### GET /api/tdx/hq/probe

探测 TDX HQ server。

Request:

```http
GET /api/tdx/hq/probe?server=180.153.18.170:7709&server=60.191.117.167:7709
```

Response:

```json
[
  {
    "server": "180.153.18.170:7709",
    "success": true,
    "latency_ms": 12,
    "preferred": true
  },
  {
    "server": "60.191.117.167:7709",
    "success": false,
    "latency_ms": 5000,
    "error": "connect TDX HQ server ..."
  }
]
```

### GET /api/tdx/hq/securities

读取标准行情市场证券列表。

Request:

```http
GET /api/tdx/hq/securities?market=sh
```

Response:

```json
[
  {
    "market": "sh",
    "symbol": "600519",
    "name": "贵州茅台",
    "volunit": 100,
    "decimal_point": 2,
    "pre_close": 1270.0
  }
]
```

### GET /api/tdx/hq/bars

读取标准行情 K 线。

Request:

```http
GET /api/tdx/hq/bars?market=sh&symbol=600519&category=9&start=0&count=100
```

Query parameters:

| Name | Required | Description |
| --- | --- | --- |
| `market` | yes | `sh` / `sz` / `bj` |
| `symbol` | yes | 6 位代码 |
| `category` | no | K 线类别，默认 `9` |
| `start` | no | 起始 offset，默认 `0` |
| `count` | no | 返回数量，最大 `800` |
| `index` | no | `true` 时使用 index K-line 请求 |

### GET /api/tdx/hq/minute

读取当日或历史分时。

```http
GET /api/tdx/hq/minute?market=sh&symbol=600519
GET /api/tdx/hq/minute?market=sh&symbol=600519&date=20260605
```

`date` 存在时读取历史分时，格式为 `YYYYMMDD`。

### GET /api/tdx/hq/transactions

读取当日或历史分笔。

```http
GET /api/tdx/hq/transactions?market=sh&symbol=600519&start=0&count=1000
GET /api/tdx/hq/transactions?market=sh&symbol=600519&date=20260605&start=0&count=1000
```

`count` 最大 `1800`。

### Company, Finance, XDXR, Block

```http
GET /api/tdx/hq/company-categories?market=sh&symbol=600519
GET /api/tdx/hq/company-content?market=sh&symbol=600519&filename=xxx.txt&start=0&length=1024
GET /api/tdx/hq/xdxr?market=sh&symbol=600519
GET /api/tdx/hq/finance?market=sh&symbol=600519
GET /api/tdx/hq/block-meta?file=block.dat
GET /api/tdx/hq/block-chunk?file=block.dat&start=0&size=30000
GET /api/tdx/hq/block?file=block.dat
```

## Extended ExHQ

扩展行情路径使用：

```text
/api/tdx/exhq/...
```

ExHQ 使用 numeric market id 和 instrument code，不使用 `sh` / `sz` / `bj`。

### GET /api/tdx/exhq/markets

读取扩展行情市场列表。

```http
GET /api/tdx/exhq/markets
```

### GET /api/tdx/exhq/count

读取扩展行情 instrument 总数。

```http
GET /api/tdx/exhq/count
```

Response:

```json
{
  "count": 12345
}
```

### GET /api/tdx/exhq/instruments

读取扩展行情 instrument 列表。

```http
GET /api/tdx/exhq/instruments?start=0&count=100
```

`count` 最大 `1000`。

### GET /api/tdx/exhq/quote

读取扩展品种 quote。

```http
GET /api/tdx/exhq/quote?market=47&code=IF1709
```

### GET /api/tdx/exhq/bars

读取扩展品种 K 线。

```http
GET /api/tdx/exhq/bars?market=47&code=IF1709&category=4&start=0&count=100
```

`count` 最大 `800`。

### GET /api/tdx/exhq/minute

读取扩展品种当日或历史分时。

```http
GET /api/tdx/exhq/minute?market=47&code=IF1709
GET /api/tdx/exhq/minute?market=47&code=IF1709&date=20260605
```

### GET /api/tdx/exhq/transactions

读取扩展品种当日或历史分笔。

```http
GET /api/tdx/exhq/transactions?market=47&code=IF1709&start=0&count=1800
GET /api/tdx/exhq/transactions?market=47&code=IF1709&date=20260605&start=0&count=1800
```

`count` 最大 `1800`。

### GET /api/tdx/exhq/history-bars

读取扩展品种历史 K 线日期区间。

```http
GET /api/tdx/exhq/history-bars?market=47&code=IF1709&start_date=20260601&end_date=20260605
```

也兼容 `start-date` / `end-date`。

## Compatibility Notes

- `/api/tdx/*` 调用方不应假设结果已落库。
- `/api/tdx/*` 调用方不应依赖 ClickHouse 表名。
- `/api/tdx/hq/*` 和 `/api/tdx/exhq/*` 的参数模型故意不同，因为底层协议不同。
- `/api/tdx/*` 返回的字段尽量贴近 TDX 解码结果；产品级字段归一化应在 `/api/v1` 或上层 adapter 中完成。
