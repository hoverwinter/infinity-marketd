# TDX 实时行情实现说明

本文档说明 `marketd` 已实现的 TDX 标准行情和扩展行情能力，以及这些能力的实现原理。

相关代码：

- `internal/tdx/quote.go`：实时行情请求包、响应解析、价格和成交额解码。
- `internal/tdx/quote_ops.go`：server 探测、候选 server 重试、批量会话复用、证券列表、扫盘 workflow。
- `internal/tdx/exquote.go`：`exhq` 扩展行情 setup、市场列表、单合约 quote 请求和响应解析。
- `internal/cli/cli.go`：`quote`、`quote-probe`、`quote-sweep`、`exquote-markets`、`exquote` CLI。

该实现用于平替 pytdx `hq.get_security_quotes` 和 `exhq.get_instrument_quote` 的部分实时行情路径，不依赖 Python、pytdx、mootdx、pandas，也不要求 ClickHouse 连接。

## 已实现能力

### 单只或多只 A 股实时快照

命令：

```bash
go run ./cmd/marketd quote \
  --symbol sh:600519 \
  --symbol sz:000001 \
  --server 180.153.18.170:7709
```

支持：

- `sh` 上海市场。
- `sz` 深圳市场。
- 一个命令请求多只股票。
- `--symbol` 重复传入。
- `--symbol` 使用逗号分隔。
- 未显式 market 时按代码前缀推断。

symbol 示例：

| 输入 | 结果 |
| --- | --- |
| `sh:600519` | 上海 `600519` |
| `sz:000001` | 深圳 `000001` |
| `600519` | 推断为 `sh:600519` |
| `000001` | 推断为 `sz:000001` |

返回 JSON array。每条 quote 包含：

| 字段 | 含义 |
| --- | --- |
| `market` | `sh` 或 `sz` |
| `symbol` | 六位证券代码 |
| `price` | 当前价 |
| `last_close` | 昨收 |
| `open` / `high` / `low` | 当日开高低 |
| `server_time` | 兼容字段，TDX 返回的日内服务器时间 |
| `server_intraday_time` | 明确语义后的日内服务器时间 |
| `trade_date` | 调用方显式传入交易日时输出 |
| `quote_time` | `trade_date + server_intraday_time` |
| `volume` | 成交量字段 |
| `current_volume` | 现量字段 |
| `amount` | 成交额 |
| `sell_volume` / `buy_volume` | TDX payload 中的卖/买量字段 |
| `bids` | 五档买盘 |
| `asks` | 五档卖盘 |

示例：

```json
[
  {
    "market": "sh",
    "symbol": "600519",
    "price": 1272.86,
    "last_close": 1268,
    "open": 1278,
    "high": 1283,
    "low": 1267.74,
    "server_time": "14:52:22.494",
    "server_intraday_time": "14:52:22.494",
    "trade_date": "2026-06-05",
    "quote_time": "2026-06-05 14:52:22.494",
    "volume": 31303,
    "current_volume": 560,
    "amount": 3984001792,
    "sell_volume": 17408,
    "buy_volume": 13896,
    "bids": [
      { "price": 1271, "volume": 1 }
    ],
    "asks": [
      { "price": 1272.86, "volume": 7 }
    ]
  }
]
```

### 多 server 候选和自动重试

`quote` 支持多个 TDX HQ server：

```bash
go run ./cmd/marketd quote \
  --symbol sh:600519 \
  --server 119.147.212.81:7709 \
  --server 180.153.18.170:7709 \
  --server 60.191.117.167:7709
```

实现行为：

- 依次尝试候选 server。
- 连接失败、setup 失败、请求收发失败时切到下一个 server。
- 一旦某个 server 返回有效行情，停止访问后续候选。
- 解码错误不会被重试隐藏，直接返回错误，方便暴露协议解析问题。

默认候选 server 在 `tdx.DefaultHQServers` 中维护。当前默认 server 是：

```text
180.153.18.170:7709
```

### HQ server 探测

命令：

```bash
go run ./cmd/marketd quote-probe \
  --server 180.153.18.170:7709 \
  --server 60.191.117.167:7709
```

输出每个候选 server 的探测结果：

| 字段 | 含义 |
| --- | --- |
| `server` | server 地址 |
| `success` | 是否成功完成 TCP connect + TDX setup |
| `latency_ms` | 探测耗时 |
| `error` | 失败原因 |
| `preferred` | 最快可用 server |

探测结果会把成功 server 排在失败 server 前面，并标记最快成功 server 为 `preferred: true`。

### 批量请求和连接复用

`quote` 和 `quote-sweep` 支持 `--batch-size`：

```bash
go run ./cmd/marketd quote \
  --symbol sh:600519,sz:000001 \
  --batch-size 80 \
  --server 180.153.18.170:7709
```

实现行为：

- 将请求 symbol 按 batch size 切分。
- 每个 server candidate 打开一个 setup 完成后的 `QuoteSession`。
- 同一个 server 上的多个 batch 复用同一条 TCP 连接。
- 批量 workflow 结束后关闭连接。
- 当前没有全局长期连接池，避免 CLI 场景引入生命周期复杂度。

默认 batch size 是 `80`。

### 在线证券列表和扫盘 workflow

`quote-sweep` 支持显式 symbol list，也支持在线发现证券列表后扫盘。

显式 symbol list：

```bash
go run ./cmd/marketd quote-sweep \
  --symbol sh:600519,sz:000001 \
  --server 180.153.18.170:7709
```

在线发现市场证券并扫盘：

```bash
go run ./cmd/marketd quote-sweep \
  --market sh \
  --market sz \
  --limit 200 \
  --server 180.153.18.170:7709
```

实现能力：

- 请求 TDX 标准行情 server 的市场证券数量。
- 按 offset 请求市场证券列表。
- 解析 29 字节证券列表记录。
- 将发现到的 `sh` / `sz` 六位代码转换为 quote request。
- 使用批量 quote workflow 获取行情。
- `--limit` 可限制扫盘数量，方便 smoke test。

证券列表返回结构：

| 字段 | 含义 |
| --- | --- |
| `market` | `sh` 或 `sz` |
| `symbol` | 六位证券代码 |
| `name` | 证券名称，无法安全解码时为空 |
| `volunit` | TDX volume unit |
| `decimal_point` | 小数位 |
| `pre_close` | TDX 编码中的前收字段 |

### 扩展行情 `exhq`

`exhq` 是独立于标准 A 股 `hq` 的 TDX 扩展行情协议，常用于期货、期权、港股、外盘等扩展市场。它使用数字 market id 和 instrument code，不使用 `sh` / `sz` / `bj`。

查询扩展市场列表：

```bash
go run ./cmd/marketd exquote-markets \
  --server 61.152.107.141:7727
```

查询单个扩展品种 quote：

```bash
go run ./cmd/marketd exquote \
  --market 47 \
  --code IF1709 \
  --server 61.152.107.141:7727
```

`exquote` 输出单个 JSON object，字段包括：

| 字段 | 含义 |
| --- | --- |
| `market` | TDX 扩展市场 ID |
| `code` | 扩展品种代码 |
| `pre_close` | 昨收 |
| `open` / `high` / `low` | 当日开高低 |
| `price` | 当前价 |
| `kaicang` | 开仓类字段 |
| `zongliang` | 总量 |
| `xianliang` | 现量 |
| `neipan` / `waipan` | 内盘 / 外盘 |
| `chicang` | 持仓类字段 |
| `bids` / `asks` | 五档买卖盘 |

当前 `exhq` 只实现：

- market list；
- single instrument quote；
- 多 server candidate 顺序 fallback；
- JSON CLI 输出。

当前 `exhq` 不实现：

- instrument count/list；
- K 线；
- 分时；
- 分笔；
- 历史接口；
- ClickHouse 持久化。

### 时间字段语义

TDX quote payload 只提供日内服务器时间，不提供完整日期。

因此当前输出：

- `server_time`：兼容字段。
- `server_intraday_time`：语义明确的日内时间。
- `trade_date`：只有传入 `--trade-date YYYY-MM-DD` 时输出。
- `quote_time`：只有传入 `--trade-date` 时输出。

示例：

```bash
go run ./cmd/marketd quote \
  --symbol sh:600519 \
  --trade-date 2026-06-05 \
  --server 180.153.18.170:7709
```

输出中会包含：

```json
{
  "server_intraday_time": "14:52:22.494",
  "trade_date": "2026-06-05",
  "quote_time": "2026-06-05 14:52:22.494"
}
```

### 明确不写 ClickHouse

实时行情命令当前只返回查询结果，不写入 ClickHouse。

原因：

- quote snapshot 属于高频数据，写入量和保留策略需要单独设计。
- 需要先定义表结构、逻辑键、分区、去重、保留周期。
- 不能把未定型的快照表混入当前 canonical OHLCV fact table。

如果未来决定持久化实时快照，应另起 OpenSpec change 定义 schema 和写入语义。

### `bj` 和 `exhq` 边界

当前标准行情实现只支持 `sh` / `sz`。

`bj`：

- 当前仍明确拒绝。
- 原因是 TDX 标准行情下的北交所 market mapping 和 live sample 解码尚未验证。
- 不在未验证前伪实现。

`exhq`：

- 当前已实现扩展市场列表和单合约 quote。
- 仍然独立于标准 A 股 `hq` client。
- 不支持 `exhq` K 线、分时、分笔、历史和品种列表。
- 不应和标准 A 股 `hq` client 混在同一个协议路径里。

## 实现原理

### 总体流程

```mermaid
sequenceDiagram
    participant CLI as marketd quote
    participant TDX as internal/tdx
    participant HQ as TDX HQ Server

    CLI->>TDX: 解析 symbol / server / batch 参数
    TDX->>TDX: NormalizeHQServers
    loop server candidates
        TDX->>HQ: TCP connect
        TDX->>HQ: setup packet 1
        HQ-->>TDX: setup response
        TDX->>HQ: setup packet 2
        HQ-->>TDX: setup response
        TDX->>HQ: setup packet 3
        HQ-->>TDX: setup response
        loop batches
            TDX->>HQ: quote request packet
            HQ-->>TDX: 16-byte header + body
            TDX->>TDX: 可选 zlib 解压
            TDX->>TDX: DecodeQuoteResponse
        end
    end
    TDX-->>CLI: []Quote
    CLI->>CLI: JSON 输出
```

核心思想：

- CLI 只负责参数解析和 JSON 输出。
- TDX 协议细节集中在 `internal/tdx`。
- server retry 放在高层 workflow，不放在底层 decoder。
- decoder 只做协议解析，遇到截断、未知 market、格式错误直接报错。

### 请求参数解析

`ParseQuoteRequest` 把用户输入转换为：

```go
type QuoteRequest struct {
    Market string
    Symbol string
}
```

规则：

- `market:symbol` 显式指定市场。
- 无 market 时使用 `InferMarketFromCode`。
- 只允许 `sh` 和 `sz`。
- symbol 必须是六位数字。

TDX market byte：

| marketd | TDX market byte |
| --- | --- |
| `sz` | `0` |
| `sh` | `1` |

### 连接和 setup

`OpenQuoteSession` 完成：

1. TCP connect。
2. 设置 deadline。
3. 发送三个 TDX 标准行情 setup 包。
4. 返回可复用的 `QuoteSession`。

setup 包来自 pytdx 标准行情握手路径，对应：

- `SetupCmd1`
- `SetupCmd2`
- `SetupCmd3`

每次 setup 和请求都通过统一的 `quoteConn.call` 收发。

### Quote 请求包结构

`BuildQuoteRequestPacket` 构造 pytdx `hq.get_security_quotes` 等价请求。

对于 `N` 个 symbol：

```text
dataLen = N * 7 + 12
packetLen = 22 + N * 7
```

请求头：

| offset | size | value | 说明 |
| --- | ---: | --- | --- |
| `0` | 2 | `0x010c` | command prefix |
| `2` | 4 | `0x02006320` | command/magic |
| `6` | 2 | `dataLen` | payload length |
| `8` | 2 | `dataLen` | repeated payload length |
| `10` | 4 | `0x0005053e` | command parameter |
| `14` | 4 | `0` | reserved |
| `18` | 2 | `0` | reserved |
| `20` | 2 | `N` | symbol count |

每个证券条目 7 字节：

| size | 含义 |
| ---: | --- |
| 1 | TDX market byte |
| 6 | ASCII symbol |

示例：`sz:000001` + `sh:600519`

```text
00 30 30 30 30 30 31  01 36 30 30 35 31 39
```

### 响应封包和 zlib 解压

所有 TDX 请求都先读 16 字节响应头：

| header bytes | 含义 |
| --- | --- |
| `12..14` | compressed size |
| `14..16` | uncompressed size |

处理逻辑：

1. 读取 16 字节 header。
2. 按 compressed size 读取 body。
3. 如果 compressed size 等于 uncompressed size，body 不解压。
4. 如果两者不同，用 zlib 解压。
5. 解压后的 body 交给具体 decoder。

这部分由 `quoteConn.call` 实现。

### Quote 响应体结构

`DecodeQuoteResponse` 解析实时行情响应。

body 前缀：

| offset | size | 含义 |
| --- | ---: | --- |
| `0` | 2 | 忽略 |
| `2` | 2 | quote count |

每条 quote 开头：

| size | 含义 |
| ---: | --- |
| 1 | TDX market byte |
| 6 | symbol |
| 2 | `active1`，当前跳过 |

后续字段大多使用 TDX 变长有符号整数编码。

### TDX 变长有符号整数

`readTDXVarInt` 实现 TDX quote payload 中的变长整数。

第一个字节：

| bit | 含义 |
| --- | --- |
| `0..5` | value 低 6 位 |
| `6` | sign bit |
| `7` | continuation bit |

如果 continuation bit 为 1，后续字节每个贡献 7 位：

```text
value += (nextByte & 0x7f) << shift
shift 初始为 6，每读一个后续字节增加 7
```

如果 sign bit 为 1，最终值取负。

该编码用于：

- 当前价。
- 昨收、开、高、低相对当前价的差值。
- 五档买卖价相对当前价的差值。
- 成交量类字段。
- server time。
- 若干当前未暴露的协议字段。

### 价格解码

TDX A 股价格按整数分传输。

响应中 `price` 是当前价基准值，其他价格大多是相对当前价的 diff：

```text
decoded_price = (base_price + diff) / 100.0
```

字段映射：

| 输出字段 | 解码方式 |
| --- | --- |
| `price` | `base / 100.0` |
| `last_close` | `(base + lastCloseDiff) / 100.0` |
| `open` | `(base + openDiff) / 100.0` |
| `high` | `(base + highDiff) / 100.0` |
| `low` | `(base + lowDiff) / 100.0` |
| bid price | `(base + bidDiff) / 100.0` |
| ask price | `(base + askDiff) / 100.0` |

### 成交额解码

`amount` 不是普通 IEEE-754 float。

实现中先读取 4 字节 little-endian integer，再用 `decodeTDXFloat` 按 pytdx `get_volume` 的等价算法解码为 `float64`。

### server time 解码

TDX server time 也是变长整数。

`formatTDXQuoteTime` 按 pytdx 行为格式化：

```text
9300000 -> 9:30:00.000
```

注意：这个时间没有日期。只有当调用方显式传入 `--trade-date` 时，系统才输出完整 `quote_time`。

### HQ server 探测原理

`ProbeHQServers` 对每个候选 server 执行：

1. TCP connect。
2. 三个 setup 包。
3. 记录耗时。
4. 成功则标记 `success: true`。
5. 失败则记录错误文本。

`BestHQServer` 从成功结果中选择 `latency_ms` 最小的 server。

这不是长期健康检查，只代表探测时刻的可达性和响应速度。

### 自动重试原理

`FetchRealtimeQuoteBatches` 对候选 server 逐个尝试：

1. `OpenQuoteSession`。
2. 按 batch 发送 quote request。
3. 成功则返回结果。
4. 连接、setup、收发失败则尝试下一个 server。
5. decoder 错误直接返回，不继续重试。

这样做的原因是：

- 网络错误和 server 不可达可以通过切换 server 解决。
- decoder 错误通常说明协议结构不匹配，应该暴露给开发者。

### 批量连接复用原理

`SplitQuoteRequests` 把 symbol 切成多个 batch。

`QuoteSession` 在一个 server 上完成 setup 后，依次发送多个 batch 请求：

```text
connect + setup
  -> batch 1 quote request
  -> batch 2 quote request
  -> batch 3 quote request
close
```

这样避免每个 batch 都重复 TCP connect 和 setup。

### 在线证券列表原理

TDX 标准行情提供两个请求：

| 函数 | 作用 |
| --- | --- |
| `BuildSecurityCountPacket` | 获取市场证券数量 |
| `BuildSecurityListPacket` | 按 offset 获取证券列表页 |

证券数量响应：

```text
uint16 count
```

证券列表响应：

```text
uint16 item_count
item_count * 29-byte record
```

每条 29 字节记录当前解析：

| 字段 | 来源 |
| --- | --- |
| `symbol` | `record[0:6]` |
| `volunit` | `record[6:8]` |
| `name` | `record[8:16]`，当前只在 UTF-8 有效时保留 |
| `decimal_point` | `record[20]` |
| `pre_close` | `record[21:25]`，TDX float-like 解码 |

`QuoteSweep` 在未传入显式 symbol 时，会先调用证券列表发现，再进入批量 quote。

### ExHQ 实现原理

`OpenExQuoteSession` 完成：

1. TCP connect。
2. 设置 deadline。
3. 发送一个 TDX 扩展行情 setup 包。
4. 返回可复用的 `ExQuoteSession`。

扩展市场列表请求包：

```text
01 02 48 69 00 01 02 00 02 00 f4 23
```

扩展市场列表响应：

```text
uint16 count
count * 64-byte record
```

单合约 quote 请求包由固定 12 字节前缀加 10 字节 identity 组成：

```text
01 01 08 02 02 01 0c 00 0c 00 fa 23
uint8 market
char[9] code
```

单合约 quote 响应使用定长结构：

```text
uint8 market
char[9] code
byte[4] ignored
float32 pre_close, open, high, low, price
uint32 kaicang, ignored, zongliang, xianliang, ignored, neipan, waipan, ignored, chicang
float32 bid1..bid5
uint32 bid_vol1..bid_vol5
float32 ask1..ask5
uint32 ask_vol1..ask_vol5
```

`exhq` quote 价格字段是普通 `float32`，不同于标准 `hq` A 股 quote 的变长整数差分编码。

### CLI 错误处理

参数错误返回 exit code `2`：

- 没有 `--symbol`。
- symbol 格式错误。
- market 不支持。
- flag 解析失败。

运行时错误返回 exit code `1`：

- 所有 server candidates 都失败。
- TCP connect 超时。
- setup 失败。
- 请求收发失败。
- 响应截断。
- zlib 解压失败。
- 响应中出现不支持的 TDX market code。

stdout 只输出 JSON，stderr 输出错误。

## 测试和验证

单元测试：

```bash
go test ./internal/tdx ./internal/cli
```

全量测试：

```bash
go test ./...
```

OpenSpec 校验：

```bash
openspec validate --all
```

实时 smoke test：

```bash
go run ./cmd/marketd quote \
  --symbol sh:600519 \
  --symbol sz:000001 \
  --server 180.153.18.170:7709
```

server 探测：

```bash
go run ./cmd/marketd quote-probe \
  --server 180.153.18.170:7709 \
  --server 60.191.117.167:7709
```

小规模扫盘：

```bash
go run ./cmd/marketd quote-sweep \
  --market sh \
  --limit 10 \
  --server 180.153.18.170:7709
```

扩展市场列表：

```bash
go run ./cmd/marketd exquote-markets \
  --server 61.152.107.141:7727
```

扩展品种 quote：

```bash
go run ./cmd/marketd exquote \
  --market 47 \
  --code IF1709 \
  --server 61.152.107.141:7727
```

负向验证：

```bash
go run ./cmd/marketd quote --symbol bj:920001
```

预期：返回 unsupported market 错误。

## 当前限制

- 不支持 `bj` 实时行情。
- `exhq` 只支持 market list 和 single instrument quote。
- `exhq` 市场名称目前没有做 GBK 解码，非 UTF-8 名称会置空。
- 不做长期心跳保活。
- 不做全局连接池。
- 不做实时流式订阅。
- 不写 ClickHouse。
- 证券名称目前没有做 GBK 解码，非 UTF-8 名称会置空。
- `quote_time` 只有调用方显式提供 `--trade-date` 时才输出。

## 后续方向

- 验证北交所实时行情的真实 TDX market mapping 和响应样本。
- 补齐 `exhq` instrument list、K 线、分时、分笔和历史接口。
- 为证券列表名称增加 GBK 解码。
- 如果需要长期运行的行情服务，再引入连接池、心跳和定期重连。
- 如果需要持久化 quote snapshot，另起 OpenSpec change 定义 ClickHouse schema、保留策略和去重语义。
