# pytdx and mootdx Capability Reference

整理日期：2026-06-07

本文档整理 `pytdx` 和 `mootdx` 的主要能力，用于 `marketd` 规划和验收通达信能力平替。目标不是依赖这些 Python 包，而是明确它们提供了哪些功能、哪些能力已经或应该由 Go 实现覆盖。

## Sources

- pytdx docs: https://pytdx-docs.readthedocs.io/zh-cn/latest/
- pytdx standard行情 docs: https://rainx.gitbooks.io/pytdx/content/pytdx_hq.html
- pytdx trade docs: https://pytdx-docs.readthedocs.io/zh-cn/latest/pytdx_trade/
- pytdx reader CLI docs: https://pytdx-docs.readthedocs.io/zh-cn/latest/hqreader/
- pytdx financial crawler docs: https://pytdx-docs.readthedocs.io/zh-cn/latest/pytdx_crawler/
- pytdx source: https://github.com/rainx/pytdx
- mootdx GitHub: https://github.com/mootdx/mootdx
- mootdx docs: https://www.mootdx.com
- mootdx PyPI: https://pypi.org/project/mootdx/

## pytdx

`pytdx` 是较底层的通达信协议实现。它提供标准行情、扩展行情、本地文件读取、财务文件下载解析、连接池和交易包装等能力。对 `marketd` 最有参考价值的是 `hq` 的标准 A 股行情协议和 `reader` 的本地 TDX 文件格式覆盖。

### Package Layout

| Module | Capability | Notes for marketd |
| --- | --- | --- |
| `pytdx.hq` | 标准行情，主要用于 A 股、指数、分时、分笔、F10、除权除息、财务信息、板块信息 | A 股实时行情平替的主参考 |
| `pytdx.exhq` | 扩展行情，期货、期权、港股、外盘等扩展市场 | 不是 A 股标准实时行情主路径 |
| `pytdx.reader` | 读取本地通达信数据文件 | 可作为 `.day`、`.lc1`、`.lc5`、`.1`、`.5`、板块文件解析参考 |
| `pytdx.crawler` | 下载和解析历史专业财务文件 `gpcw*.zip/.dat` | 可规划为后续财务数据导入能力 |
| `pytdx.pool` | 连接池、failover、主备连接 | 可参考但不必照搬 |
| `pytdx.trade` | 通过 `TdxTradeServer` 包装 `trade.dll` 的交易接口 | 不属于 `marketd` 数据平面目标 |
| CLI | `hqget`、`hqreader` | 可参考数据导出和调试方式 |

### `pytdx.hq` Standard行情

典型用法：

```python
from pytdx.hq import TdxHq_API
from pytdx.params import TDXParams

api = TdxHq_API(heartbeat=True, auto_retry=True)

with api.connect("180.153.18.170", 7709):
    rows = api.get_security_quotes([
        (TDXParams.MARKET_SH, "600519"),
        (TDXParams.MARKET_SZ, "000001"),
    ])
    df = api.to_df(rows)
```

市场代码：

| TDX market | Meaning |
| --- | --- |
| `0` | Shenzhen / `sz` |
| `1` | Shanghai / `sh` |

Core API surface:

| API | Purpose |
| --- | --- |
| `get_security_quotes` | 多证券实时快照，含当前价、OHLC、成交量额、五档盘口 |
| `get_security_bars` | 股票 K 线 |
| `get_index_bars` | 指数 K 线 |
| `get_security_count` | 市场证券数量 |
| `get_security_list` | 股票列表 |
| `get_minute_time_data` | 当日分时 |
| `get_history_minute_time_data` | 历史分时 |
| `get_transaction_data` | 当日分笔成交 |
| `get_history_transaction_data` | 历史分笔成交 |
| `get_company_info_category` | 公司信息目录 |
| `get_company_info_content` | 公司信息内容 |
| `get_xdxr_info` | 除权除息信息 |
| `get_finance_info` | 财务信息 |
| `get_block_info_meta` / `get_block_info` | 板块信息 |

#### A-share realtime quote fields

`get_security_quotes` 返回的关键字段：

| Field | Meaning |
| --- | --- |
| `market`, `code` | 市场和证券代码 |
| `price` | 当前价 |
| `last_close` | 昨收 |
| `open`, `high`, `low` | 当日开高低 |
| `servertime` | 行情服务器时间 |
| `vol`, `cur_vol` | 总成交量、现量 |
| `amount` | 成交额 |
| `s_vol`, `b_vol` | 内外盘类字段 |
| `bid1..bid5`, `ask1..ask5` | 五档买卖价 |
| `bid_vol1..5`, `ask_vol1..5` | 五档买卖量 |
| `reversed_bytes9` | pytdx 注释为涨速 |
| `active1`, `active2`, `reversed_bytes*` | 未完全确认语义的协议字段 |

#### Protocol notes

`pytdx.hq` 标准行情协议要点：

- 标准 HQ server 常用端口是 `7709`。
- 连接后先发送 3 个 setup 包。
- 实时快照请求支持批量证券，每个证券编码为 `market + 6 byte symbol`。
- 响应先读取 16 字节 header，header 中包含压缩长度和解压长度。
- 当压缩长度和解压长度不一致时，body 使用 zlib 解压。
- 快照价格字段不是普通定长整数。很多字段使用 TDX 变长有符号整数编码。
- `price` 是基准当前价，`last_close/open/high/low/bid/ask` 多数是相对 `price` 的差值。
- A 股价格按整数分传输，最终价格通常是 `(base + diff) / 100.0`。

`marketd` 已实现等价的 Go 能力：

```bash
go run ./cmd/marketd quote \
  --symbol sh:600519 \
  --symbol sz:000001 \
  --server 180.153.18.170:7709
```

Current coverage:

| pytdx capability | marketd status |
| --- | --- |
| `hq.get_security_quotes` for `sh` / `sz` | Implemented as `marketd quote` and `tdx.FetchRealtimeQuotes` |
| zlib response handling | Implemented |
| TDX variable integer price decoding | Implemented |
| five-level depth | Implemented |
| server probe / fallback | Implemented through `quote-probe` and multi-server quote retry |
| heartbeat / long-lived reconnect | Not implemented |
| `bj` realtime quotes | Not implemented |

### `pytdx.exhq` Extended行情

`exhq` 使用扩展市场服务器，常见端口是 `7727`。它面向期货、期权、港股、外盘等市场，不是 A 股标准行情主路径。

典型用法：

```python
from pytdx.exhq import TdxExHq_API

api = TdxExHq_API()
with api.connect("61.152.107.141", 7727):
    rows = api.get_instrument_quote(47, "IF1709")
```

Core API surface:

| API | Purpose |
| --- | --- |
| `get_markets` | 扩展市场列表 |
| `get_instrument_count` | 品种数量 |
| `get_instrument_info` | 品种列表/基础信息 |
| `get_instrument_quote` | 扩展品种实时快照 |
| `get_minute_time_data` | 分时 |
| `get_history_minute_time_data` | 历史分时 |
| `get_instrument_bars` | K 线 |
| `get_transaction_data` | 分笔 |
| `get_history_transaction_data` | 历史分笔 |

`get_instrument_quote` 字段结构和 `hq.get_security_quotes` 相似，但价格字段是更直接的 `float32` 定长结构，市场代码也不是 `0/1` 的深沪市场。

Key fields:

| Field | Meaning |
| --- | --- |
| `market`, `code` | 扩展市场 ID 和合约/品种代码 |
| `pre_close`, `open`, `high`, `low`, `price` | 昨收、开高低、当前价 |
| `kaicang`, `chicang` | 开仓、持仓类字段 |
| `zongliang`, `xianliang` | 总量、现量 |
| `neipan`, `waipan` | 内盘、外盘 |
| `bid1..bid5`, `ask1..ask5` | 五档买卖价 |
| `bid_vol1..5`, `ask_vol1..5` | 五档买卖量 |

`marketd` status: partially implemented as a separate capability from A-share standard realtime quotes.

Implemented:

- `marketd exquote-markets` for `get_markets`.
- `marketd exquote --market <id> --code <instrument>` for `get_instrument_quote`.

Not implemented:

- instrument count/list;
- K-line;
- minute-time;
- transaction and history APIs;
- ClickHouse persistence.

### `pytdx.reader`

`reader` 读取本地通达信数据文件和导出文件。

Covered formats include:

| Format | Purpose |
| --- | --- |
| `.day` | 日线 OHLCV，价格为整数分 |
| `ex_daily` | 扩展市场日线 |
| `.lc1`, `.lc5` | 1 分钟 / 5 分钟 float32 价格格式 |
| `.1`, `.5` | 1 分钟 / 5 分钟整数分价格格式 |
| `gbbq` | 股本变迁 |
| `block` | 板块股票列表 |
| `customblock` | 自定义板块 |
| `history_financial` / `hf` | 历史专业财务文件 |

CLI example from pytdx:

```bash
hqreader -o /tmp/daily.csv -d daily /path/to/vipdoc/sz/lday/sz000001.day
```

`marketd` current coverage:

| pytdx reader format | marketd status |
| --- | --- |
| `.day` | Implemented |
| `.lc1` | Implemented |
| `.1` | Implemented |
| `.lc5` | Implemented |
| `.5` | Implemented |
| block/customblock | Not implemented |
| `gbbq` | Not implemented |
| historical financial | Not implemented |

### `pytdx.crawler`

`pytdx.crawler` focuses on historical professional financial files:

- list available financial files;
- download `gpcw*.zip`;
- parse `.zip` / `.dat` financial files;
- export parsed data through `hqreader -d hf`.

`marketd` status: not implemented.

### `pytdx.trade`

`pytdx.trade` does not provide native trading by itself. It wraps `trade.dll` through `TdxTradeServer` and exposes login, query, order, cancel, quote, and margin repayment APIs. The pytdx docs explicitly present it as a risk-bearing wrapper.

`marketd` status: out of scope. `marketd` is a market data daemon, not a trading client.

## mootdx

`mootdx` is a higher-level wrapper around 通达信/TDX data access. Compared with `pytdx`, it is more DataFrame- and CLI-oriented, provides cleaner factories, frequency aliases, adjustment options, and convenience commands.

### Package Layout

| Module | Capability | Notes for marketd |
| --- | --- | --- |
| `mootdx.quotes` | Online quote/K-line/minute/F10/financial access | Higher-level reference for operator UX |
| `mootdx.reader` | Local TDX directory/file reader | Useful for path conventions and exported formats |
| `mootdx.affair` | Financial file list/download/parse | Possible future financial-data capability |
| CLI | `quotes`, `reader`, `affair`, `bestip` | Useful model for commands and server selection |

### `mootdx.quotes`

Typical usage:

```python
from mootdx.quotes import Quotes

client = Quotes.factory(market="std", multithread=True, heartbeat=True)
client.quotes(symbol=["600519", "000001"])
client.bars(symbol="600519", frequency=9, offset=10)
```

Main capabilities:

| Capability | Purpose |
| --- | --- |
| `Quotes.factory(market="std")` | 标准市场客户端，主要用于沪深 A 股 |
| `Quotes.factory(market="ext")` | 扩展市场客户端 |
| realtime quotes | 实时行情快照 |
| `bars` / `k` / `ohlc` | K 线查询 |
| `index` | 指数 K 线 |
| `minute` / historical minute | 分时 |
| transaction records | 分笔 |
| company info | F10/公司信息 |
| xdxr | 除权除息 |
| finance info | 财务信息 |
| `bestip` option | 自动选择较快 TDX server |

Frequency aliases and adjustment support are a major convenience layer:

| mootdx concept | Meaning |
| --- | --- |
| `1m`, `5m`, `15m`, `30m`, `1h` | intraday K-line frequencies |
| `day`, `week`, `mon`, `3mon`, `year` | daily and larger periods |
| `adjust="qfq"` | 前复权 |
| `adjust="hfq"` | 后复权 |

`marketd` status:

| mootdx quotes feature | marketd status |
| --- | --- |
| standard A-share realtime quote | Implemented through `marketd quote` |
| server bestip selection | Partially implemented through `marketd quote-probe` |
| online K-line | Not implemented |
| online minute/time sharing | Not implemented |
| online transaction records | Not implemented |
| qfq/hfq adjustment | Not implemented |
| F10/company/finance online data | Not implemented |
| extended market quote | Partially implemented through `marketd exquote` |

### `mootdx.reader`

Typical usage:

```python
from mootdx.reader import Reader

reader = Reader.factory(market="std", tdxdir="C:/new_tdx")
reader.daily(symbol="600036")
reader.minute(symbol="600036")
reader.fzline(symbol="600036")
```

Main capabilities:

| Capability | Purpose |
| --- | --- |
| `Reader.factory(market="std", tdxdir=...)` | 读取标准市场本地 TDX 目录 |
| `daily` | 日线 |
| `minute` | 1m/5m 分钟 K 线 |
| `fzline` | 分时线 |
| extended daily | 扩展市场日线 |
| block/custom block | 普通板块、自定义板块 |
| custom block write | 写入自定义板块 |

`marketd` status:

| mootdx reader feature | marketd status |
| --- | --- |
| daily `.day` import | Implemented |
| 1m/5m import | Implemented |
| fzline | Not implemented |
| block/custom block | Not implemented |
| custom block write | Not implemented |

### `mootdx.affair`

`Affair` handles professional financial files:

```python
from mootdx.affair import Affair

files = Affair.files()
Affair.fetch(downdir="tmp", filename="gpcw19960630.zip")
Affair.parse(downdir="tmp")
```

Main capabilities:

| Capability | Purpose |
| --- | --- |
| list files | 获取远程财务文件列表 |
| fetch file | 下载单个 `gpcw*.zip` |
| fetch all | 批量下载 |
| parse | 解析财务文件 |
| export | 保存 CSV / Excel 等 |

`marketd` status: not implemented.

### mootdx CLI

Common commands:

| Command | Purpose |
| --- | --- |
| `mootdx quotes` | 在线行情/K 线导出 |
| `mootdx reader` | 本地 TDX 文件读取和导出 |
| `mootdx affair` | 财务文件下载/解析 |
| `mootdx bestip` | 测速并选择 TDX server |

CLI output commonly supports CSV, JSON, Excel, HDF5, depending on command and options.

## pytdx vs mootdx

| Dimension | pytdx | mootdx |
| --- | --- | --- |
| Primary role | Lower-level TDX protocol implementation | Higher-level convenience wrapper |
| API style | Closer to raw TDX calls | Factory-based, DataFrame-oriented |
| Realtime A-share quote | `TdxHq_API.get_security_quotes` | `Quotes.factory("std").quotes(...)` |
| Extended market | `TdxExHq_API` | `Quotes.factory("ext")` |
| Local reader | `pytdx.reader` and `hqreader` | `Reader.factory(...)` |
| Financial data | `pytdx.crawler` | `Affair` |
| Adjustment | Mostly caller-managed | qfq/hfq convenience options |
| Server selection | Host lists and optional connection management | CLI/API convenience around bestip |
| Best use as reference | Protocol details and parsers | Product shape, CLI UX, data workflow |

## Replacement Priorities for marketd

### Implemented

- Local `.day` daily bars.
- Local `.lc1` / `.1` 1-minute bars.
- Local `.lc5` / `.5` 5-minute bars.
- ClickHouse bootstrap and imports for canonical OHLCV facts.
- Standard A-share realtime quote snapshots for `sh` / `sz`.
- Extended market list and single instrument quote through TDX `exhq`.

### High-value next candidates

| Candidate | Source analogue | Reason |
| --- | --- | --- |
| TDX server bestip selection | `mootdx bestip` | Improves realtime quote reliability |
| Online security list | `pytdx.hq.get_security_list` | Useful for full-market realtime sweeps |
| Online K-line fetch | `pytdx.hq.get_security_bars`, `mootdx.bars` | Complements local-file imports |
| 分时线 local/import | `mootdx.reader.fzline` | Distinct from OHLCV minute bars |
| Block/custom block reading | `pytdx.reader`, `mootdx.reader` | Useful for sector/index membership workflows |
| Financial file download/parse | `pytdx.crawler`, `mootdx.affair` | Builds fundamental data plane |

### Keep out of scope

- `pytdx.trade` / `trade.dll` trading operations.
- Automatic destructive schema replacement.
- Storing cross-row derived metrics in canonical fact tables.

## Current marketd realtime quote contract

Command:

```bash
go run ./cmd/marketd quote \
  --symbol sh:600519 \
  --symbol sz:000001 \
  --server 180.153.18.170:7709
```

JSON fields:

| Field | Meaning |
| --- | --- |
| `market`, `symbol` | `sh` / `sz` and six-digit code |
| `price`, `last_close`, `open`, `high`, `low` | Decimal prices |
| `server_time` | TDX server quote time |
| `volume`, `current_volume`, `amount` | Volume and turnover fields |
| `sell_volume`, `buy_volume` | Sell/buy volume fields from TDX payload |
| `bids`, `asks` | Five price/volume levels |

Validation:

- Supports `sh` and `sz`.
- Rejects `bj` in the current implementation.
- Accepts `--symbol sh:600519`, `--symbol sz:000001`, or inferred `--symbol 600519`.
- Uses `--server host:port` for TDX HQ server override.
