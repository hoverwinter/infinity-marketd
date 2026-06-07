## Context

`marketd` currently implements the core TDX online read plane:

- standard `hq` quote, security list, K-line, minute-time, transactions, F10 category/content, XDXR, finance summary, and HQ block-file reads;
- `exhq` market, instrument, quote, K-line, minute-time, transaction, and historical reads;
- HQ BestIP cache and `quotesvc` connection pools for long-running quote sweeps.

The remaining gap is not "can marketd connect to TDX" but "can it expose higher-level TDX products that are already present in `millken/tdx`":

- sorted quote lists (`GetQuotesList`);
- top boards/rankings (`GetTopBoard`);
- SP/MAC live board members (`GetBoardMembers`);
- F10-derived Dragon-Tiger list (`GetLHB`);
- fund-specific 7727 reads (`GetFundKline`, `GetFundDetail`);
- SDK-like dial/probe/pool behavior.

These features are live upstream reads. They are not canonical market facts unless a later change designs storage, retention, logical keys, and query contracts.

## Goals / Non-Goals

**Goals:**

- Add testable protocol decoders and request builders for sorted quote lists, top boards, SP board members, LHB parsing, and fund-specific 7727 reads.
- Expose these reads through `marketd` CLI commands and `/api/tdx/*` provider endpoints.
- Reuse existing server selection, BestIP cache, short-read fallback helpers, and `quotesvc` pool concepts where appropriate.
- Keep the user-facing API stable enough for console/scanner workflows: named sort keys, named ranking groups, explicit pagination, and JSON responses.
- Document upstream protocol uncertainty and distinguish server capability from `marketd` implementation status.

**Non-Goals:**

- No ClickHouse market fact tables, derived scan tables, or background persistence for these live reads.
- No broad public SDK layer that duplicates `millken/tdx` or wraps every internal connection object.
- No trading capability.
- No guarantee that every public TDX server supports every request.
- No attempt to fully semantic-map every raw `FundDetail` item in the first implementation.

## Decisions

### 1. Keep Advanced Online APIs In The Provider Boundary

Add CLI commands and `/api/tdx/*` endpoints, not `/api/v1/*`.

Rationale:

- `/api/v1/*` is ClickHouse-backed and reproducible.
- These new calls are live upstream reads with server-specific availability.
- Existing docs already define `/api/tdx/hq/*` and `/api/tdx/exhq/*` as online provider APIs.

Alternative considered: materialize rankings or fund detail immediately. Rejected because retention, keys, and refresh cadence are separate product decisions.

### 2. Implement Decoders Locally Instead Of Importing `millken/tdx`

Use `millken/tdx` as a protocol reference, but keep `marketd` decoders in `internal/tdx`.

Rationale:

- The repo already owns TDX packet framing, fake-server tests, and typed response models.
- Adding a second TDX client dependency would create duplicate connection and retry behavior.
- Local decoders keep protocol support consistent with existing `QuoteSession` / `ExQuoteSession` patterns.

Alternative considered: vendor or call `millken/tdx`. Rejected unless a specific packet proves too risky to reimplement cleanly.

### 3. Split HQ, SP, And Fund Sessions Explicitly

Use separate entry points:

- `hq` standard session for sorted quote lists, top boards, F10 LHB, and existing standard reads;
- SP/MAC session mode for live board members;
- fund-specialized 7727 bootstrap for fund K-line and fund detail.

Rationale:

- These protocols have different handshakes and request families.
- Conflating SP board members with existing HQ block-file reads would hide a real upstream distinction.
- Generic ExHQ bars exist, but `GetFundKline` / `GetFundDetail` are fund-specific 7727 operations and need their own validation.

### 4. Productize Sort And Board Names, Preserve Raw IDs

Expose named request values for common scanner use cases, while returning raw protocol identifiers in the response:

- quote-list sort keys: gain, amount, turnover, volume ratio, speed, price, volume, etc.;
- top-board groups: gainers, losers, amplitude, speed-up, speed-down, volume-ratio, commission-ratio, turnover;
- SP board sort types and bitmap fields as raw values plus known decoded fields.

Rationale:

- Users should not need to memorize magic integers for common commands.
- Raw IDs are still necessary for cross-checking against upstream references and adding unmapped fields later.

### 5. LHB Is A Parser Over Existing F10 Reads

Implement LHB as:

1. fetch F10 categories;
2. find the category whose title matches `资金动向` or a configured alias;
3. fetch the content range;
4. parse Dragon-Tiger records into date, reason, buy/sell seats, amounts, and net amount where available.

Rationale:

- `marketd` already has F10 category/content reads.
- LHB is text parsing, not a new binary packet.
- Parser fixtures can be deterministic and independent from live server availability.

### 6. SDK-like DialBest/Pool Scope Is Limited

Do not expose a general SDK API. Add only the shared internal hooks needed by these features:

- optional `--bestip` / cache flags where the command uses standard HQ servers;
- SP and fund server candidates plus probe helpers if the implementation needs them;
- optional future reuse of `quotesvc` pool internals only for long-running services, not one-shot CLI reads.

Rationale:

- `marketd` is an application/service, not a general-purpose Go SDK.
- Existing BestIP and `quotesvc` code already solve the operational problem for current surfaces.

## Risks / Trade-offs

- [Protocol drift] `millken/tdx` packet formats are reverse-engineered and public servers vary. -> Use fixture/fake-server tests and document live probe results separately from decoder correctness.
- [Feature breadth] Implementing all APIs at once can sprawl. -> Stage implementation by capability group: quote lists/top boards first, then SP board members, then LHB, then fund APIs.
- [Ambiguous field semantics] SP board bitmap fields and fund detail item IDs may not be fully mapped. -> Return raw fields and map only confirmed semantics.
- [Server availability] Some public servers may support HQ core reads but not SP or fund bootstraps. -> Support explicit `--server`, separate server defaults, and clear error messages.
- [API instability] Named sort keys may not cover all upstream IDs. -> Include raw numeric override flags/query parameters for advanced users.
- [Latency and rate limits] Ranking APIs can be used repeatedly by scanner UIs. -> Add count limits, timeouts, and no hidden background polling in this change.

## Migration Plan

This change is additive:

1. Add protocol decoders and fake-server tests.
2. Add CLI commands.
3. Add provider endpoints.
4. Update docs with capability status and examples.

Rollback is non-destructive: remove or stop using the new commands/endpoints. No data migration is required because no new ClickHouse fact tables are introduced.

## Open Questions

- Which exact public SP and fund server candidates should be used by default, and should they have their own BestIP cache files?
- Should SP board member raw bitmap fields be represented as `map[string]any`, fixed struct fields, or both?
- How strict should LHB parsing be when F10 text layout differs across servers or client versions?
- Should fund K-line response reuse the existing `ExBar` shape or expose a fund-specific `FundBar` type even if fields overlap?
