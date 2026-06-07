# TDX Server Capability Reference

整理日期：2026-06-07

本文档描述通达信行情服务器自身可提供的主要能力和协议边界。它不描述 `marketd` 已经实现了哪些命令；`marketd` 的实现状态见 [marketd TDX Server Integration Design](../design/tdx-server-capabilities.md)。

## 基本定位

TDX server 指通达信客户端协议使用的行情服务器。通达信客户端、`pytdx`、`mootdx`、`millken/tdx` 和 `marketd` 在线行情实现连接的都是这类服务器。

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
      -> 通达信客户端 / pytdx / mootdx / millken/tdx / marketd
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

| 类型 | 常见端口 | 上游主要覆盖 | 标识体系 |
| --- | ---: | --- | --- |
| 标准行情 `hq` | `7709` | A 股、指数、证券列表、K 线、分时、分笔、F10、除权除息、财务摘要、板块文件等 | `sh` / `sz` / `bj` 对应 market byte |
| 扩展行情 `exhq` | `7727` / `7720` | 期货、期权、港股、外盘等扩展市场，具体覆盖取决于 server | numeric market id + instrument code |

`hq` 与 `exhq` 使用不同 server、market 标识和响应解析路径。两者不能共享 A 股市场映射。

## 标准行情 hq

标准行情 `hq` 的多个能力是独立请求。证券列表、实时快照、K 线、分时、分笔、F10 等不是同一个接口返回的不同字段。

### 能力矩阵

| TDX hq 能力 | 上游语义 | 主要边界 |
| --- | --- | --- |
| server setup / heartbeat | 连接后执行协议初始化，部分实现支持心跳 | public server 可能连接成功但业务请求不可用 |
| server 探测 | TCP connect + setup 包可用于可用性探测 | 探测成功不代表所有业务请求都成功 |
| 证券数量 | 按 market byte 返回证券总数 | 不同市场和 server 覆盖不同 |
| 证券列表 | 按 start/count 分页返回证券代码、名称等 | 名称通常需要 GBK/GB18030 类编码解码 |
| 实时行情快照 | 批量读取当前价、昨收、OHLC、成交量额、五档盘口等 | 价格字段使用 TDX 变长整数和差值编码 |
| 股票 K 线 | 按 category/start/count 返回 OHLCV bar | 单次 `count` 通常不超过 800 |
| 指数 K 线 | 与股票 K 线类似，但走独立请求 | 指数和证券请求类型不同 |
| 当日分时 | 返回当日 `price + volume` 分时点 | 不是 1 分钟 OHLCV |
| 历史分时 | 按单个 `YYYYMMDD` 返回历史分时点 | 非交易日、上市前或 server 未保留时可返回空 |
| 当日分笔 | 按 start/count 返回当日分笔成交 | 单次条数存在上限 |
| 历史分笔 | 按单个 `YYYYMMDD` 返回历史分笔成交 | 可用性取决于 server |
| F10 目录 | 返回公司信息目录和文件索引 | 文本字段需要中文编码解码 |
| F10 正文 | 按文件名、offset、length 读取正文片段 | 需要先读目录取得文件名和范围 |
| 除权除息 | 返回送转、分红、配股等事件字段 | 字段口径需按协议样本校验 |
| 财务摘要 | 返回实时协议中的财务摘要字段 | 不等同于专业财务文件 `gpcw*.dat` |
| 板块元数据 | 读取板块文件大小等元信息 | 只说明文件存在和大小 |
| 板块成分 | 读取板块文件内容并解析成员 | 板块文件类别多，字段格式需分别确认 |

### 市场标识

常见标准行情 market byte：

| 市场 | TDX hq market byte | 说明 |
| --- | ---: | --- |
| `sz` | `0` | 深市 |
| `sh` | `1` | 沪市 |
| `bj` | `2` | 北交所，public server 支持程度不稳定 |

北交所相关边界：

- `920*`、`8*`、`4*` 通常可按本地规则推断为北交所。
- 部分标准行情 server 可以返回北交所单只 quote。
- `market byte 2` 的证券数量/列表在 public server 上不一定可用。
- 如果 server 对未知市场做 fallback，客户端必须校验响应中的代码和市场身份。

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

### 能力矩阵

| TDX exhq 能力 | 上游语义 | 主要边界 |
| --- | --- | --- |
| 扩展市场列表 | 返回扩展市场目录 | public server live 可能超时 |
| 品种数量 | 返回 catalog 总数 | 多个 public server 可返回该能力 |
| 品种列表 | 按 start/count 返回品种代码、名称、市场等 | 文本字段通常需要 GBK/GB18030 类编码解码 |
| 单品种实时行情 | 返回扩展市场实时 quote | public server 覆盖和权限差异很大 |
| K 线 | 按 category/start/count 请求扩展市场 K 线 | 不同市场 category 支持不同 |
| 当日分时 | 按品种请求当日分时 | 不是 A 股标准 hq 分时 |
| 历史分时 | 按单个日期请求历史分时 | server 支持度不稳定 |
| 当日分笔 | 按 start/count 请求当日分笔 | server 支持度不稳定 |
| 历史分笔 | 按单个日期请求历史分笔 | server 支持度不稳定 |
| 历史 K 线范围 | 按 start-date/end-date 请求历史 K 线 | 与 hq K 线分页语义不同 |

扩展行情不等同于 Level-2 认证行情，也不等同于券商交易接口。

### Public Server 样本

2026-06-07 live 探测显示，public ExHQ server 能力不完全一致：

| server | 样本结果 |
| --- | --- |
| `47.112.95.207:7720` | TCP 可连；`instrument count` / `instrument list` 可返回；market list、quote/K 线/分时/分笔超时 |
| `47.102.108.214:7727` | TCP 可连；`instrument list` 可返回；quote/K 线/分时/分笔 reset |
| `112.74.214.43:7727` / `120.25.218.6:7727` / `116.205.143.214:7727` / `124.71.223.19:7727` | TCP 可连；可返回 `instrument count`；其他请求超时 |
| `61.152.107.141:7727` / `121.14.110.210:7727` | 当前网络下 TCP 超时 |

因此使用 public ExHQ server 时需要区分：

- 协议中存在某项请求。
- 当前 server 在当前网络和当前时间能返回该请求。
- 返回内容是否完整、实时、延迟或受权限限制。

## 与本地和离线数据的关系

TDX 数据源分三类，工程边界不同：

| Source class | Meaning | Examples |
| --- | --- | --- |
| `client-local` | 已安装 TDX 客户端在本机维护的目录和文件 | `vipdoc/`, `T0002/blocknew/`, 本机 `gbbq`, 本机扩展行情目录如 `Lxxx` |
| `offline-package` | 从 TDX 官方页面或显式远程文件命令下载的 ZIP 或 `.dat` 包 | `hsjday.zip`, `shlday.zip`, `tdxfin.zip`, `tdxgp.zip`, `gpcwYYYYMMDD.zip` |
| `online-provider` | 连接 TDX `hq` / `exhq` server 的请求/响应读取 | `hq-xdxr`, `hq-block`, `exquote-bars`, `/api/tdx/*` |

关键边界：

- 在线 `hq` / `exhq` 返回 live 或 server 保留的请求结果，默认不落库。
- `client-local` 文件来自某台机器的 TDX 客户端状态，完整性取决于客户端版本、下载记录和用户配置。
- `offline-package` 更适合全量 bootstrap/backfill，但仍需校验包清单、文件大小、日期和格式版本。
- 远程财务包清单/下载使用显式 `tdx-fin-files` / `tdx-fin-fetch` 命令，下载后仍按本地文件解析、预检或导入。
- 分时点不是 1 分钟 OHLCV；不能把 server 分时直接当作本地 `.lc1` 一分钟 K。
- 需要落库 server 分时点时，使用显式 `import-tdx-intraday-points` 写入 `a_share_intraday_points`；普通 `/api/tdx/*` live provider 读取不隐式写 ClickHouse。
- 专业财务 ZIP/`.dat` 文件不是 `hq` 的财务摘要字段。
- 在线 `hq-xdxr` 可交叉验证本机 `gbbq`，但不能替代 `client-local` `gbbq` reader。
- 在线 `hq-block` 可读取 server block 内容，但不能替代 `T0002/blocknew` 自定义板块 reader/write。

## 参考实现

可用于交叉验证的开源实现：

- `pytdx.hq`：标准行情协议、证券列表、quote、K 线、分时、分笔、F10、除权除息、财务摘要和板块。
- `pytdx.exhq`：扩展行情协议、扩展市场、品种、quote、K 线、分时和分笔。
- `mootdx`：对 TDX 行情、本地文件和命令行工具做了更高层封装。
- `mirrowall/gotdx`：早期 Go 标准行情原型，主要可参考证券数量/列表请求。
- `millken/tdx`：较新的 Go TDX client，可参考 sorted quote list、top board、board members、fund detail 和 LHB 解析。

更多 TDX 生态库能力对照见 [TDX Library Capability Reference](tdx-python-libraries.md)。

## Advanced online reads (implemented)

Quote list (0x054B), top board (0x053F), F10-derived LHB, SP board members
(0x122C), and fund 7727 (kline 0x2489 / detail 0x2488) are implemented as live
provider reads. Standard 0x0c commands run on the existing HQ session; SP and
fund use a separate handshake (SP login + optional fund bootstrap) and return
0x01 SP frames. SP/fund have no known-good public server defaults — pass an
explicit server. Protocol details: `tdx-advanced-protocol-notes.md`.
