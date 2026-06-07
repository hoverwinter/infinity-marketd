# TDX Library Capability Reference

整理日期：2026-06-07

本文档整理 `pytdx`、`mootdx`、`mirrowall/gotdx` 和 `millken/tdx` 的主要能力，用于 `marketd` 规划和验收通达信能力平替。

目标不是依赖这些库，而是明确：

- 哪些能力 `marketd` 已经用 Go 实现并接入 ClickHouse / HTTP API；
- 哪些能力只是外部库的便利层，`marketd` 不需要照搬；
- 哪些协议能力仍值得从外部项目借鉴。

## Sources

- pytdx docs: https://pytdx-docs.readthedocs.io/zh-cn/latest/
- pytdx standard行情 docs: https://rainx.gitbooks.io/pytdx/content/pytdx_hq.html
- pytdx trade docs: https://pytdx-docs.readthedocs.io/zh-cn/latest/pytdx_trade/
- pytdx reader CLI docs: https://pytdx-docs.readthedocs.io/zh-cn/latest/hqreader/
- pytdx financial crawler docs: https://pytdx-docs.readthedocs.io/zh-cn/latest/pytdx_crawler/
- pytdx archive source: https://github.com/rainx/pytdx
- mootdx GitHub: https://github.com/mootdx/mootdx
- mootdx docs: https://www.mootdx.com
- mootdx PyPI: https://pypi.org/project/mootdx/
- mirrowall/gotdx GitHub: https://github.com/mirrowall/gotdx
- millken/tdx GitHub: https://github.com/millken/tdx
- millken/tdx package docs: https://pkg.go.dev/github.com/millken/tdx

## Current marketd Snapshot

`marketd` is not just a TDX client. It combines:

```text
TDX protocol / local file decoders
  -> ingest orchestration
  -> ClickHouse fact/reference/ops tables
  -> infinity query API and TDX provider API
  -> quote service operational state
```

The strongest part of `marketd` is the data plane and operational contract. External libraries are stronger in ad-hoc SDK convenience and some online ranking/board APIs.

### Implemented

| Area | Current marketd coverage |
| --- | --- |
| Local OHLCV files | `.day`, `.lc1`, `.1`, `.lc5`, `.5` parse/import into canonical ClickHouse bars |
| Offline package import | `import-tdx-vipdoc-zip` for local vipdoc minute packages |
| Client-local reference data | `gbbq`, system block, custom block, custom block write, extended-market daily bars |
| Standard HQ quote | realtime snapshots for `sh` / `sz` / verified `bj`, five-level depth, zlib response handling, TDX variable integer price decoding |
| Standard HQ data APIs | security list/count, K-line, index K-line, minute-time, history minute-time, transaction, history transaction, F10/company, XDXR, finance summary, block meta/content |
| Extended ExHQ data APIs | markets, instrument count/list, quote, K-line, minute-time, history minute-time, transaction, history transaction, history bars |
| Reliability | server probe, bestip cache, multi-server retry, quote sweep, long-running quote service with pools, heartbeat-before-reuse, rate limiting, retry/backoff, durable run/batch state |
| Product query API | `/api/v1/bars`, `/api/v1/intraday-points`, `/api/v1/resolve-symbol`, `/api/v1/health` |
| Live provider API | `/api/tdx/hq/...` and `/api/tdx/exhq/...`, isolated from `/api/v1` and not implicitly persisted |
| Adjustment | XDXR refresh, factor refresh, and `adjust=none/qfq/hfq` bars query when factors are precomputed |
| Financial packages | local `tdxfin.zip` / `tdxgp.zip` parse/import plus remote `gpcw` `tdx-fin-files` / `tdx-fin-fetch` / `tdx-fin-parse` workflow |

### Financial Package Status

The financial package workflow is now split deliberately:

- `import-tdx-fin` imports local `tdxfin.zip` or a fetched single `gpcwYYYYMMDD.zip` package.
- `import-tdx-gp` imports local `tdxgp.zip`.
- `tdx-fin-files` lists remote `gpcwYYYYMMDD.zip` packages from `gpcw.txt`.
- `tdx-fin-fetch` downloads selected or all remote `gpcw` packages with size/MD5 verification.
- `tdx-fin-parse` validates a fetched `gpcw` package without opening ClickHouse.

This covers the `pytdx.crawler` / `mootdx.affair` financial file workflow while keeping database writes explicit.

## pytdx

`pytdx` is the lower-level Python TDX protocol reference. The GitHub project is archived; use it as protocol documentation and historical implementation reference, not as an actively maintained upstream.

### Package Layout

| Module | Capability | Notes for marketd |
| --- | --- | --- |
| `pytdx.hq` | Standard HQ protocol: A-share quotes, bars, minute, transactions, F10, XDXR, finance, block | Main protocol reference for standard A-share online data |
| `pytdx.exhq` | Extended quote protocol for futures/options/HK/external markets | Reference for ExHQ packet shapes |
| `pytdx.reader` | Local TDX file reader | Reference for `.day`, minute files, block, gbbq, historical financial files |
| `pytdx.crawler` | Historical professional financial file list/download/parse | Reference for `tdxfin` / `gpcw` workflows |
| `pytdx.pool` | Connection pool/failover helpers | Operational ideas only |
| `pytdx.trade` | HTTP wrapper around Windows `trade.dll` via `TdxTradeServer` | Out of scope for `marketd` |

### Standard HQ Coverage

| pytdx capability | marketd status |
| --- | --- |
| `get_security_quotes` | Implemented as `marketd quote`, `/api/tdx/hq/quotes`, and `tdx.FetchRealtimeQuotes` |
| `get_security_count` / `get_security_list` | Implemented and used by quote sweep / quote service discovery |
| `get_security_bars` / `get_index_bars` | Implemented as `hq-bars` / `hq-index-bars` and provider API |
| `get_minute_time_data` / `get_history_minute_time_data` | Implemented |
| `get_transaction_data` / `get_history_transaction_data` | Implemented |
| `get_company_info_category` / `get_company_info_content` | Implemented |
| `get_xdxr_info` | Implemented; can be persisted through `refresh-tdx-xdxr` |
| `get_finance_info` | Implemented as online finance summary |
| `get_block_info_meta` / `get_block_info` | Implemented as online block reads |
| heartbeat / auto-retry | Covered at `quotesvc` operational layer, not copied as pytdx API shape |

`get_security_bars` / `get_index_bars` have an 800-row page limit. Longer online history should page by `start=0,800,1600,...`.

### ExHQ Coverage

| pytdx ExHQ capability | marketd status |
| --- | --- |
| `get_markets` | Implemented as `exquote-markets` and `/api/tdx/exhq/markets` |
| `get_instrument_count` | Implemented |
| `get_instrument_info` | Implemented |
| `get_instrument_quote` | Implemented |
| `get_instrument_bars` | Implemented |
| `get_minute_time_data` / `get_history_minute_time_data` | Implemented |
| `get_transaction_data` / `get_history_transaction_data` | Implemented |
| history bars range parser | Implemented as `exquote-history-bars` |

ExHQ live reads are on-demand provider reads. They are not implicitly persisted to canonical market fact tables.

### Reader / Crawler Coverage

| pytdx reader/crawler format | marketd status |
| --- | --- |
| `.day` | Implemented and persisted |
| `.lc1` / `.1` | Implemented and persisted |
| `.lc5` / `.5` | Implemented and persisted |
| `ex_daily` | Implemented as `import-tdx-ex-daily` |
| `gbbq` | Implemented as `import-tdx-gbbq` |
| block / custom block | Implemented as local block import and custom block write |
| `history_financial` / `gpcw` | Implemented as local import and parse-only validation |
| remote financial file download | Implemented as `tdx-fin-files` / `tdx-fin-fetch` / `tdx-fin-parse` |

## mootdx

`mootdx` is a higher-level DataFrame and CLI wrapper around TDX access. Compared with `pytdx`, it is less useful as raw protocol reference but more useful as a product/UX reference.

### Main Capabilities

| mootdx area | Capability | marketd comparison |
| --- | --- | --- |
| `mootdx.quotes` | `Quotes.factory("std")`, quotes, bars, index, minute, transactions, F10, XDXR, finance, block | Protocol coverage mostly implemented in `marketd`; `marketd` returns JSON/ClickHouse-backed results rather than pandas |
| `mootdx.reader` | Local TDX daily/minute/fzline/block/custom block readers | Daily/minute/block/custom block mostly covered; local fzline remains a distinct candidate |
| `mootdx.affair` | Financial file list/fetch/parse/export | `marketd` now covers files/fetch/parse for remote `gpcw` packages; export remains out of scope |
| `mootdx bestip` | Server speed test and config | `marketd quote-bestip` and bestip cache cover this |
| adjustment helpers | `adjust="qfq"` / `adjust="hfq"` convenience | `marketd` uses explicit XDXR/factor refresh plus query-time join; more auditable, less convenient |
| CLI output | CSV/JSON/Excel/HDF5-style export convenience | `marketd` is oriented around JSON, ClickHouse, and HTTP APIs |

### Practical Gap

The real mootdx gap is not raw protocol coverage. It is ad-hoc analyst ergonomics:

- no DataFrame-native API;
- no one-command CSV/Excel export equivalent;
- no "fetch adjusted bars right now" convenience path.

Those are useful only if `marketd` should become a notebook/CLI exploration tool. For the current daemon/data-plane goal, they are secondary.

## mirrowall/gotdx

`mirrowall/gotdx` is a small early Go implementation. It has connection scaffolding and only a narrow standard-HQ surface:

| gotdx capability | marketd comparison |
| --- | --- |
| Connect to HQ server | Implemented with stronger session handling |
| `GetStockCount` | Implemented |
| `GetStockList` | Implemented; marketd decodes names and uses list discovery for sweeps |
| Quote/K-line/minute/F10/XDXR/finance/ExHQ/local files | Not materially covered by gotdx; marketd is far ahead |

`gotdx` is no longer a meaningful feature target. Keep it only as a historical Go protocol reference.

## millken/tdx

`millken/tdx` is the Go project most worth tracking. It is a richer online TDX SDK than `mirrowall/gotdx`.

### Capabilities Worth Comparing

| millken/tdx capability | marketd status |
| --- | --- |
| `Dial`, `DialSP`, `DialEx`, `DialBest`, host probes/cache | `marketd` has HQ/Ex sessions, probe, bestip cache, and quote-service pools; SP-specific mode is not a general public API |
| `GetTick` / `GetTicks` | `marketd quote` covers realtime snapshot with five-level depth |
| `GetBatchQuotes` | `marketd` batches quote requests, but does not expose this compact API shape separately |
| `GetMarketCodes` | `marketd` has HQ security list/count |
| `GetKline` | `marketd hq-bars` / provider API cover standard HQ bars |
| `GetFundKline` | Not implemented as a fund-specialized API; ExHQ generic bars exist |
| `GetTickChart` | Covered conceptually by HQ minute-time / intraday points, but not same API shape |
| `GetTransaction` / `GetHistoryTrade` | Implemented as HQ transactions/history transactions |
| `GetCompanyCategory` / `GetCompanyContent` | Implemented |
| `GetFinance` | Implemented as HQ finance summary |
| `GetXdXr` | Implemented and persistable |
| `GetQuotesList` | Not implemented; high-value candidate for sorted market scans/rankings |
| `GetTopBoard` | Not implemented; high-value candidate for 涨跌幅/量比/换手等榜单 |
| `GetBoardMembers` | Not implemented as SP live board-member API; local and HQ block support exist |
| `GetFundDetail` | Not implemented |
| `GetLHB` | Not implemented; could be derived from F10 content parsing |
| SDK pool | `marketd` quote service has stronger durable orchestration; `millken/tdx` has simpler SDK pooling |

### What millken/tdx Adds Beyond marketd

The important gaps are online convenience endpoints:

- sorted quote lists (`GetQuotesList`);
- top boards / rankings (`GetTopBoard`);
- SP board member query with rich bitmap fields (`GetBoardMembers`);
- fund-specific detail/K-line protocols (`GetFundDetail`, `GetFundKline`);
- LHB parsing from F10 text (`GetLHB`).

These are not storage fundamentals. They are candidates for a future `/api/tdx/hq/...` or `/api/tdx/sp/...` provider API expansion.

## Capability Matrix

| Dimension | pytdx | mootdx | mirrowall/gotdx | millken/tdx | marketd |
| --- | --- | --- | --- | --- | --- |
| Standard realtime quote | Yes | Yes | No meaningful coverage | Yes | Yes |
| Standard K-line/minute/transaction | Yes | Yes | No | Yes | Yes |
| Security list/count | Yes | Yes | Yes | Yes | Yes |
| F10/company | Yes | Yes | No | Yes | Yes |
| XDXR | Yes | Yes | No | Yes | Yes |
| Finance summary | Yes | Yes | No | Yes | Yes |
| ExHQ generic quote/bars/minute/transactions | Yes | Partial/wrapper | No | Partial/fund-oriented | Yes |
| Local `.day` / minute reader | Yes | Yes | No | No | Yes, persisted |
| Local block/custom block | Yes | Yes | No | No | Yes, persisted/write custom |
| Local `gbbq` | Yes | Reader-level | No | No | Yes, persisted |
| Professional financial package parse | Yes | Yes | No | No | Yes |
| Professional financial package download | Yes | Yes | No | No | Yes, for remote `gpcw` packages |
| qfq/hfq convenience | Caller-managed | Yes | No | No | Yes after explicit factor refresh |
| Sorted quote lists / ranking boards | No main focus | Some convenience | No | Yes | Not implemented |
| Fund detail / fund-specific K-line | No main focus | Ex wrapper | No | Yes | Not implemented as specialized API |
| Dragon-tiger list parsing | No main focus | No main focus | No | Yes | Not implemented |
| Trading wrapper | Yes, via `trade.dll` adapter | No main focus | No | No | Out of scope |
| ClickHouse persistence | No | No | No | No | Yes |
| Task runs / watermarks / quality issues | No | No | No | No | Yes |
| Stable HTTP query API | No | No | No | No | Yes |
| Durable quote sweep service | No | No | No | No | Yes |

## Replacement Priorities for marketd

### Finish First

| Candidate | Reason |
| --- | --- |
| Archive completed financial changes after validation | The local import and remote `gpcw` workflow now have separate OpenSpec changes |
| Restore `go test ./...` if unrelated failures appear | Avoid confusing planned capability with delivered capability |
| Keep financial wide tables explicit | Raw financial facts are imported; derived/wide views should remain separate refresh jobs |

### High-Value Protocol Candidates

| Candidate | Source analogue | Reason |
| --- | --- | --- |
| Sorted quote list / market scanner | `millken/tdx.GetQuotesList` | Useful for full-market scan views without first collecting all symbols |
| Top boards | `millken/tdx.GetTopBoard` | Direct support for 涨幅、跌幅、振幅、量比、换手等榜单 |
| SP board members | `millken/tdx.GetBoardMembers` | Rich live board membership/metrics beyond local static block files |
| Fund detail / fund K-line | `millken/tdx.GetFundDetail`, `GetFundKline` | Useful if funds become first-class assets |
| LHB parsing | `millken/tdx.GetLHB` | Can reuse existing F10 content fetch and add a parser |
| Local fzline import | `mootdx.reader.fzline` | Distinct from OHLCV minute bars and may support intraday chart workflows |

### Keep Out of Scope

- `pytdx.trade` / `trade.dll` trading operations.
- Authenticated Level-2 feeds unless there is a separate credential and storage contract.
- Implicit persistence from `/api/tdx/*` live provider reads.
- Automatic destructive schema replacement.
- Adding cross-row derived metrics to canonical fact tables.

## millken/tdx gap status (updated)

`GetQuotesList`, `GetTopBoard`, `GetBoardMembers` (SP), `GetLHB`, `GetFundKline`,
and `GetFundDetail` are now covered by marketd's `internal/tdx` advanced online
APIs (CLI `hq-quotes-list` / `hq-top-board` / `hq-lhb` / `sp-board-members` /
`fund-kline` / `fund-detail` and `/api/tdx/*` provider endpoints). Decoders are
ported from millken/tdx with fixture/`net.Pipe` tests; no live-server validation.
