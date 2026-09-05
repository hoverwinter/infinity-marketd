# A 股每日复盘数据层设计与首版实现

更新：2026-09-05。本文区分已实现能力与后续计划；以 [存储 schema](../storage/clickhouse.md#a-股复盘表约定) 和 [HTTP API](../api/README.md#每日复盘-api) 为具体契约。

## 结论和边界

客观复盘数据由 infinity-marketd 的 Go 写入链路管理，长期存放 ClickHouse；infinity-quant/quman 负责工作台、报告、策略解释和人工笔记。旧 JSON 只是一轮迁移的输入，不再作为新线上接口的兼容格式。

同一个最终事件可以来自当前 THS 接口，也可以来自 2016 年后的历史复盘补录。下游只按日期、股票、事件查询，不需要区分两条采集路线。来源、导入批次、订正原因留在既有 ops 和原始归档，不新增候选、证据或来源事实表。

涨停原因保存的是当时记录的归因，不把复盘文本中的推测当作可证明的因果。OCR/文本解析、冲突核对由复盘工作台完成；marketd 只接收明确的结构化最终值。

### 已实现

- 六张复盘表、Go 模型、批量写入，复用 Store、现有 binary 和 ops。
- quman 完整快照的 Go 一次性导入：目录/单文件、日期过滤、显式单位、校验、去重、dry-run。
- 最终事件 JSONL 的 upsert 订正入口。
- 专用表现指数与市场宽度的规范 JSON 导入入口。
- 六个分类只读接口和单日复盘组合接口。
- THS 在线涨停/炸板/跌停池刷新：完整分页、响应日期校验、上海时间转换和严格连续板数。
- TDX 昨日涨停、昨日连板、昨日跌停专用指数历史导入，运行时核对证券目录名称。
- 最长 93 个自然日的 relay/题材订正重建，以及按股票分页的复盘矩阵 API。
- 隔离真实 ClickHouse 的六表写入、FINAL 订正、单日/区间查询、矩阵联调。
- TDX HQ 多条 K 线解码的游标修复，包含普通证券和指数回归测试。

### 尚未实现

- 昨日非 ST 涨停专用指数的 OHLCV 代码验证与在线采集。
- TDX 880005 各市场宽度字段的语义映射与自动写入。
- HTTP 补录写接口、缺字段补丁、删除错误事件、跨日期 relay/题材重新物化（查询时重建已实现）。
- infinity-quant 工作台接入、跨日矩阵页面、Excel 导出。
- 生产库 bootstrap、实际历史写入与线上切换。本次代码验证中的存量导入为 dry-run。

不能把“有表、有导入口”理解为“已经取得 2016 年以来所有完整客观数据”，尤其是历史首封、封单、题材、专用指数及宽度仍存在来源缺口。

## 存量输入和保留范围

盘点输入位于 quman 的 `data/ashare/`：

| 输入 | 本次试读规模 | 使用方式 |
| --- | ---: | --- |
| limit_review/YYYY/MM/DD.json | 112 份 | 近期完整快照，百分数单位 |
| historical_review/YYYY/MM/DD.json | 2594 份 | 2016+ 日线规则回放快照，比率单位 |
| historical_review/evidence/... | 不自动导入 | 工作台证据包，人工确认后生成补录 |
| blogger_archive/... | 不自动导入 | 博主原文、图片、OCR，留在工作台 |
| limit_up/YYYY/MM/DD.json | 不自动导入 | 旧版简化聚合，必要时人工转换 |

近期试读：16,072 条事件、112 条摘要、10,231 条 relay、560 条题材，共 26,975 行。历史试读：180,527 条事件、2,594 条摘要，共 183,121 行。以上均未实际写入、无解析丢行；两套目录有日期重叠，数字不能直接相加当作最终逻辑事实数。

### JSON 到最终数据

| 原数据 | 最终处理 |
| --- | --- |
| limit_up_pool、broken、limit_down | 事件表；分别为 limit_up、open_limit、limit_down |
| code | 去空白、验证六位代码、通过现有 TDX 工具推导 market |
| reason_type、theme_primary、theme_tags | reason_text、主题材、去重标签 |
| order_amount、amount、market_value、turnover_rate、open_num | 封单额、成交额、市值、换手率、开板次数 |
| first_limit_up_time、last_limit_up_time | 上海时间 HH:MM；空值和旧占位符 "-" 转 NULL |
| summary | 摘要表；板数分布由事件重建 |
| relay.height_groups[].stocks | 昨涨停样本；前日板数 >=2 同时进入昨连板组 |
| theme_overview | 题材日聚合表 |
| feedback.top_examples | 截断示例，不充当完整样本 |
| ladder、strong_limit_up、reason_buckets、time_buckets 等 | 展示聚合，不重复存成事实表 |
| name | 不落市场事实，名称走证券主数据 |
| 事件 pct_chg、latest | 不写事件表；价格走已有日线，涨幅走 a_share_daily_derived |
| headline、主观评价、截图、观察名单 | 保留在工作台/原始归档，不迁入市场事实 |

事件 pct_chg 不迁入事件表不是允许丢弃原文件：切换前必须校验日线和派生涨幅的历史覆盖；latest 不能作为一条完整 OHLCV 伪造写入日线。未被规范事实覆盖的旧信息仍保留原始归档。

旧代码的 board_count 是输入板数，不保证在所有 THS 快照里都等于严格连续涨停天数。例如旧 parse_board_count 会把“11天7板”解析为 7。首版原样保留该数，不能据此宣称“连续7板”；严格连板研究应在确认交易日历和每日事件完整后重算，在线适配也必须区分多日多板与连续板数。

## 表结构和用途

六表均在 infinity_market。完整可执行 DDL 和字段类型见 [存储文档](../storage/clickhouse.md#a-股复盘表约定)，不要从旧方案中复制过期建表语句。

| 表 | 逻辑键 | 用途 |
| --- | --- | --- |
| a_share_limit_events | trade_date, event_type, market, symbol | 某天某股发生了什么、记录的原因、题材和封板数据 |
| a_share_limit_daily_summary | trade_date | 保存日报摘要与上一交易日；基本计数查询时校正 |
| a_share_limit_relay_events | trade_date, sample_group, market, symbol | 保存昨日样本的今日结果，是派生明细，不是表现指数 |
| a_share_limit_performance_index_bars_1d | index_code, trade_date | 软件定义的昨日涨停、非 ST 涨停、连板、跌停表现指数 OHLCV |
| a_share_market_breadth_daily | trade_date | 市场涨跌家数、强涨强跌累计数量 |
| a_share_limit_theme_daily | trade_date, theme_name | 保存题材日聚合；排名不是主键 |

共用规则：

- ReplacingMergeTree，无 version/source/updated_at，按 trade_date 年份分区。
- 对同一个逻辑键 upsert 写最终整行；读取用 FINAL 消除物理重复。
- 输入同键完全相同只留一行；同批存在不同内容即报错，禁止随意挑一条。
- 不同批次的覆盖顺序由操作方控制；必须串行处理同键写入。
- 市场事实只放规范化值；summary/relay/theme 明确属于派生或预聚合数据。
- 封单额、成交额、市值用元；换手率用百分数。事件未知盘口字段为 NULL。
- 宽度 up_count、down_count、total_count 必须明确提供；其他字段可缺失为 NULL，未知不能写零。
- 摘要 big_noodle_count、high_level_break_count、strong_theme_count 可空；在线池未提供这些统计时为 NULL，不以零代替未获取。
- 主题聚合主键不带 strength_rank，否则改排名会遗留第二条逻辑记录。

### 专用表现指数

语义代码：

| index_code | 含义 |
| --- | --- |
| prev_limit_up_perf | 昨日涨停表现 |
| prev_non_st_limit_up_perf | 昨日非 ST 涨停表现 |
| prev_ladder_perf | 昨日连板表现 |
| prev_limit_down_perf | 昨日跌停表现 |

这些必须是软件专用指数，不能把昨日涨停股票涨幅简单平均后写入同一序列。每个语义代码只选定一种一致口径和基点，不能把不同供应商的指数点位交替拼接。

2026-09-05 对 TDX 服务器 `180.153.18.170:7709` 的实际证券目录核验结果：

| 语义代码 | TDX 代码 | 目录名称 |
| --- | --- | --- |
| prev_limit_up_perf | 880863 | 昨日涨停 |
| prev_ladder_perf | 880812 | 昨日连板 |
| prev_limit_down_perf | 880751 | 昨日跌停 |
| prev_non_st_limit_up_perf | 未启用 | 未确认同口径的 OHLCV 序列 |

`880864` 实际名称为“昨日振荡”，不是昨日连板；`880863` 没有非 ST 标识，禁止借用。THS 有[昨日非 ST 涨停表现概念页](https://q.10jqka.com.cn/gn/detail/code/308726/)，但概念页 ID 不能直接当作已验证的指数行情代码。

`import-limit-performance-tdx` 每次先核对目录代码/名称，再分页抓取，限制最大 64000 条探测范围，拒绝重复日期、逆序失效、错误身份、无效 OHLCV。请求区间没有数据时报错；供应商历史晚于请求起点时返回 `index_history_starts_late` warning，不伪造早期数据。连板指数本次实测 2004 条，最早 2018-06-05，不能补齐 2016 至该日期的缺口。成交量按 TDX 手数原值保存，成交额为元；规范 JSON 也须遵守相同单位。

同次实测昨日涨停 2444 条（最早 2016-08-12），昨日跌停 1182 条（最早 2021-09-15）。这些只是该服务端此次返回的边界，不是宣称指数成立日期或全部供应端的最大历史。

当前表及 API 保存/返回 OHLCV，不另存跨行涨跌幅。上层用前一交易日该指数 close 计算当日变化；如果前收缺失则显示缺失。个股 relay 用来解释样本结果，不替代此指数。

### 市场宽度与 880005

宽度字段包含涨跌家数、平盘/停牌相关计数、涨跌幅大于 3/5/7% 的累计计数、涨跌停数和总数。不同计数必须使用同一证券范围与时间口径。

已通过现有 `hq-index-bars --market sh --symbol 880005` 路径取得连续多日原始记录。此前第二条记录解码失败，原因是通用 HQ 解码循环中的 Go 短变量声明遮蔽了 pos，而不是证明 880005 使用特殊记录格式。修复后普通指数/证券多条记录均有回归测试。

**原始记录可解码，不等于字段语义已确认。** 不把原始 open/high/low/close/up_count/down_count 擅自映射成涨跌总数或 >7% 数量。下一步需与同日软件界面/可核验文档逐项对照并固定真实样本，之后才能启用在线写入。首版只提供规范 JSON 导入口，不用全市场均值或猜测字段冒充该指标。

[通达信官方函数表](https://help.tdx.com.cn/gspt/docs/markdown/redword/functionlist.html) 明确说明：880005/880006 的 ADVANCE、DECLINE 为正负 0..3% 分档，不是总上涨/下跌家数。这至少排除了直接套用普通指数 `up_count/down_count` 的错误方案；并未证明 >7% 如何映射。当前仍只开放已归一 JSON 的宽度导入。

## Go 代码归属

没有新增 binary、独立服务或生产 Python 依赖：

| 位置 | 职责 |
| --- | --- |
| internal/model/limit_review.go | 六种存储结构 |
| internal/clickhouse/schema.go | 追加六表 DDL |
| internal/clickhouse/limit_review_store.go | 六类批量 INSERT |
| internal/ingest/limit_review.go | 快照归一、订正、去重、ops |
| internal/ingest/limit_review_aux.go | 规范指数/宽度 JSON 校验与导入 |
| internal/ingest/limit_review_ths.go | THS 在线三池刷新 |
| internal/ingest/limit_review_indices.go | 已核验 TDX 专用指数历史导入 |
| internal/cli/limit_review.go | marketd 的六个导入/刷新命令 |
| internal/querier/limit_review*.go | DTO、参数、HTTP、组合复盘和纯计算 |
| internal/clickhouse/query.go | 所有 ClickHouse 读取 SQL |
| internal/tdx/hq_data.go | 通用多条 K 线游标修复 |

参考 quman 的 `backend/pkg/ths` 接口、字段和分页模式，在线适配现在位于现有 ingest 包；没有整包复制或删除 quman 旧链路。原始秒级时间戳校验当日上海日期后转成 HH:MM；`首板` 和 `N天N板` 可直接解释，多日多板则沿真实前一交易日的完整涨停池回溯连续后缀，不把“11天7板”直接写成连续 7 板。THS 响应回退到别的日期、分页不完整或连续板数无法确认都会拒绝本次写入。

### 在线命令

```bash
go run ./cmd/marketd refresh-limit-review --date 2026-09-04 --dry-run

go run ./cmd/marketd import-limit-performance-tdx \
  --index-code prev_ladder_perf --since 2016-01-01 --until 2026-09-04 \
  --server 180.153.18.170:7709 --dry-run
```

命令只接受已收盘日期，当日 15:05 前拒绝写收盘数据。THS 每次完整读取三池，固定请求过滤参数 `HS,GEM2STAR,ST`，过滤范围记在 ops；不要把这一池与非 ST 指数口径混为一谈。若炸板池与收盘池重叠，会标注回封/跌停收盘，封板成功率不重复计算回封样本。事件原因取接口归因，主题材和标签尚未自动分类，保留空值。在线写两张表，其余摘要指标不猜测；relay/题材由读接口使用已有事件和行情重建。

去掉 dry-run 才写库；未增加自动调度。先创建并核对 schema，再运行写入。同日在线刷新会整行覆盖同键历史订正，且不能删除旧池里已不在本次响应的成员，因此只应作为新日期的基础采集，订正后不要盲目重刷。跨表失败不能回滚，在线 runner 的失败记录不保证已提交的部分行数；应按逻辑键对账后重试。

## 一次性迁移

### 1. 冻结与试读

备份原目录，停止对同一目录同时重写。两个目录分别执行，不把不同单位的数据合成一批：

```bash
go run ./cmd/marketd import-limit-review-json \
  --root /path/to/quman/data/ashare/historical_review \
  --since 2016-01-01 --until 2026-09-04 --percent-unit ratio --snapshot-kind historical-replay --dry-run

go run ./cmd/marketd import-limit-review-json \
  --root /path/to/quman/data/ashare/limit_review \
  --since 2016-01-01 --until 2026-09-04 --percent-unit percent --snapshot-kind ths --dry-run
```

percent 是 10% 写 10，ratio 是 10% 写 0.10。只对保留的 relay.today_pct_chg 做单位归一；不按 abs(value)<=1 猜单位，真实 0.5% 不能误放大为 50%。

`--snapshot-kind` 默认 generic；historical-replay/ths 按已核对的旧写入器处理占位 0，不推断来源或单位。历史未取得的换手率、封单、开板次数和扩展统计等保持 NULL；原始 warnings 归入 ops 质量告警。应用中的历史计数只表示已有记录，不承诺全市场完整。

目录模式只接受 YYYY/MM/DD.json，跳过 evidence；目录日期和 JSON trade_date 必须一致。单文件可用 --file。--since 默认 2016-01-01，--until 可省略，日期边界包含当天。

dry-run 不连接 ClickHouse，不写市场表/ops；rows_written 是计划行数而非已落库数。坏日期、字段格式、枚举或冲突阻止整批写入；摘要与池数不符作为 warning 留待对账，不无声丢掉坏行继续宣称完整。

### 2. 创建新表并写入

操作方检查配置目标库和 dry-run 报告后运行现有 bootstrap。只 CREATE IF NOT EXISTS，不替换已有数据；若有人已按旧草案建表，bootstrap 不会自动改列，应先检查 schema 并制定人工迁移方案。

本次将摘要的三个扩展计数字段改为 Nullable(UInt32)。已按早期首版建表的环境不能只重复 bootstrap：操作方需核对现有类型，备份并人工评估这三个字段的可空转换；已有 0 的含义不能自动推定为未知。助手不自动 ALTER 或重建已有表。

```bash
go run ./cmd/marketd bootstrap --config configs/config.yaml
```

首选按日期覆盖清单划分两套目录的负责范围：同一天有近期完整快照时不要再导入规则回放版本。因为 upsert 只替换同键，不删除输入里不存在的旧事件；“先历史后近期”并不能自动消除两者不一致的池成员。需要混合某天数据时，在写入前由工作台输出已合并且核对过的最终快照。

确认范围后按同样参数去掉 --dry-run。2026-09-05 已按冻结清单完成一次生产迁移，详见 [实际迁移记录](ashare-limit-review-migration-20260905.md)，不要直接重放这些写入。所有同键写入串行，订正在基础迁移之后导入，后续不要用旧快照再覆盖人工订正。

跨四张表没有事务：某张表失败时已完成的写入不会回滚。错误任务保留已确认的写入行数，整批水位不推进；重试相同批次，再逐日对账。网络超时下服务端是否已接收需查询确认，计数不是恰好一次保证。

### 3. 核对再切换

通过 HTTP 读取，不从 CLI 直接查 ClickHouse：

- 对照交易日覆盖清单，不把“2016 年起有文件”当成“交易日已全部覆盖”。
- 对照每个日期的逻辑事件键与原因/题材/盘口字段，不能只看总行数。
- 对照摘要计数，核对 open_limit 与 broken_reseal 的统计口径。
- 检查日线和 a_share_daily_derived 的覆盖；历史涨幅和次日表现缺失不能视作零。
- 检查专用指数和宽度的真实覆盖；没有导入就显示空值。
- 保存 dry-run 输出、task_run、issues 和原始归档，验证完成前不删旧 JSON。

## 博主恢复和订正

工作台继续维护三个博主的原文、图片与 OCR 证据。解析后的结果归一到最终事件 contract，而不是让 marketd 依赖某一个博主的模板。

### JSONL contract

每行一个 JSON 对象；下面仅为字段示例，不是真实行情：

```json
{"trade_date":"2016-01-04","mode":"enrich_existing","reason":"核对原始复盘后补全题材","audit_ref":"workbench:review-001","events":[{"code":"000001","event_type":"limit_up","close_status":"sealed","board_count":1,"reason_text":"示例归因","theme_primary":"示例题材","theme_tags":[],"first_limit_minute":null,"last_limit_minute":null,"open_count":null,"seal_order_amount":null,"amount":null,"turnover_rate":null,"market_value":null}]}
```

```bash
go run ./cmd/marketd import-limit-review-corrections \
  --file /path/to/corrections.jsonl --since 2016-01-01 --dry-run
```

规则：

- 材料补录使用 `enrich_existing`，reason 非空，audit_ref 可选，events 非空。Go 会读取当前事件，缺行直接拒绝。上面的其余字段必须与真实已存行一致，示例不能直接用于生产。
- 这是**完整最终事件**，不是只改 reason_text 的补丁。只允许补空原因和空/`未分类`题材，已有非空归因和所有其余字段必须保留，遗漏已存可选值会被拒绝。CLI 通过 `--read-url` 指定 HTTP querier。
- 核心事实订正使用操作员明确授权的 `upsert` 加 `--allow-fact-replacement`，不向材料 HTTP 接口开放。该操作可以清空字段，执行前必须备份并核对完整行。
- code 六位，event_type 和 close_status 合法；涨停事件 board_count >0。
- 字段名错误/不支持的字段会报错，包括试图写事件 pct_chg、latest 或来源列。
- 同一 payload 同键不同内容整批拒绝；校验完全部行后才开始写入。
- 首版不支持 insert_missing、patch_missing_fields，不删除事件，也不能把更改 event_type 当作原键订正。
- reason/audit_ref 进入 task_runs.params，不进入事件表。不需要新增补录表。
- 订正只写事件，不自动更改物化 relay、题材或所有摘要指标；单日和最长 93 自然日的区间读接口会按已实现规则重建，缺少基础事实时的回退见 API。
- 已提供 `POST /api/console/imports/limit-review-corrections`，由现有 infinity-console 显式启用，复用 Go JSONL 校验；`/api/v1` 与普通 querier 仍只读。工作台后端提交单个 JSON 对象，默认预览，确认后 `?dry_run=false`。令牌只保留在受信任后端；当前 Infinity gateway 没有开放匿名写代理。鉴权、限流与失败处理见 `docs/api/README.md`。

## 指数和宽度补录入口

可以直接提供数组；指数也支持 {"bars":[...]}，宽度支持 {"rows":[...]}。

指数字段示例：

```json
[{"trade_date":"2026-09-04","index_code":"prev_non_st_limit_up_perf","open":1000,"high":1030,"low":990,"close":1020,"volume":null,"amount":null}]
```

宽度字段示例：

```json
[{"trade_date":"2026-09-04","up_count":3000,"down_count":2000,"total_count":5500,"up_gt_7_count":120,"down_gt_7_count":45}]
```

这里没提供 flat_count 或 >3/>5 数量，它们保持 NULL。导入检查 OHLC 正数及上下界、金额非负、日期、代码、同键冲突、涨跌计数上界和已知阈值的嵌套。零是已知零，缺失不能靠零占位。

```bash
go run ./cmd/marketd import-limit-performance-json --file /path/to/indices.json --dry-run
go run ./cmd/marketd import-market-breadth-json --file /path/to/breadth.json --dry-run
```

## 上层应用切换

Infinity gateway 使用 HTTP querier，不直接读取 ClickHouse。先接入 /api/v1/limit-review 获取某一天的客观复盘，再将主观复盘结论、笔记、观察清单留在自己的应用数据层。

常见场景对应：

| 场景 | 接口 |
| --- | --- |
| 某天有哪些涨停股票 | limit-events + trade_date + event_type=limit_up |
| 某股票哪些天因什么原因涨停 | limit-events + market/symbol + since/until |
| 昨日跌停股今天表现 | 单日 limit-relay + sample_group=prev_limit_down |
| 某天完整客观复盘 | limit-review |
| 股票为行、日期为列的复盘矩阵 | limit-review-matrix + since/until |
| 专用昨日非 ST 涨停表现曲线 | limit-performance-indices + index_code + 日期区间 |
| 每天涨跌家数和强涨强跌数量 | market-breadth + 日期区间 |

前一交易日由已保存摘要、已有 relay 或单日显式 prev_trade_date 提供，不自然日减一。缺少行情返回未知，不推定停牌。单日与区间 relay 都会按前日最终事件重建样本；日期或事件缺失才回退已有 relay。范围限制 93 自然日，十年研究需分段查询，不把部分补录视为完整全市场样本。

### 手工表格的替代

现有接口可以支撑“题材/股票为行、交易日为列”的表格数据装配：

1. 查区间事件，取股票集合，按交易日填入涨停/炸板/跌停、板数、原因。
2. 查摘要、宽度、专用指数填顶部市场概况；缺失值保留空白。
3. 查已有 bars/分钟数据绘制价格缩略图，不从事件表假造分时。
4. 用户的情绪分析、深水/大回撤等评价保留为应用笔记；要自动标记必须先定义可验证阈值。
5. 明确股票入选规则：只看涨停股，还是涨停/炸板/跌停并集、人工观察名单。后者仍由上层负责。
6. 名称走证券主数据，历史名称需要对应历史日期口径。

矩阵端点默认每页 100 股、最大 500 股，过滤条件选择股票后保留该股票区间内全部事件/relay；日头部包含摘要、宽度、指数、题材。无记录的日期不伪造成交易日，空单元格为未知。

Infinity（本地 quman 仓库）已新增 `/ashare/limit-review/matrix` 页面及 gateway 的 `/api/ashare/limit-review/matrix`、`matrix.xlsx` 两个 GET 入口。页面按 20 股分页，按日期/题材/事件选股，展示涨跌停状态、板数、原因、接力涨跌幅、市场计数和专用指数原始点位；点击详情可跳到已有日 K/分时页。Excel 使用已有 openpyxl，导出全部筛选结果而非当前页，最多 5000 股，包含矩阵、事件明细、接力明细、数据说明；文本按字面保存，证券代码保留前导零，发现跨页计数/日期头变化则拒绝不完整导出。

这还不是截图的完全复刻：未加入每格行情缩略图、历史证券名称或主观笔记。指数目前显示点位，不把点位冒充涨跌百分比。旧整体复盘/历史回放页未自动切换，旧 JSON 未删除。新页面没有旧 JSON 兜底，marketd 未部署或数据未迁移时明确显示缺数/不可用。

## 验收与下一阶段

验收覆盖：Go 单测、命令 dry-run、SQL 构造/参数/分页测试、单日/区间/矩阵、缺失数据、THS 多页和连续板数、TDX 身份与历史缺口。此前在配置服务器创建 `review_test_<timestamp>_market/ops` 隔离库完成真实 CREATE/INSERT/FINAL 和订正联调，测试本身不改生产库、未执行 DROP/清理。后续获确认的正式迁移已执行，见上面的实际记录。隔离测试可复跑：

```bash
MARKETD_REVIEW_INTEGRATION_CONFIG="$PWD/configs/config.yaml" \
  go test ./internal/clickhouse -run TestLimitReviewClickHouseIntegration -v -count=1
```

该测试每次创建新的隔离库，需由操作方管理留存。THS 2026-09-04 实时 dry-run 为 98 事件 + 1 摘要；这不是已写生产数据。

2026-09-05 验证通过 `go test ./...`、相关四包 `go test -race`、`go build ./cmd/infinity ./cmd/marketd ./cmd/infinity-console`、`go vet ./...`、`openspec validate --all`（24 项）及真实库集成测试。新增 HTTP 实库测试可运行 `MARKETD_REVIEW_INTEGRATION_CONFIG="$PWD/configs/config.yaml" go test ./internal/consoleops -run TestLimitCorrectionHTTPClickHouseIntegration -v -count=1`；本轮隔离库为 `review_http_test_1788604267328140000_market/ops`，保留未清理。

Infinity 侧的 gateway/client 29 项 Python 测试和前端生产构建通过。Playwright 使用明确标注的合成数据验证 1440×1000 与 390×844 视口、筛选/分页/详情/下载/空值/错误及历史行情深链接；这不是生产数据已接通的证明。Excel 文件内容通过 openpyxl 回读验证。在线命令仍会出现既有的 `logging cleanup: sync /dev/stderr: bad file descriptor` 日志清理警告；不等于导入失败，本轮没有修改通用日志模块。

下一阶段按依赖顺序：

1. 生产六表已创建，包含可空摘要字段；落实查询与采集的常驻部署。
2. 已完成已有快照的第一次正式迁移及全字段事件对账；继续补历史行情/原因/炸板缺口，保留原始归档。
3. 三种已验证专用指数已写入可用历史；后续用在线命令采集新日期，串行控制同键写入，订正在基础采集后执行。
4. 补非 ST 指数的已验证行情代码和缺失历史；确认 880005 每个字段含义，补真实 fixture 后启用采集。
5. 部署已实现的矩阵页面/导出与可选 HTTP 补录；受信任解析任务已通过 Infinity CLI 完成首批 49 行生产订正，后续扩展版式和核验范围。旧复盘页全面切换、每格行情图、删除/patch 和离线重物化仍为后续范围。

博主材料接入的首批实际结果、代码归属、日期/名称/行列问题和完整操作边界见 [博主复盘材料试点](ashare-review-evidence-pilot-20260905.md)。首版补空原因和明确题材，不把 OCR ready 当作事实核验，也不自动覆盖已有归因。

不删除旧表、不自动清理原始档案。回退是停止新导入并让应用暂时回到旧读路径，不是删除 ClickHouse 数据。
