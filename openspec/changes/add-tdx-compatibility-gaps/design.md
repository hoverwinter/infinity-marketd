## Context

`marketd` already has three relevant foundations:

- standard HQ provider reads in `internal/tdx/hq_data.go`, `quote.go`, and `hq_advanced.go`;
- SP/fund session support in `sp_client.go` and `fund_client.go`;
- persisted adjustment factors plus `/api/v1/bars?adjust=...` in the read plane.

The requested work fills compatibility gaps without changing the data-plane boundary. Compact batch quotes and tick chart are additional standard HQ packets. SP/fund best-server support is server discovery around existing SP/fund sessions. Labeled fund detail is a decoder/model improvement. One-shot online adjusted bars are a live provider convenience path and must not change the persisted adjusted bars contract.

## Goals / Non-Goals

**Goals:**

- Implement `0x054C` compact batch quotes with typed rows and raw protocol metadata.
- Implement `0x0537` tick-chart reads with `start/count` validation and typed points.
- Add SP and fund server catalogs, probe functions, best-server selection, CLI commands, and provider endpoints.
- Add a fund detail dictionary for confirmed item ids while preserving raw item rows for unknown ids.
- Add one-shot online adjusted HQ bars using live HQ bars plus live XDXR, exposed only under `/api/tdx/*` and `marketd` provider commands.
- Keep tests deterministic with fixtures and fake servers.

**Non-Goals:**

- No ClickHouse schema changes.
- No hidden persistence from `/api/tdx/*`.
- No change to `/api/v1/bars?adjust=...`; that path remains persisted and reproducible.
- No trading APIs.
- No guarantee that bundled public SP/fund candidates are reachable from every environment.

## Decisions

### 1. Add HQ Packets Beside Existing Advanced HQ Code

Place compact batch quote and tick-chart support in `internal/tdx/hq_advanced.go` or adjacent files that reuse `QuoteSession.call` and `buildHQDirectFrame`.

Rationale: both packets use standard HQ framing, and the existing quote-list/top-board code already contains the varint helpers and scanner-style request validation.

Alternative considered: model compact batch quotes as another variant of `Quote`. Rejected because `0x054C` carries a different field set; collapsing it into `Quote` would hide useful fields and create confusing zero values.

### 2. Keep SP/Fund BestIP Separate From HQ BestIP

Add explicit SP and fund server candidates and probe helpers instead of reusing the HQ BestIP cache directly.

Rationale: SP and fund sessions use different handshakes and often different ports. A server that is good for HQ quote snapshots may fail SP or fund bootstrap.

Alternative considered: always require `server` for SP/fund. Rejected for this change because the user explicitly asked for the missing automatic address/best-server capability.

### 3. Fund Detail Uses A Dictionary Plus Raw Fallback

Return both raw rows and decoded rows where ids are known:

- raw `id` and `[6]uint16` values remain stable;
- decoded `name`, `value`, `unit`, and `raw_values` are populated only for confirmed mappings;
- unknown ids are preserved without fabricated labels.

Rationale: the current decoder is protocol-correct but not analyst-friendly. A partial dictionary is useful, but pretending all fields are known would be worse than raw output.

### 4. Online Adjusted Bars Are Provider Reads, Not `/api/v1`

Add `marketd hq-adjusted-bars-online` and `/api/tdx/hq/adjusted-bars` as live provider surfaces. They fetch raw HQ bars and XDXR from the selected upstream, compute adjustment factors in memory for the response window, and return adjusted OHLC only when enough history is available.

Rationale: `/api/v1/bars` is a stable ClickHouse-backed contract. A mootdx-like convenience path is useful, but it belongs in the live provider namespace and must make its non-persistent nature visible.

Alternative considered: make `/api/v1/bars` fall back to live XDXR when factors are missing. Rejected because it would make reproducible queries depend on live TDX availability and could silently mix persisted and live semantics.

### 5. Reuse Existing Adjustment Math Where Practical

Use the existing `internal/adjust` factor-generation logic for online adjusted bars by converting live HQ bars and XDXR rows into the same normalized in-memory inputs.

Rationale: one formula should own qfq/hfq behavior. If the existing generator needs a smaller pure helper to avoid ClickHouse-oriented assumptions, extract that helper without changing persisted behavior.

## Risks / Trade-offs

- [Server candidates rot] Public SP/fund servers can go stale. -> Keep explicit server override, expose probes, and document candidate status as best-effort.
- [Unknown fund fields] Fund detail item semantics may be incomplete. -> Preserve raw items and decode only confirmed ids.
- [Online adjustment history window] Correct qfq/hfq needs enough raw bars around XDXR events. -> Require a sufficient fetch window or return an explicit missing-factor error rather than silently returning raw prices.
- [Protocol fixture drift] Reverse-engineered packet layouts can diverge. -> Add decoder fixtures, fake-server round trips, and docs that separate fixture correctness from live-server validation.
- [API sprawl] More live endpoints can look like product APIs. -> Keep them under `/api/tdx/*` and document that they are non-persistent provider reads.

## Migration Plan

This change is additive:

1. Add protocol models, decoders, and tests.
2. Add CLI commands and provider functions.
3. Add HTTP endpoints.
4. Update docs and run `go test ./...` plus `openspec validate --all`.

Rollback is non-destructive: stop using the new commands/endpoints or remove them in a follow-up. No database migration is required.
