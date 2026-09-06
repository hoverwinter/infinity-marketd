# 在线行情数据源 API

`/api/providers` 提供按数据产品统一的在线读取接口。默认注册 `tdx`、`ths`、`eastmoney` 三个来源，调用方显式选择；结果来自上游，不写入 ClickHouse，也不通过 `/api/v1/bars` 隐式补数据。

## 代码结构与扩展约定

```text
internal/marketdata/
  types.go       Instrument、BarsQuery/Result、Board、可选能力接口
  validate.go    公共日期/行情校验、错误类型
  registry.go    启动时注册、能力发现、按 source 分发
internal/ths/
  client.go      HTTP 超时、响应大小、请求间隔、Cookie、GBK 解码
  boards.go      行业/概念目录、页面代码 → 行情代码
  bars.go        按年获取指数日线、解析 JSONP
internal/tdx/
  marketdata.go  将共同 K 线请求适配到现有 HQ 协议客户端
  hq_data.go    现有请求、游标分页与二进制解码
internal/eastmoney/
  client.go      HTTP 限流/超时/大小限制、rc/data JSON 响应
  boards.go      行业/概念目录全分页、按分类解析 BK 代码
  bars.go        按年度区间取日线、校验 90.BK... 身份及字段顺序
internal/querier/
  marketdata.go  注册默认数据源、HTTP 路由、错误映射、HTTP 客户端方法
internal/infinitycli/
  providers.go  调用 querier HTTP 的薄 CLI
```

依赖方向：`ths/tdx/eastmoney → marketdata`，`querier → marketdata + 三个来源适配器`，`infinitycli → querier.HTTPClient`。`marketdata` 不认识具体数据源、HTTP 或数据库；适配器不认识 querier、Store 或导入任务。

采用 AKShare 按数据产品和来源拆文件的方式，但不把所有抓取函数放入一个巨大的 Go 接口：

- `Provider` 只声明 `ID()`。
- `BarsProvider` 声明支持的市场/周期及 `Bars(ctx, query)`。
- `BoardProvider` 声明板块分类、目录及 `ResolveBoard(ctx, kind, code)`。

数据源只实现真实支持的能力。TDX 暂时不实现统一板块目录；它原有的 block、分时、逐笔、财务和扩展市场功能仍由 `/api/tdx/...` 提供。已有导入任务和涨停复盘工作流保持各自的数据产品语义。

东方财富已经按这套扩展路径实现；后续接入其他来源时：

1. 在对应来源包放置客户端和产品文件，实现所需的小接口。例如东方财富的 `BK...` 符号、分页和字段换算均由 `internal/eastmoney` 处理。
2. 用 `marketdata.NewRegistry` 注册实例，并在 querier 启动时传入 `WithMarketDataProviders`。不需要增加 provider 分支或复制 HTTP handler。
3. 增加契约测试和显式启用的联网测试。区分协议构造样本与实测数据；未知单位保留为 `provider_native`，实际覆盖和联网结果单独记录。

未来有跨源实时快照、成分股等具体需求时，再增加对应的独立能力接口。不要要求所有 provider 实现无关方法，也不要自动将同名板块合并或在来源失败时切换口径。

## 当前能力

| 来源 | 产品 | 市场/分类 | 周期 |
| --- | --- | --- | --- |
| `ths` | 指数 K 线 | `market=board`, `kind=index` | `1d` |
| `ths` | 板块目录、代码解析 | `industry`, `concept` | 当前页面目录 |
| `tdx` | 证券、指数 K 线 | `sh`, `sz`, `bj`; `kind=security/index` | `1d`, `1m`, `5m`, `15m`, `30m`, `60m` |
| `eastmoney` | 指数 K 线 | `market=board`, `kind=index`, `symbol=BK...` | `1d` |
| `eastmoney` | 板块目录、代码解析 | `industry`, `concept` | 当前分类目录全分页 |

表中列的是适配器实现的能力；具体标的、历史范围与当下网络可用性仍取决于上游。三源接入不意味着每个来源支持所有产品。东方财富的分钟线、个股及交易所指数扩展尚未纳入本次板块指数契约。

统一 K 线只支持 `adjust=none`，不能静默接受前复权/后复权请求。需要既有 TDX 特有操作时使用 [TDX API](tdx.md)。

## 身份与代码

K 线身份是 `(provider, kind, market, symbol)`。`symbol` 是该数据源的行情代码，不能仅凭代码前缀推断交易所或跨源对应关系。

同花顺板块另有 `(provider, kind, code)` 页面身份。例如：

| 名称 | 分类 | 页面 `code` | 行情 `symbol` |
| --- | --- | --- | --- |
| 元件 | industry | 881270 | 881270 |
| 阿里巴巴概念 | concept | 301558 | 885611 |

目录接口返回页面代码，不会为全部概念逐一请求详情以猜测行情代码。先解析目标板块，再把返回的 `instrument` 用于 K 线请求。解析会交叉检查目录名称、详情标题和隐藏字段 `clid`，防止上游重定向到其他板块后取错数据。

东方财富板块代码本身就是行情符号。例如小金属为 `BK1027`、绿色电力为 `BK0715`，上游使用 `secid=90.BK1027`。公共身份为 `provider=eastmoney, kind=index, market=board, symbol=BK1027`；`90` 只留在适配器中。解析仍须检查所选行业/概念分类，不能把存在于概念目录的代码当作行业返回，也不会与同花顺的同名板块合并。

## HTTP

### 数据源能力

```http
GET /api/providers
```

返回 `ProviderInfo[]`，按 ID 排序，每项包含 `id`、`bars`（kind/markets/periods）和 `board_kinds`。未实现的能力返回空数组。

### 板块目录与解析

```http
GET /api/providers/ths/boards?kind=industry
GET /api/providers/ths/boards?kind=concept
GET /api/providers/ths/boards/concept/301558
GET /api/providers/eastmoney/boards?kind=industry
GET /api/providers/eastmoney/boards?kind=concept
GET /api/providers/eastmoney/boards/industry/BK1027
```

目录返回：

```json
{
  "provider": "ths",
  "kind": "concept",
  "scope": "current_page_catalog",
  "boards": [{"kind": "concept", "code": "301558", "name": "阿里巴巴概念"}]
}
```

示例仅显示一项。`current_page_catalog` 表示详情页当前导航目录，不保证包含所有新建概念，也不表示历史板块或历史成分股。概念时间表分页入口实测可能返回 401，本实现不将其失败吞掉并宣称获得完整概念全集。

东方财富返回同一结果结构，但 `scope=current_provider_catalog`，表示通过 `clist/get` 完整读完指定分类的当前目录。请求按代码排序，页大小以首个响应实际条数为准（上游可能把请求的 100 条限制为 20 条）。最多 50 页，逐页检查总数、预期长度、重复代码和 market=90；任何页失败都不返回部分目录。

解析返回：

```json
{
  "provider": "ths",
  "board": {
    "kind": "concept", "code": "301558", "name": "阿里巴巴概念",
    "instrument": {"kind": "index", "market": "board", "symbol": "885611"}
  }
}
```

### K 线

```http
GET /api/providers/ths/bars?kind=index&market=board&symbol=885611&period=1d&since=2026-09-03&until=2026-09-04
GET /api/providers/tdx/bars?kind=index&market=sh&symbol=000001&period=1d&since=2026-09-03&until=2026-09-04
GET /api/providers/tdx/bars?kind=security&market=sh&symbol=600519&period=5m&since=2026-09-03&until=2026-09-04
GET /api/providers/eastmoney/bars?kind=index&market=board&symbol=BK1027&period=1d&since=2026-09-03&until=2026-09-04
```

| 参数 | 约定 |
| --- | --- |
| `kind`, `market`, `symbol` | 必填，显式来源内身份；HTTP 不默认选择证券/指数 |
| `period` | 默认 `1d`；仅接受来源支持的周期 |
| `adjust` | 默认且只支持 `none` |
| `since`, `until` | 必填 `YYYY-MM-DD`，两端包含，分钟线包含整日；起始不早于 1990 年，跨度最多十年，不能请求未来日期 |

不接受 `limit`、`start`、`server`、Cookie 或任意上游 URL 参数，也不接受重复参数，避免调用方误以为截断或复权等选项已生效。服务器、传输选项通过 Go 适配器构造参数配置；旧的 TDX 协议接口仍支持原有参数。

结果包含 `provider`、规范化的 `query`、`timezone`、`volume_unit`、`amount_unit`、`bars`、`warnings`。每条 bar 包含 `time/open/high/low/close/volume/amount`。

- 日线 `time`：`2026-09-04`；分钟线：`2026-09-04T09:35:00+08:00`，时区为 `Asia/Shanghai`。
- 按时间升序；完全相同的重复记录合并，冲突记录报错。非法日期、NaN/Infinity、负值或 OHLC 不相容均报错，即使该行不在最终筛选范围内。
- TDX HQ 的 `volume_unit=hand`；同花顺和东方财富板块为 `provider_native`。这些板块成交量字段尚未独立证实单位，因此不做“手/股”转换，也不能直接与其他来源成交量相加。`amount_unit=CNY`。
- 同花顺只请求涉及的年份，所有年份成功后才返回；404、401、challenge HTML 或错误字段不是“该年没有数据”。即使成功也不宣称年度文件证明了完整历史覆盖。
- TDX 从最新位置向历史翻页，最多 64 页，每页 800 条。达到扫描上限返回 422；上游历史提前结束时返回已取得的区间数据，并在 `warnings` 中说明边界。较早的分钟历史可能无法通过此入口取得，应使用原始 TDX 分页或已有离线数据路径。
- 东方财富将日期区间按年度切分，传入 `klt=101, fqt=0, lmt=1000`，每个区间最多覆盖 366 个日历日。返回字段为日期、开、收、高、低、量、额及四个未采用的派生字段。适配器检查代码/市场、11 个字段、日期归属和行数上限，再映射为共同结果。`klines=[]` 可表示空区间，`data=null` 或缺失 klines 不能视为成功；任一区间失败则整次失败。成功响应也不证明历史覆盖完整。

### 错误

错误体保持 `{"error":"..."}`：

| HTTP | 含义 |
| ---: | --- |
| 400 | 无效身份、日期、未知或重复参数 |
| 404 | 来源未注册或板块不在所选分类目录 |
| 422 | 来源不支持该能力、周期或复权；扫描超过预算 |
| 502 | 上游载荷格式、身份、行情数据或分页进度异常 |
| 503 | 上游 HTTP/网络不可用或请求取消 |
| 504 | 上游/整体请求超过期限 |
| 500 | 未预期内部错误 |

## CLI 与配置

```bash
go run ./cmd/infinity querier providers --url http://127.0.0.1:8808

go run ./cmd/infinity querier provider-boards \
  --provider ths --kind concept

go run ./cmd/infinity querier provider-board \
  --provider ths --kind concept --code 301558

go run ./cmd/infinity querier provider-bars \
  --provider ths --kind index --market board --symbol 885611 \
  --period 1d --since 2026-09-03 --until 2026-09-04

go run ./cmd/infinity querier provider-bars \
  --provider tdx --kind security --market sh --symbol 600519 \
  --period 1d --since 2026-09-03 --until 2026-09-04

go run ./cmd/infinity querier provider-boards \
  --provider eastmoney --kind industry

go run ./cmd/infinity querier provider-board \
  --provider eastmoney --kind industry --code BK1027

go run ./cmd/infinity querier provider-bars \
  --provider eastmoney --kind index --market board --symbol BK1027 \
  --period 1d --since 2026-09-03 --until 2026-09-04
```

各命令均支持既有 `--url`；`--provider` 必填。`provider-bars` CLI 默认 `--kind index`；日期不设隐式默认值。CLI 只调用 querier，不直连行情来源或数据库。

`infinity querier serve` 启动时可从 `INFINITY_THS_COOKIE` 读取操作者提供的 Cookie。默认请求无需 Cookie；不会自动执行 challenge JS 或获取 Cookie，也不会在日志、错误或响应中显示其内容。THS 连接默认间隔 300ms、每次含排队最多 10 秒、单次数据产品请求最多 30 秒、响应最多 4 MiB。并发请求共用一个客户端限速，等待可取消。

东方财富使用独立客户端，沿用相同的资源上限和可取消等待。默认目录服务器为 `https://17.push2.eastmoney.com`，历史服务器为 `https://push2his.eastmoney.com`；可通过 Go 构造参数 `eastmoney.Options{QuoteURL, HistoryURL, Client}` 指定运维/测试所需的端点或传输。公共 HTTP 参数不开放这些选项。上游失败不切换成其他数据源。

`WithTHSCookie` 仅替换注册表中的 THS 实例，保留东方财富、TDX 及自定义来源；通过 `Registry.WithProvider` 生成新注册表，原注册表保持不变。

## 参考与验证

协议参考（2026-09-06 核对）：[AKShare 同花顺行业](https://github.com/akfamily/akshare/blob/main/akshare/stock_feature/stock_board_industry_ths.py)、[同花顺概念](https://github.com/akfamily/akshare/blob/main/akshare/stock_feature/stock_board_concept_ths.py)、[东方财富行业](https://github.com/akfamily/akshare/blob/main/akshare/stock/stock_board_industry_em.py)、[东方财富概念](https://github.com/akfamily/akshare/blob/main/akshare/stock/stock_board_concept_em.py)、[东方财富分页](https://github.com/akfamily/akshare/blob/main/akshare/utils/func.py)。本实现使用 Go 和现有依赖，不安装 AKShare 或 JS 运行时。

自动测试使用本地 httptest；THS 有裁剪实测样本，东方财富使用明确标记为合成的协议样本。默认不访问外网。三源 HTTP 集成测试运行真实适配器、模拟上游传输，并验证无存储访问或跨源替换。显式联网验证命令：

```bash
MARKETD_THS_PROBE=1 go test ./internal/ths -run TestLiveTHS -v -count=1
MARKETD_PROVIDER_TDX_PROBE=180.153.18.170:7709 \
  go test ./internal/tdx -run TestLiveMarketDataTDX -v -count=1
MARKETD_EASTMONEY_PROBE=1 \
  go test ./internal/eastmoney -run TestLiveEastmoney -v -count=1
```

2026-09-06 实测：THS 当前目录返回 90 个行业、351 个概念；元件 `881270` 和阿里巴巴概念 `301558 → 885611` 均成功返回 2026-09-03 至 09-04 两日日线。TDX 上证指数 `sh:000001` 与贵州茅台 `sh:600519` 同区间均返回两条日线。以上验证证明这些请求当时可用，不代表所有指数、分钟周期或历史年份已逐一验证。

东方财富 2026-09-06 后续联网复验通过：上述 Go 探针使用默认客户端与默认端点，完整读取行业目录 496 项、概念目录 504 项，确认 `BK1027` / `BK0715` 分别存在于对应目录，且两者均返回 2026-09-03 至 09-04 两条日线。验收直接请求公开 API，没有使用本地样本或其他数据源替代响应。

东方财富此前的 Python/curl/Go 请求曾因上游关闭连接而失败，同期独立目录请求仍观察到断连。因此，本次成功表示这些目录与历史请求已通过真实取数验收，不保证接口持续可用、所有板块或全部历史覆盖。探针可通过 `MARKETD_EASTMONEY_QUOTE_URL` / `MARKETD_EASTMONEY_HISTORY_URL` 指定其他已确认的东方财富端点进行复验。协议样本仍明确标记为合成数据，与上述联网证据区分。
