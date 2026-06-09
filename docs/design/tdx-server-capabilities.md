# marketd TDX Server Integration Design

整理日期：2026-06-07

本文档描述 `marketd` 如何接入 TDX 行情服务器、当前实现了哪些读取能力，以及这些在线读取能力和 ClickHouse 落库数据产品之间的边界。TDX server 自身能力、协议语义和 public server 样本见 [TDX Server Capability Reference](../reference/tdx-server-capabilities.md)。

## 设计目标

`marketd` 对 TDX server 的定位是 provider/protocol 读取能力，而不是交易所官方行情源，也不是当前 canonical market fact tables 的主写入来源。

目标：

- 提供可调试、可测试的 Go 协议实现，覆盖常用 TDX `hq` 和 `exhq` 读取请求。
- 对 CLI 和 HTTP provider API 返回结构化 JSON，便于验证、研究和后续产品化。
- 把在线 TDX upstream read 和 ClickHouse-backed `/api/v1` 查询隔离。
- 在写入事实表前，先明确 schema、逻辑键、去重、保留周期和查询 contract。

非目标：

- 不把 public TDX server 视为生产级授权行情源。
- 不把在线 quote、分时、分笔、F10、板块、财务摘要等隐式写入 ClickHouse。
- 不把分时点转换成 1 分钟 OHLCV。
- 不在本地文件导入命令中连接远程 TDX server。

## 实现层次

```text
CLI / HTTP provider API
  -> internal/tdx protocol client
    -> TDX hq / exhq upstream server

ClickHouse-backed /api/v1
  -> internal/querier
    -> internal/clickhouse/query.go
```

两条链路保持隔离：

- `/api/v1/*` 查询已落库、可重复读取的数据。
- `/api/tdx/hq/*` 和 `/api/tdx/exhq/*` 是 live upstream read，不写库。
- `marketd import-tdx-*` 只处理本地 TDX 文件和离线包，不连接 TDX server。

## 状态口径

本文中的状态按以下含义使用：

| 状态 | 含义 |
| --- | --- |
| 已实现读取 | `marketd` 已实现请求构造、响应解析和 CLI/内部读取函数 |
| 已实现 provider API | `infinity querier serve` 暴露了对应 `/api/tdx/*` HTTP endpoint |
| 不持久化 | 当前只返回 JSON/内存对象，不写入 ClickHouse canonical fact tables |
| 未启用 | 协议可能存在，但 `marketd` 当前不暴露或不默认使用 |
| 未实现 | `marketd` 当前没有可用实现 |

## 标准行情 hq

标准行情 `hq` client 面向 A 股、指数和标准行情附属数据。`marketd` 使用 `sh` / `sz` / `bj` 作为内部市场标识，并在协议层转换为 TDX market byte。

### CLI 覆盖矩阵

| TDX hq 能力 | marketd 状态 | CLI | 持久化 |
| --- | --- | --- | --- |
| server 探测 | 已实现读取 | `quote-probe` | 不持久化 |
| 多 server fallback | 已实现读取 | 多数 `hq` / `quote` 命令的 `--server` | 不持久化 |
| 证券数量 | 已实现读取 | `quote-sweep --market` 内部使用 | 不持久化 |
| 证券列表 | 已实现读取 | `quote-sweep --market` 内部使用 | 不持久化 |
| 实时行情快照 | 已实现读取 | `quote` / `quote-sweep` | 不持久化 |
| 股票 K 线 | 已实现读取 | `hq-bars` | 不持久化 |
| 指数 K 线 | 已实现读取 | `hq-index-bars` | 不持久化 |
| 当日分时 | 已实现读取 | `hq-minute` | 不持久化 |
| 历史分时 | 已实现读取 | `hq-history-minute` | 不持久化 |
| 当日分笔 | 已实现读取 | `hq-transactions` | 不持久化 |
| 历史分笔 | 已实现读取 | `hq-history-transactions` | 不持久化 |
| F10 目录 | 已实现读取 | `hq-company-categories` | 不持久化 |
| F10 正文 | 已实现读取 | `hq-company-content` | 不持久化 |
| 除权除息 | 已实现读取 | `hq-xdxr` | 不持久化 |
| 财务摘要 | 已实现读取 | `hq-finance` | 不持久化 |
| 板块元数据 | 已实现读取 | `hq-block-meta` | 不持久化 |
| 板块成分 | 已实现读取 | `hq-block` | 不持久化 |

### Provider API 覆盖

`infinity querier serve` 暴露独立的 TDX provider API。完整 endpoint 说明见 [TDX Provider API Reference](../api/tdx.md)。

| 能力 | HTTP endpoint |
| --- | --- |
| 实时 quote | `GET /api/tdx/hq/quotes` |
| server 探测 | `GET /api/tdx/hq/probe` |
| 证券列表 | `GET /api/tdx/hq/securities` |
| K 线 | `GET /api/tdx/hq/bars` |
| 当日/历史分时 | `GET /api/tdx/hq/minute` |
| 当日/历史分笔 | `GET /api/tdx/hq/transactions` |
| F10 目录 | `GET /api/tdx/hq/company-categories` |
| F10 正文 | `GET /api/tdx/hq/company-content` |
| 除权除息 | `GET /api/tdx/hq/xdxr` |
| 财务摘要 | `GET /api/tdx/hq/finance` |
| 板块元数据 | `GET /api/tdx/hq/block-meta` |
| 板块成分 | `GET /api/tdx/hq/block` |

### 市场覆盖

| 市场 | marketd 状态 | 说明 |
| --- | --- | --- |
| `sz` | 已启用 | 实时 quote、证券数量/列表和各类 hq 读取请求可用 |
| `sh` | 已启用 | 实时 quote、证券数量/列表和各类 hq 读取请求可用 |
| `bj` | 已启用 | 单只实时 quote 已实现并验证过 `920*`；证券数量/列表 discovery 通过标准行情 market byte `2` 启用并由协议 fixture 覆盖，但可用性仍取决于所选 public server |

`bj` 相关实现边界：

- `920*`、`8*`、`4*` 代码会按本地规则推断为 `bj`。
- 含 `bj` 的批量 quote 当前会保守拆成单只请求。
- 如果 server 返回不匹配的 fallback 代码，客户端会用 response identity 校验拒绝该结果。
- `quote-sweep --market bj` 使用标准行情证券数量/列表 discovery；如果所选 server 不支持该路径，会返回 source/upstream failure，不会 fallback 到其他来源。
- `internal/tdx/quote_ops_test.go` 覆盖了 `bj` count/list packet path；2026-06-10 对 `180.153.18.170:7709` 和 `60.191.117.167:7709` 的 live 小样本请求返回 read timeout，因此线上可用性仍需按 server 观察。

## 扩展行情 exhq

扩展行情 `exhq` 使用 numeric market id 和 instrument code。`marketd` 将其作为独立 client 实现，不与标准 A 股 `hq` 共享连接、market mapping 或 decoder。

### CLI 覆盖矩阵

| TDX exhq 能力 | marketd 状态 | CLI | 持久化 |
| --- | --- | --- | --- |
| 扩展市场列表 | 已实现读取 | `exquote-markets` | 不持久化 |
| 品种数量 | 已实现读取 | `exquote-count` | 不持久化 |
| 品种列表 | 已实现读取 | `exquote-instruments` | 不持久化 |
| 单品种实时行情 | 已实现读取 | `exquote` | 不持久化 |
| K 线 | 已实现读取 | `exquote-bars` | 不持久化 |
| 当日分时 | 已实现读取 | `exquote-minute` | 不持久化 |
| 历史分时 | 已实现读取 | `exquote-history-minute` | 不持久化 |
| 当日分笔 | 已实现读取 | `exquote-transactions` | 不持久化 |
| 历史分笔 | 已实现读取 | `exquote-history-transactions` | 不持久化 |
| 历史 K 线范围 | 已实现读取 | `exquote-history-bars` | 不持久化 |

### Provider API 覆盖

| 能力 | HTTP endpoint |
| --- | --- |
| 扩展市场列表 | `GET /api/tdx/exhq/markets` |
| 品种数量 | `GET /api/tdx/exhq/instrument-count` |
| 品种列表 | `GET /api/tdx/exhq/instruments` |
| 单品种 quote | `GET /api/tdx/exhq/quote` |
| K 线 | `GET /api/tdx/exhq/bars` |
| 当日/历史分时 | `GET /api/tdx/exhq/minute` |
| 当日/历史分笔 | `GET /api/tdx/exhq/transactions` |
| 历史 K 线范围 | `GET /api/tdx/exhq/history-bars` |

当前 `exhq` 不实现：

- Level-2 认证行情。
- ClickHouse 持久化。
- 和标准 A 股 `hq` 共享连接、market mapping 或 decoder。

## 数据产品边界

`marketd` 当前对 TDX 在线能力的定位是只读 provider/protocol 能力：

- 在线 `quote` / `hq-*` / `exquote-*` 命令返回 JSON 或内部结构。
- `/api/tdx/*` endpoint 返回 live upstream response。
- 这些命令和 endpoint 不写入 ClickHouse。
- 现有 canonical fact tables 仍以本地 TDX 文件导入的 `.day`、`.lc1`、`.1`、`.lc5`、`.5` 为主。
- `client-local` 参考数据由独立 import 命令写入，例如 `import-tdx-gbbq`、`import-tdx-block`、`import-tdx-ex-daily`；这些命令读取本机 TDX 客户端文件，不连接 TDX server。
- 在线 `hq-xdxr` 可用于校验 `gbbq`，在线 `hq-block` 可用于校验系统板块，在线 `exquote-bars` 可用于校验扩展行情日线，但这些 provider reads 不会隐式写入 client-local reference tables。
- 实时快照、高频分笔、F10、板块、财务摘要等如需落库，应先单独设计 schema、逻辑键、分区、去重、保留周期和查询 contract。

这也意味着 `/api/v1` 的 ClickHouse-backed 查询能力不应直接混入 live TDX upstream 请求。对外暴露在线 TDX 能力时，使用独立 provider API 边界：

```text
/api/v1/...        已落库数据查询
/api/tdx/hq/...    标准行情 live upstream read
/api/tdx/exhq/...  扩展行情 live upstream read
```

## 错误处理与可靠性

TDX public server 行为不稳定，因此实现需要保持保守：

- 支持显式 `--server` 或 query parameter 覆盖候选 server。
- 多 server fallback 只用于网络连接、超时和上游不可用等情况。
- 协议解码错误不应被 fallback 静默隐藏，应暴露为实现或上游响应问题。
- quote response 必须校验请求和响应 identity，避免 server fallback 到错误证券。
- public server live 可用性不能作为 CI 唯一验证依据，核心 decoder 需要 fixture 或 fake server 测试。

## 实现位置

主要代码位置：

- `internal/tdx/quote.go`：标准行情实时 quote 请求包、响应解析、价格和成交额解码。
- `internal/tdx/quote_ops.go`：标准行情 server 探测、候选 server fallback、批量会话复用、证券数量/列表和扫盘 workflow。
- `internal/tdx/hq_data.go`：标准行情 K 线、分时、分笔、F10、除权除息、财务摘要和板块读取。
- `internal/tdx/exquote.go`：扩展行情 session、market list 和单品种 quote。
- `internal/tdx/exquote_data.go`：扩展行情品种列表、K 线、分时、分笔和历史读取。
- `internal/cli/cli.go`：`quote`、`quote-probe`、`quote-sweep`、`hq-*` 和 `exquote-*` CLI。
- `internal/querier/tdx_provider.go`：`/api/tdx/hq/*` 和 `/api/tdx/exhq/*` provider API。
- `docs/api/tdx.md`：HTTP provider API contract。

新增市场或新增 TDX 能力前，应先取得 live 或 fixture 样本，再补 parser、测试和文档状态说明。

## 高级在线读能力（add-tdx-advanced-online-apis）

实现状态：均为 live 上游读，**不落 ClickHouse**（fact/derived/ops 均不写）。协议移植自 millken/tdx，正确性由 round-trip / net.Pipe fixture 测试保证，无 live server 验证。

| 能力 | CLI | Provider 端点 | 会话 |
| --- | --- | --- | --- |
| 排序行情列表 | `marketd hq-quotes-list` | `/api/tdx/hq/quotes-list` | 既有 QuoteSession（0x054B） |
| 排行榜 | `marketd hq-top-board` | `/api/tdx/hq/top-board` | 既有 QuoteSession（0x053F） |
| 龙虎榜 | `marketd hq-lhb` | `/api/tdx/hq/lhb` | 既有 QuoteSession（F10 文本解析） |
| SP 板块成分 | `marketd sp-board-members` | `/api/tdx/sp/board-members` | 新 SP 会话（0x122C，需显式 server） |
| 基金 K 线 | `marketd fund-kline` | `/api/tdx/fund/kline` | 新 fund 会话（0x2489，需显式 server） |
| 基金明细 | `marketd fund-detail` | `/api/tdx/fund/detail` | 新 fund 会话（0x2488，需显式 server） |

边界：标准 0x0c 命令复用既有 `QuoteSession`；SP / fund 使用 `internal/tdx/sp_client.go`、`fund_client.go` 的独立握手（SP login 0x2454 + 可选 fund bootstrap 0x23F0），其响应是 0x01 SP 帧。SP/fund 无公共默认 server。
