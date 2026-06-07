## Context

`marketd` stores canonical A-share daily, 1-minute, and 5-minute OHLCV bars as unadjusted facts. The read plane exposes these facts through `/api/v1/bars`, while live TDX protocol reads are isolated under `/api/tdx/...`.

The TDX standard HQ path already decodes xdxr corporate-action rows with fields needed for common adjustment calculations, including cash dividend, rights issue price, bonus/transfer shares, rights shares, and split/shrink ratios. That data is currently only returned from live provider calls and is not persisted.

Adjusted prices are not source facts. They depend on a full symbol history of corporate actions and a chosen anchor policy. Computing them only from the requested date window can produce wrong values because future corporate actions affect older qfq prices.

## Goals / Non-Goals

**Goals:**

- Persist normalized TDX xdxr corporate-action events.
- Generate rebuildable daily qfq and hfq adjustment factors from raw daily bars and xdxr events.
- Keep canonical OHLCV fact tables unadjusted.
- Serve adjusted daily, 1-minute, and 5-minute OHLC through `/api/v1/bars?adjust=qfq|hfq`.
- Make the refresh workflow explicit and observable through existing task run, watermark, and quality issue patterns.
- Keep the implementation in Go with no runtime Python, pandas, mootdx, or external factor-provider dependency.

**Non-Goals:**

- No full adjusted K-line materialized tables in this change.
- No adjustment for indexes or extended-market instruments.
- No automatic live TDX xdxr request inside `/api/v1/bars`.
- No mutation of existing canonical OHLCV tables.
- No replacement for provider endpoints under `/api/tdx/...`.
- No storage of third-party qfq/hfq factors as authoritative data.

## Decisions

### Store xdxr Events Separately From Bar Facts

Add a normalized xdxr event table keyed by `market + symbol + event_date + category`. Store the decoded TDX fields as nullable numeric columns, for example `fenhong`, `peigujia`, `songzhuangu`, `peigu`, `suogu`, `fenshu`, and `xingquanjia`.

Rationale:

- xdxr rows are source-like corporate-action events, not OHLCV bars.
- They can be refreshed independently from local `.day/.lc1/.lc5` imports.
- Keeping events visible makes factor generation auditable.

Alternative considered: calculate factors from live `/api/tdx/hq/xdxr` on every adjusted query. Rejected because `/api/v1` is the stable ClickHouse-backed query surface and must not depend on live TDX upstream availability.

### Store Daily Factors, Not Adjusted K-line Tables

Add a narrow daily factor table:

```text
a_share_adjust_factors_1d:
  market
  symbol
  trade_date
  qfq_factor
  hfq_factor
  computed_at
```

Adjusted OHLC values are produced by joining raw bars to factors at query time. Minute bars join by `market + symbol + trade_date`.

Rationale:

- Factor generation is the cross-row stateful part; multiplying OHLC is cheap and deterministic.
- One daily factor per trading date supports daily and minute bars without materializing huge adjusted minute tables.
- Rebuilding a narrow factor table is simpler than rewriting full adjusted K-line history.

Alternative considered: materialize `a_share_bars_1d_qfq`, `a_share_bars_1d_hfq`, and minute equivalents. Rejected because it duplicates large fact tables and creates hidden refresh cost.

### Use Explicit Refresh Commands

Introduce operator commands for:

```text
marketd refresh-tdx-xdxr --symbol sh:600519 --server ...
marketd refresh-adjust-factors --market sh --symbol 600519
```

Bulk variants can be added with the same batching and ops patterns used elsewhere. Local TDX OHLCV imports do not implicitly refresh xdxr or factors.

Rationale:

- Operators control when to pay network and recomputation cost.
- Failures are visible through `task_runs`, `watermarks`, and `data_quality_issues`.
- Backfills and factor refreshes have different dependency profiles.

Alternative considered: trigger factor refresh after every daily import. Rejected because imports may be partial, xdxr may be unavailable, and hidden side effects make bulk backfills harder to reason about.

### Define Adjustment Semantics Around Latest Available Trading Date

For `qfq`, the latest available trading date for the refreshed symbol has factor `1.0`; older dates accumulate later corporate-action ratios.

For `hfq`, the earliest available trading date for the refreshed symbol has factor `1.0`; later dates accumulate corporate-action ratios forward.

For ordinary xdxr category `1`, calculate the theoretical ex-rights previous close using:

```text
theoretical_preclose =
  (prev_close * 10 - fenhong + peigu * peigujia)
  / (10 + peigu + songzhuangu)
```

The event ratio is:

```text
event_ratio = theoretical_preclose / prev_close
```

Use `NULL` factor values when required inputs are missing, non-positive, or not supported. Query results for adjusted mode skip adjustment for rows with missing factors only if the API explicitly documents a fallback; the initial contract should fail or surface the missing factor rather than silently returning mixed raw and adjusted prices.

Alternative considered: use Sina qfq/hfq factor APIs like mootdx convenience paths. Rejected because external factors are opaque, outside TDX local/protocol data ownership, and introduce a non-TDX network dependency.

### Preserve Raw Volume And Amount In `/api/v1/bars`

Adjusted queries multiply only `open`, `high`, `low`, and `close`. `volume` and `amount` remain raw fields.

Rationale:

- Volume adjustment conventions differ by provider and use case.
- Amount is already traded value in source currency terms and should not be factor-scaled without a separate contract.
- Keeping raw volume and amount avoids pretending adjusted bars are executable market facts.

Alternative considered: scale volume by the inverse factor. Rejected for the first implementation because it needs a separate product decision and validation against downstream consumers.

## Risks / Trade-offs

- Incomplete xdxr history can produce wrong historical factors -> refresh commands must record watermarks and factor queries must expose missing-factor failures or counts.
- Corporate-action categories beyond ordinary category `1` are heterogeneous -> start with tested category `1` and explicit handling for `suogu` only when validated.
- qfq anchor changes when new trading days arrive -> factor refresh is required after new daily bars or new xdxr events.
- ClickHouse `CREATE TABLE IF NOT EXISTS` will not alter existing deployments -> docs must include a non-destructive migration plan using new tables and explicit refresh.
- Adjusted prices are analytical values, not traded prices -> API docs must state that adjusted mode changes OHLC only and leaves volume/amount raw.

## Migration Plan

1. Add non-destructive bootstrap DDL for xdxr events and daily adjustment factors.
2. Add xdxr refresh command using the existing TDX HQ client and ops metadata patterns.
3. Add factor refresh command that rebuilds symbol histories from raw daily bars and persisted xdxr events.
4. Extend `/api/v1/bars` with `adjust=none|qfq|hfq`; default remains `none`.
5. Update storage and API docs.
6. Backfill xdxr events and factors for selected symbols, validate against known events, then expand to broader market coverage.

Rollback is non-destructive: stop using adjusted queries. Existing raw OHLCV tables remain unchanged.

## Open Questions

- Should adjusted queries return HTTP 409/422 when factors are missing, or return raw rows with an explicit per-row adjustment status?
- Should ETF split/shrink category `11` be included in the first implementation or handled in a follow-up after fixture validation?
- Should the factor refresh command support full-market batching in the first implementation, or start with single-symbol refresh plus documented scripting?
