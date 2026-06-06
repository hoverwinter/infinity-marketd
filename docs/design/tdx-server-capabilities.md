# TDX Server 与 marketd 能力边界

整理日期：2026-06-07

本文档客观描述 TDX 行情服务器可提供的主要能力、这些能力的边界，以及 `marketd` 当前已经实现到什么程度。本文不把 public TDX server 视为交易所官方接口，也不把当前 CLI 能力等同于已落库的数据产品能力。

## 基本定位

TDX server 指通达信客户端协议使用的行情服务器。`pytdx`、`mootdx` 和 `marketd` 当前在线行情实现连接的都是这类服务器。

典型标准行情地址示例：

```text
180.153.18.170:7709
60.191.117.167:7709
119.147.212.81:7709
```

这些服务器通常不是交易所官方公开 API，而是通达信客户端或券商通达信终端使用的行情转发节点。可以把常见链路理解为：

```text
交易所官方行情源
  -> 授权行情商 / 券商 / 转发系统
    -> TDX 行情服务器
      -> 通达信客户端 / pytdx / mootdx / marketd
```

因此：

- TDX server 不是上交所、深交所、北交所的官方公共接口。
- 协议没有稳定的官方公开文档，实践中主要参考开源实现、抓包样本和 live server 行为。
- 可用性取决于网络、server 状态、交易/非交易时段、限流策略和节点覆盖。
- 不同 server 可能超时、断开、返回延迟数据、返回空数据或只支持部分请求。
- 适合作为研究、校验、轻量抓取和工具辅助数据源。
- 正式生产级行情服务应优先使用授权行情源、券商正式接口或行情商正式接口。

## Server 类型

TDX 生态中至少有两类常用行情服务器：

| 类型 | 常见端口 | 上游主要覆盖 | marketd 当前定位 |
| --- | ---: | --- | --- |
| 标准行情 `hq` | `7709` | A 股、指数、证券列表、K 线、分时、分笔、F10、除权除息、财务摘要、板块文件等 | 已实现多项只读请求和 CLI，不写入 ClickHouse |
| 扩展行情 `exhq` | `7727` / `7720` | 期货、期权、港股、外盘等扩展市场，具体覆盖取决于 server | 已实现协议请求/解析和 CLI；public server live 可用性不完整 |

`hq` 与 `exhq` 使用不同 server、market 标识和响应解析路径。`marketd` 也将两者作为独立 client 实现，不混用市场映射。

## 状态口径

本文中的状态按以下含义使用：

| 状态 | 含义 |
| --- | --- |
| 上游能力 | TDX 协议或开源实现中存在对应请求，具体 public server 是否可用需要实测 |
| 已实现读取 | `marketd` 已实现请求构造、响应解析和 CLI/内部读取函数 |
| 已验证样本 | 在指定日期和 server 上实测过，不能外推为所有 server 的稳定承诺 |
| 不持久化 | 当前只返回 JSON/内存对象，不写入 ClickHouse canonical fact tables |
| 未实现 | `marketd` 当前未提供可用实现或未启用该路径 |

## 标准行情 hq

标准行情 `hq` 的多个能力是独立请求。证券列表、实时快照、K 线、分时、分笔、F10 等不是同一个接口返回的不同字段。

### marketd 覆盖矩阵

| TDX hq 能力 | marketd 状态 | CLI | 说明 |
| --- | --- | --- | --- |
| server 探测 | 已实现读取 | `quote-probe` | TCP connect + TDX setup 探测，返回延迟和最快可用 server |
| 多 server fallback | 已实现读取 | 多数 `hq` / `quote` 命令的 `--server` | 按候选顺序尝试；协议解码错误不会被 fallback 隐藏 |
| 证券数量 | 已实现读取 | `quote-sweep --market` 内部使用 | 当前用于 `sh` / `sz` 发现 |
| 证券列表 | 已实现读取 | `quote-sweep --market` 内部使用 | 当前用于 `sh` / `sz`；名称按 GB18030/GBK fallback 解码 |
| 实时行情快照 | 已实现读取，不持久化 | `quote` / `quote-sweep` | 支持批量请求、连接复用和五档盘口字段 |
| 股票 K 线 | 已实现读取，不持久化 | `hq-bars` | 单次请求 `count <= 800`，长区间由调用方分页 |
| 指数 K 线 | 已实现读取，不持久化 | `hq-index-bars` | 与股票 K 线走独立请求 |
| 当日分时 | 已实现读取，不持久化 | `hq-minute` | 返回分时点，不等同于 1 分钟 OHLCV |
| 历史分时 | 已实现读取，不持久化 | `hq-history-minute` | 按单个 `YYYYMMDD` 请求，空结果是正常可能结果 |
| 当日分笔 | 已实现读取，不持久化 | `hq-transactions` | 支持 `start` / `count` |
| 历史分笔 | 已实现读取，不持久化 | `hq-history-transactions` | 按单个 `YYYYMMDD` 请求 |
| F10 目录 | 已实现读取，不持久化 | `hq-company-categories` | 文本字段按 GB18030/GBK fallback 解码 |
| F10 正文 | 已实现读取，不持久化 | `hq-company-content` | 按文件名、offset、length 读取 |
| 除权除息 | 已实现读取，不持久化 | `hq-xdxr` | 返回除权除息、送转、分红等事件字段 |
| 财务摘要 | 已实现读取，不持久化 | `hq-finance` | 不包含专业财务文件下载/解析 |
| 板块元数据 | 已实现读取，不持久化 | `hq-block-meta` | 读取板块文件大小等元信息 |
| 板块成分 | 已实现读取，不持久化 | `hq-block` | 读取板块文件内容并解析成员 |

### 市场覆盖

| 市场 | TDX hq market byte | marketd 状态 |
| --- | ---: | --- |
| `sz` | `0` | 实时 quote、证券数量/列表和各类 hq 读取请求可用 |
| `sh` | `1` | 实时 quote、证券数量/列表和各类 hq 读取请求可用 |
| `bj` | `2` | 单只实时 quote 已实现并验证过 `920*`；证券数量/列表 discovery 未启用 |

`bj` 相关边界：

- `920*`、`8*`、`4*` 代码会按本地规则推断为 `bj`。
- 已验证 `bj:920001`、`920001`、`920799` 可在部分标准行情 server 上返回 `market=bj` 的 quote response。
- 含 `bj` 的批量 quote 当前会保守拆成单只请求。
- 如果 server 返回不匹配的 fallback 代码，客户端会用 response identity 校验拒绝该结果。
- `market byte 2` 的证券数量/列表在已探测 server 上未返回可用列表，因此 `quote-sweep --market bj` 不启用在线发现。

### K 线分页语义

TDX K 线请求的分页语义需要和日期区间区分：

- `count` 是单次请求返回的 K 线根数，TDX/pytdx 约定最大值为 800。
- `start` 是从最近一根可见 K 线向历史方向偏移的 bar 位置，不是日期。
- 读取超过 800 根历史 K 线时，应固定 `count <= 800`，按 `start=0,800,1600,...` 分页。
- 800 是单次请求页大小限制，不代表“服务器只保留最近 800 天”。

2026-06-07 样本验证：

- `180.153.18.170:7709` 上，`sh:600519` 日 K 使用 `category=9` 可从 `start=0` 翻页到 `start=5600`，覆盖 `2001-08-27` 至 `2026-06-05`。
- 同一 server 上，`sz:000001` 日 K 可从 `start=0` 翻页到 `start=8000`，覆盖 `1991-04-03` 至 `2026-06-05`。

这些结果只说明对应 server 在测试日期能返回较长历史，不构成所有 server 的保留周期承诺。

### 分时语义

分时请求返回的是分时点，通常是 `price + volume` 形态，不等同于 1 分钟 OHLCV K 线。

历史分时按单个 `YYYYMMDD` 日期请求。完整 A 股交易日通常最多返回 240 个点，但非交易日、上市前日期、server 未保留或不可服务的日期可能返回空。

2026-06-07 样本验证：

- `180.153.18.170:7709` 和 `60.191.117.167:7709` 上，`sh:600519` 的 `2001-08-27` 历史分时均返回 240 个点。
- `180.153.18.170:7709` 上，`sh:600519` 的 `2001-08-24`（上市前）和 `2024-06-08`（周六）返回空。

这些结果说明历史分时并不必然受“最近 800 天”限制，但实现仍必须以实际返回为空/非空为准。

## 扩展行情 exhq

扩展行情 `exhq` 通常覆盖期货、期权、港股、外盘和其他扩展市场。它使用 numeric market id 和 instrument code，不使用 `sh` / `sz` / `bj`。

### marketd 覆盖矩阵

| TDX exhq 能力 | marketd 状态 | CLI | 说明 |
| --- | --- | --- | --- |
| 扩展市场列表 | 已实现读取，不持久化 | `exquote-markets` | public server live 可能超时 |
| 品种数量 | 已实现读取，不持久化 | `exquote-count` | 多个 public server 能返回该能力 |
| 品种列表 | 已实现读取，不持久化 | `exquote-instruments` | 文本字段按 GB18030/GBK fallback 解码 |
| 单品种实时行情 | 已实现读取，不持久化 | `exquote` | public server live 可用性待持续验证 |
| K 线 | 已实现读取，不持久化 | `exquote-bars` | 按 category / start / count 请求 |
| 当日分时 | 已实现读取，不持久化 | `exquote-minute` | 按品种请求 |
| 历史分时 | 已实现读取，不持久化 | `exquote-history-minute` | 按单个日期请求 |
| 当日分笔 | 已实现读取，不持久化 | `exquote-transactions` | 支持 `start` / `count` |
| 历史分笔 | 已实现读取，不持久化 | `exquote-history-transactions` | 按单个日期请求 |
| 历史 K 线范围 | 已实现读取，不持久化 | `exquote-history-bars` | 按 `start-date` / `end-date` 请求 |

当前 `exhq` 不实现：

- Level-2 认证行情。
- ClickHouse 持久化。
- 和标准 A 股 `hq` 共享连接、market mapping 或 decoder。

### public server 样本

2026-06-07 live 探测显示，public ExHQ server 能力不完全一致：

| server | 样本结果 |
| --- | --- |
| `47.112.95.207:7720` | TCP 可连；`instrument count` / `instrument list` 可返回；market list、quote/K 线/分时/分笔超时 |
| `47.102.108.214:7727` | TCP 可连；`instrument list` 可返回；quote/K 线/分时/分笔 reset |
| `112.74.214.43:7727` / `120.25.218.6:7727` / `116.205.143.214:7727` / `124.71.223.19:7727` | TCP 可连；可返回 `instrument count`；其他请求超时 |
| `61.152.107.141:7727` / `121.14.110.210:7727` | 当前网络下 TCP 超时 |

因此当前状态需要区分两层：

- 协议包、decoder 和 CLI 已在 `marketd` 实现。
- public server live 只稳定验证到部分 catalog 能力；quote/K 线/分时/分笔需要交易日、券商账号线路或其他可用 ExHQ server 继续验证。

## 持久化边界

`marketd` 当前对 TDX 在线能力的定位是只读 provider/protocol 能力：

- 在线 `quote` / `hq-*` / `exquote-*` 命令返回 JSON 或内部结构。
- 这些命令不写入 ClickHouse。
- 现有 canonical fact tables 仍以本地 TDX 文件导入的 `.day`、`.lc1`、`.1`、`.lc5`、`.5` 为主。
- 实时快照、高频分笔、F10、板块、财务摘要等如需落库，应先单独设计 schema、逻辑键、分区、去重、保留周期和查询 contract。

这也意味着 `/api/v1` 的 ClickHouse-backed 查询能力不应直接混入 live TDX upstream 请求。若对外暴露在线 TDX 能力，应使用独立 provider API 边界，例如 `/api/tdx/hq/*` 与 `/api/tdx/exhq/*`。

## 实现参考

主要代码位置：

- `internal/tdx/quote.go`：标准行情实时 quote 请求包、响应解析、价格和成交额解码。
- `internal/tdx/quote_ops.go`：标准行情 server 探测、候选 server fallback、批量会话复用、证券数量/列表和扫盘 workflow。
- `internal/tdx/hq_data.go`：标准行情 K 线、分时、分笔、F10、除权除息、财务摘要和板块读取。
- `internal/tdx/exquote.go`：扩展行情 session、market list 和单品种 quote。
- `internal/tdx/exquote_data.go`：扩展行情品种列表、K 线、分时、分笔和历史读取。
- `internal/cli/cli.go`：`quote`、`quote-probe`、`quote-sweep`、`hq-*` 和 `exquote-*` CLI。

新增市场或新增 TDX 能力前，应先取得 live 或 fixture 样本，再补 parser、测试和文档状态说明。
