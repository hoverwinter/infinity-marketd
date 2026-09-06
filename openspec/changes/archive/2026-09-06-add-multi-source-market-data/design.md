## Context

`internal/tdx` owns both local file formats and online wire clients. `querier.TDXProvider` exposes many wire-shaped function dependencies; `/api/tdx` consumers depend on that contract. `internal/ingest/limit_review_ths.go` is a separate product-specific enrichment workflow. Canonical SQL remains exclusively in `internal/clickhouse/query.go`.

AKShare groups fetchers by data product and source (`stock_board_industry_ths`, `stock_board_concept_ths`, `stock_board_industry_em`, `stock_board_concept_em`). It is a protocol reference, not a Go package architecture to copy wholesale. THS daily data is annual JSONP with semicolon-separated rows; concept page IDs differ from quotation IDs. TDX bars use reverse offsets and bounded pages. Eastmoney uses `secid=90.BK...`, paginated `clist/get` catalogs and date-bounded `kline/get` JSON with open/close/high/low field order.

## Goals / Non-Goals

**Goals:** Deliver and register all three sources: TDX security/index bars; THS and Eastmoney industry/concept index daily bars and current board discovery/resolution. Expose all through the existing common HTTP/CLI contract with source-specific protocols fully contained in adapters.

**Non-Goals:** All AKShare products, automatic cross-source fallback, rewriting TDX binary decoders or imports, persisting index data, global index equivalence, adjustment, or additional THS/Eastmoney minute/equity products without their own verified contracts. Three-source integration does not imply every provider supports every data product.

## Decisions

1. **Source-owned code, capability-owned contracts.** Add `internal/marketdata` for DTOs, optional `BarsProvider`/`BoardProvider` interfaces, validation/errors and an immutable registry. Add `internal/ths` for THS HTTP transport and board/history parsing. Put the TDX adapter in `internal/tdx/marketdata.go`, reusing its existing clients. This avoids moving a mature package or forcing THS to implement dozens of TDX functions. Querier is the composition root; adapters never depend on querier, ingest or storage.
2. **Explicit identity.** Bars use `(provider, kind, market, symbol)`. `kind` is `index` or `security`; THS uses market `board` and the quotation symbol, TDX uses explicit exchange plus symbol. Board discovery yields source-scoped `kind`/`code`/`name`; resolving a board returns its quotation `instrument`. THS concept `301558` resolves to `885611`, never by numeric inference. Provider is required; no name-based cross-provider aliasing.
3. **A data range, not a wire cursor.** Bars accept inclusive `since`/`until` dates, period and `adjust=none`; at most ten years and no future dates. THS supports `1d`; TDX adapts `1d`, `1m`, `5m`, `15m`, `30m`, `60m`. Adapters own annual/cursor pagination, return sorted unique rows and reject conflicting duplicates, malformed numbers, impossible OHLC or incomplete pagination. TDX scan caps are errors; exhausted upstream history is disclosed as a warning, never a claim of complete market history.
4. **Explicit units.** Responses retain upstream volume with a named unit (`hand` for TDX HQ; `provider_native` for THS board bars until independently verified) and amount in CNY. No implicit lot conversions. Dates and intraday timestamps use Asia/Shanghai. Daily timestamps are dates; intraday timestamps include `+08:00`. An index has no adjustment semantics and none of the new adapters silently substitutes adjusted data.
5. **Separate live access.** `/api/providers` lists installed capabilities; `/api/providers/{provider}/bars`, `/boards`, `/boards/{kind}/{code}` call online sources only. `/api/v1` remains storage-backed; `/api/tdx` remains the protocol escape hatch. Existing TDX dependency injection is reused by the new adapter. Typed errors map invalid input to 400, unknown source to 404, unsupported capabilities to 422, malformed upstream payloads to 502, and unavailable upstreams to 503.
6. **Bounded THS transport.** Reuse `net/http` and existing `x/text` GBK decoding, limit response bytes, enforce context/timeouts and per-client request spacing. Fetch only requested years. Inspect status and payload rather than treating challenge HTML or failed years as empty data. An optional operator-supplied cookie is configured at startup, never supplied through public query parameters or logged. No JavaScript challenge execution. Board lists describe the current page catalog, not historical membership or a guaranteed exhaustive conceptual universe.
7. **Eastmoney source package.** `internal/eastmoney/client.go` owns bounded HTTP/JSON envelope handling (`rc=0` plus non-null `data`); `boards.go` owns industry `m:90 t:2 f:!50` and concept `m:90 t:3 f:!50` catalogs and category-checked resolution; `bars.go` owns `klt=101`, `fqt=0`, `90.BK...` identity and field normalization. Implement existing `BarsProvider` and `BoardProvider`; no common interface or HTTP-route changes are needed. Default discovery MUST contain `eastmoney`, `tdx`, `ths`.
8. **Eastmoney complete catalog scans.** Request `np=1`, `pz=100`, stable code ordering (`fid=f12`, `po=0`) and only `f12/f13/f14`. Determine the effective page size from the first response because the server can cap `pz`. Require unchanged total, expected page lengths, market 90, unique valid codes and exact final row count; cap at 50 pages. Any failure discards the scan. `ResolveBoard` checks the entire requested category before returning `Instrument{index,board,BK...}`; a concept code supplied as an industry code must fail. Scope is `current_provider_catalog`, not historical membership.
9. **Eastmoney daily bounds and schema.** Request only the intersection of each calendar year and the inclusive requested range, with `lmt=1000` (larger than the maximum days in a year). Decode exactly `f51..f61` as date/open/close/high/low/volume/amount plus four unused derived fields. Verify returned code/market, the presence of the klines array, daily dates inside the requested chunk and row-count bounds. Empty arrays are valid; absent/null data is not evidence of empty history. Never skip failed chunks or silently return a saturated limit. Reuse shared OHLC, numeric and conflict validation. Retain native volume with `provider_native` pending independent unit verification and disclose history-coverage limitations.
10. **Three-source composition.** Register Eastmoney alongside the existing two providers. THS cookie configuration must replace only the THS instance, preserving Eastmoney and any custom registry entries. A small immutable registry replacement operation is sufficient; do not add global registration, factories or dynamic plugin loading. All adapters remain reusable from Go without HTTP or storage dependencies. Eastmoney endpoint overrides are constructor options for operators/tests, never public query parameters.

### Delivered capability matrix

| Source | Bars | Board catalog and resolution | Wire identity |
| --- | --- | --- | --- |
| TDX | security/index, `1d/1m/5m/15m/30m/60m` | existing raw TDX block APIs, not the common board capability | exchange plus six-digit symbol |
| THS | board index, `1d` | industry/concept current page catalog | quotation `88xxxx`; concept page ID resolved via `clid` |
| Eastmoney | board index, `1d` | industry/concept fully paginated current provider catalog | `market=board`, `symbol=BK...` → `secid=90.BK...` |

## Risks / Trade-offs

- THS can challenge/limit requests → return explicit upstream errors, support an optional cookie, and keep deterministic fixtures plus opt-in live probes. Do not copy AKShare's broad `except: continue`, which silently loses years.
- THS markup/column count can change → parse the observed structural fields, validate both 11/12-field annual rows, and fail on unknown layouts.
- TDX history retention varies → expose history-boundary warnings and reject non-progressing/over-budget pagination.
- Eastmoney can close connections or return null/challenge responses → report typed upstream errors, retain offline protocol fixtures and opt-in live probes, and record actual live results. A passing fixture test must never be represented as a successful live fetch.
- Eastmoney catalog totals or pagination can change during a scan → validate every page and reject partial or inconsistent snapshots; do not combine data from different hosts mid-scan.
- Existing worktree has unrelated changes → make additive files and narrow edits, preserve current changes.

## Migration Plan

Deploy additive adapters, registration and CLI help. Existing consumers require no migration; new callers select `--provider eastmoney` through the existing commands. Source-specific realtime, membership or financial products get new small contracts only when required. The THS limit-review enrichment can later reuse THS transport without changing this bars contract. Rollback removes the new registration; no storage operation is necessary.

## Open Questions

THS/Eastmoney minute data, equity extensions and historical board membership require independent coverage/unit verification before advertising those capabilities. Eastmoney live availability must be recorded separately from implementation and fixture validation.

## Confirmed Follow-up: Daily Board Storage (2026-09-06)

The agreed storage plan is documented in [board-membership-storage.md](../../../../docs/design/board-membership-storage.md). It is a future storage change, not a claim that this online-provider change already persists boards or members.

- Physically separate TDX, THS and Eastmoney into `tdx_block_*`, `ths_block_*` and `dfcf_block_*` tables in `infinity_market`; retain the existing `eastmoney` runtime provider ID.
- Keep three roles per system: daily snapshot records, snapshot-specific board definitions and stock-only membership rows. Do not mix systems in shared tables using a source discriminator or merge same-name boards.
- Store a complete snapshot for every successfully observed board/day, including unchanged days. Do not use baseline-plus-events, inferred validity intervals or cross-day content-hash references in the first implementation.
- Keep membership rows minimal: snapshot identity, board identity, security market and symbol. Store names/categories in definitions, dates/observation times in snapshot records, and operational failures in existing ops tables.
- Publish only fully validated and completely written snapshots; corrections select a new complete snapshot for that day. Missing observations must remain missing rather than copying another date or system.
- Measure actual compressed storage and query latency before adding storage optimizations. Existing client-local TDX tables require a non-destructive migration plan; no storage DDL or ingestion changes are part of this documentation update.
