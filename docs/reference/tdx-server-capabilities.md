# TDX 行情服务器能力说明

整理日期：2026-06-07

本文档说明 `marketd` 所说的 TDX server 是什么、它和交易所官方行情源的关系，以及 TDX 标准行情/扩展行情服务器提供的主要能力。

## TDX Server 是什么

TDX server 指通达信客户端协议使用的行情服务器。`pytdx`、`mootdx` 和 `marketd` 当前实时行情实现连接的都是这类服务器。

常见标准行情地址示例：

```text
180.153.18.170:7709
60.191.117.167:7709
119.147.212.81:7709
```

这些服务器通常不是交易所官方公开 API，而是通达信客户端或券商通达信终端使用的行情转发节点。

可以把数据链路理解为：

```text
交易所官方行情源
  -> 授权行情商 / 券商 / 转发系统
    -> TDX 行情服务器
      -> 通达信客户端 / pytdx / mootdx / marketd
```

## 它不是交易所公共接口

TDX server 和交易所官方行情接口不是一回事。

| 层级 | 说明 | marketd 当前是否直连 |
| --- | --- | --- |
| 交易所官方行情源 | 上交所、深交所、北交所的官方行情源，通常面向会员、券商、行情商，有授权和专线/专用协议 | 否 |
| 券商/行情商转发 | 从交易所获得授权数据后，再转发给终端或客户系统 | 间接相关 |
| TDX 行情服务器 | 通达信客户端协议使用的行情节点 | 是 |

因此需要明确：

- TDX server 不是官方交易所公共 API。
- 协议没有稳定的官方公开文档。
- 可用性取决于网络、server 状态、限流和券商/行情节点策略。
- 不同 server 可能超时、断开、返回延迟数据或覆盖范围不同。
- 适合研究、校验、轻量抓取和辅助工具。
- 如果要做正式生产级行情服务，应优先使用授权行情源、券商正式接口或行情商正式接口。

## Server 类型

TDX 生态里至少有两类行情服务器：

| 类型 | 常见端口 | 主要用途 | marketd 当前状态 |
| --- | ---: | --- | --- |
| 标准行情 `hq` | `7709` | A 股、指数、股票列表、K 线、分时、分笔、F10、除权除息等 | 已实现部分能力 |
| 扩展行情 `exhq` | `7727` | 期货、期权、港股、外盘等扩展市场 | 已实现 market list 和 single instrument quote |

`marketd` 当前实现聚焦标准行情 `hq`，并提供最小 `exhq` 查询能力。

## 标准行情 hq 能力

标准行情服务器提供多个独立请求能力。证券列表、实时行情、K 线、分时、分笔等是不同请求，不是一个接口返回所有内容。

### 证券数量

能力：

```text
get_security_count(market)
```

用途：

- 获取某个市场的证券数量。
- 后续证券列表分页请求需要知道总数。

`marketd` 状态：

- 已实现请求包构造。
- 已实现响应解析。
- 当前支持 `sh` / `sz`。
- `bj` 需要验证 TDX market mapping 后再支持。

相关代码：

```text
BuildSecurityCountPacket
DecodeSecurityCountResponse
QuoteSession.SecurityCount
```

### 证券列表

能力：

```text
get_security_list(market, start)
```

用途：

- 按市场和 offset 获取证券基础信息。
- 支持全市场 quote sweep 前的 symbol 发现。

返回信息通常包括：

| 字段 | 含义 |
| --- | --- |
| `code` | 证券代码 |
| `name` | 证券名称 |
| `volunit` | 成交量单位 |
| `decimal_point` | 价格小数位 |
| `pre_close` | 前收字段 |

`marketd` 状态：

- 已实现 `sh` / `sz` 证券列表请求和解析。
- 名称字段当前未做 GBK 解码，非 UTF-8 会置空。
- `bj` 证券列表是否走同一路径需要验证。

相关代码：

```text
BuildSecurityListPacket
DecodeSecurityListResponse
FetchSecurityList
```

### 实时行情快照

能力：

```text
get_security_quotes([(market, code), ...])
```

用途：

- 批量获取股票实时快照。
- 返回当前价、昨收、开高低、成交量额和五档盘口。

返回信息包括：

| 字段 | 含义 |
| --- | --- |
| `price` | 当前价 |
| `last_close` | 昨收 |
| `open` / `high` / `low` | 当日开高低 |
| `server_time` | TDX 服务器日内时间 |
| `volume` / `current_volume` | 成交量、现量 |
| `amount` | 成交额 |
| `bids` / `asks` | 五档盘口 |

`marketd` 状态：

- 已实现 `sh` / `sz`。
- 支持多 server candidate。
- 支持失败自动重试。
- 支持批量请求和连接复用。
- 支持 `quote` CLI。
- `bj` 当前明确拒绝，已有 OpenSpec change 跟踪验证和支持工作。

相关代码：

```text
BuildQuoteRequestPacket
DecodeQuoteResponse
FetchRealtimeQuotes
FetchRealtimeQuoteBatches
```

### K 线

能力：

```text
get_security_bars
get_index_bars
```

用途：

- 获取股票或指数 K 线。
- 支持不同周期，例如 1 分钟、5 分钟、日线、周线、月线等，具体取决于 TDX 协议 category。

`marketd` 状态：

- 在线 K 线未实现。
- 本地 TDX 文件导入已支持 `.day`、`.lc1`、`.1`、`.lc5`、`.5`。

### 分时

能力：

```text
get_minute_time_data
get_history_minute_time_data
```

用途：

- 获取当日分时线。
- 获取历史分时线。

注意：

- 分时线通常是 `price + volume` point。
- 它不等同于 1 分钟 OHLCV K 线。

`marketd` 状态：

- 在线分时未实现。
- 本地分时线导入也未实现。

### 分笔成交

能力：

```text
get_transaction_data
get_history_transaction_data
```

用途：

- 获取当日分笔成交。
- 获取历史分笔成交。

`marketd` 状态：

- 未实现。

### F10 / 公司信息

能力：

```text
get_company_info_category
get_company_info_content
```

用途：

- 获取 F10 公司信息目录。
- 获取 F10 正文内容。

`marketd` 状态：

- 未实现。

### 除权除息

能力：

```text
get_xdxr_info
```

用途：

- 获取除权除息、送转、分红等事件信息。

`marketd` 状态：

- 未实现。

### 财务信息

能力：

```text
get_finance_info
```

用途：

- 获取证券财务摘要信息。

`marketd` 状态：

- 在线财务信息未实现。
- 专业财务文件下载/解析也未实现。

### 板块信息

能力：

```text
get_block_info_meta
get_block_info
```

用途：

- 获取板块分类。
- 获取板块成分。

`marketd` 状态：

- 未实现。

## 扩展行情 exhq 能力

扩展行情通常使用不同 server 和端口，常见端口是 `7727`。

典型覆盖：

- 期货。
- 期权。
- 港股。
- 外盘。
- 其他扩展市场。

常见能力：

| 能力 | 说明 |
| --- | --- |
| market list | 扩展市场列表 |
| instrument count/list | 合约或品种数量和列表 |
| instrument quote | 扩展品种实时行情 |
| instrument bars | K 线 |
| minute data | 分时 |
| transaction data | 分笔 |

`marketd` 状态：

- 已实现独立 `exhq` TCP client。
- 已实现扩展市场列表。
- 已实现单个扩展品种实时行情。
- 未实现 instrument count/list、K 线、分时、分笔和历史接口。
- 独立于标准 A 股 `hq` client，不混用 market mapping 和响应解析。

## marketd 当前覆盖矩阵

| TDX server 能力 | marketd 状态 |
| --- | --- |
| `hq` server 探测 | 已实现 |
| `hq` 多 server 重试 | 已实现 |
| `hq` 批量 quote 连接复用 | 已实现 |
| `hq` `sh` / `sz` 实时行情 | 已实现 |
| `hq` `bj` 实时行情 | 未实现，OpenSpec 已 propose |
| `hq` `sh` / `sz` 证券数量 | 已实现 |
| `hq` `sh` / `sz` 证券列表 | 已实现，名称 GBK 解码待补 |
| `hq` `bj` 证券列表 | 未实现，待验证 |
| `hq` 在线 K 线 | 未实现 |
| `hq` 在线分时 | 未实现 |
| `hq` 在线分笔 | 未实现 |
| `hq` F10 / 公司信息 | 未实现 |
| `hq` 除权除息 | 未实现 |
| `hq` 财务信息 | 未实现 |
| `hq` 板块信息 | 未实现 |
| `exhq` 扩展市场列表 | 已实现 |
| `exhq` 单品种实时行情 | 已实现 |
| `exhq` 品种列表 / K 线 / 分时 / 分笔 / 历史 | 未实现 |

## 实现注意事项

### 可用性

公共 TDX server 经常出现：

- connect timeout；
- setup 失败；
- 部分 server 返回慢；
- 不同网络下可用 server 不同；
- 交易时段和非交易时段表现不同。

因此 `marketd` 当前提供：

```bash
marketd quote-probe
```

用于探测候选 server，并提供：

```bash
marketd quote --server A --server B
```

用于多 server retry。

扩展行情提供：

```bash
marketd exquote-markets --server A
marketd exquote --market 47 --code IF1709 --server A
```

### 协议稳定性

TDX 协议不是官方公开稳定协议。实现主要来自：

- pytdx/mootdx 的公开实现。
- live server 样本。
- 本项目测试 fixture。

因此新增市场或新增能力前，应先拿到样本，再写 parser 和测试。

### 数据使用边界

TDX server 数据适合：

- 研究。
- 工具辅助。
- 数据校验。
- 轻量抓取。

不应默认视为：

- 官方交易所行情源。
- 带 SLA 的生产行情。
- 可无限频率抓取的数据源。

正式生产用途应考虑授权行情源或券商/行情商正式接口。

## 后续候选能力

优先级较高：

- 支持北交所 `bj` 实时行情。
- 给证券列表名称增加 GBK/GB18030 解码。
- 增加在线 K 线接口。
- 增加除权除息接口，用于复权和日线派生计算。
- 增加板块信息接口，用于市场分组和扫描。

优先级中等：

- 在线分时。
- 在线分笔。
- 财务摘要信息。
- F10 公司信息。

独立大项：

- 补齐 `exhq` 扩展行情的品种列表、K 线、分时、分笔和历史接口。
- 实时 quote snapshot 持久化到 ClickHouse。
- 长期运行的行情采集服务、连接池、心跳和限速策略。
