## Context

`marketd` has three distinct data access surfaces today:

- ClickHouse-backed canonical reads exposed by `infinity querier serve` under `/api/v1`.
- TDX standard行情 (`hq`) online reads implemented in `internal/tdx` and exposed mostly through `marketd quote`, `quote-probe`, `quote-sweep`, and `hq-*` CLI commands.
- TDX extended行情 (`exhq`) online reads implemented in `internal/tdx` and exposed through `marketd exquote-*` CLI commands.

The current HTTP server only exposes the ClickHouse-backed query surface. Adding realtime and TDX protocol reads to the same `/api/v1` resource space would blur two different contracts: stable persisted facts versus live provider calls that can time out, throttle, or vary by upstream server.

## Goals / Non-Goals

**Goals:**

- Add an HTTP façade for existing TDX online read capabilities.
- Use `/api/tdx/...` for provider/protocol APIs.
- Keep `/api/v1/...` reserved for product/query APIs backed by ClickHouse or stable internal service state.
- Reuse existing `internal/tdx` protocol implementations instead of rewriting packet logic.
- Document API namespace boundaries for future control-console work.

**Non-Goals:**

- No WebSocket or SSE streaming.
- No quote snapshot persistence.
- No ClickHouse schema change.
- No migration of `/api/v1/bars` to another path.
- No merge of standard HQ and ExHQ protocol naming.
- No replacement or removal of existing `marketd` CLI commands.

## Decisions

### Decision: Use `/api/tdx/...` for provider APIs

TDX online reads will live under:

```text
/api/tdx/hq/...
/api/tdx/exhq/...
```

Rationale:

- The paths describe the true upstream provider and protocol class.
- They avoid implying the data is canonical, persisted, or replayable like `/api/v1/bars`.
- They leave room for future provider namespaces such as `/api/futu/...` or `/api/crypto/...`.

Alternative considered: `/api/v1/realtime/...`.

- Rejected because realtime is a transport/freshness property, not the actual provider boundary. TDX also exposes online K-line, transaction, F10, finance, and block reads that are not all quote snapshots.

Alternative considered: `/api/v1/tdx/...`.

- Rejected for the first provider API because `/api/v1` is currently the product/query API namespace. Promoting raw provider APIs into that namespace would make later compatibility promises harder.

### Decision: Preserve HQ and ExHQ as separate route groups

Standard A-share HQ and extended ExHQ routes remain separate:

```text
/api/tdx/hq/quotes
/api/tdx/hq/probe
/api/tdx/hq/securities
/api/tdx/hq/bars
/api/tdx/hq/minute
/api/tdx/hq/transactions

/api/tdx/exhq/markets
/api/tdx/exhq/instruments
/api/tdx/exhq/quote
/api/tdx/exhq/bars
/api/tdx/exhq/minute
/api/tdx/exhq/transactions
```

Rationale:

- HQ uses `sh` / `sz` / `bj` market names and A-share-style symbols.
- ExHQ uses numeric market ids and instrument codes.
- The protocol packets, server pools, and validation rules are different.

Alternative considered: one generic `/api/tdx/quotes` endpoint.

- Rejected because it would hide protocol differences and force ambiguous parameter models.

### Decision: Provider APIs return upstream data but do not persist it

TDX HTTP reads call upstream TDX servers and return decoded responses. They do not write snapshots or online reads to ClickHouse.

Rationale:

- Snapshot storage has volume, retention, deduplication, and lifecycle questions.
- Existing OpenSpec changes explicitly defer realtime snapshot storage to a separate storage contract.
- Keeping this change read-only reduces operational blast radius.

Alternative considered: write `/api/tdx/hq/quotes` responses into a quote snapshot table opportunistically.

- Rejected because it would mix API read latency with ingestion policy and create schema decisions without a storage proposal.

### Decision: Error status distinguishes validation, upstream, and protocol failures

TDX provider APIs use the same JSON error envelope as existing APIs:

```json
{ "error": "..." }
```

Status mapping:

```text
400 validation error
502 upstream response/protocol decode error
503 upstream server unavailable or timed out
500 unexpected service error
```

Rationale:

- Clients can distinguish bad requests from provider outages.
- Decode errors are not hidden as generic outages, which helps protocol diagnostics.

### Decision: Start with HTTP request/response

The first provider API is synchronous HTTP. No WebSocket/SSE is included.

Rationale:

- Public TDX standard行情 behaves like request/response polling, not native push.
- Existing CLI and `internal/tdx` code are request/response.
- Control-console users can start with probes and on-demand snapshots before investing in streaming lifecycle management.

## Risks / Trade-offs

- TDX public servers can be slow or unreachable -> expose probe endpoints, allow server override parameters, and map failures to `503`.
- Provider APIs may be mistaken for canonical market data -> keep them under `/api/tdx` and document that `/api/v1` remains the stable query API.
- Large quote sweeps can overload upstream servers -> enforce existing batch size behavior and add request limits in HTTP handlers.
- Exposing many protocol endpoints at once can create inconsistent parameter naming -> implement route groups incrementally and keep request structs close to existing CLI flags.
- Decode errors may surface to clients -> return `502` so operators can distinguish protocol drift from network outages.

## Migration Plan

1. Keep existing `/api/v1` endpoints unchanged.
2. Add `/api/tdx/hq/*` and `/api/tdx/exhq/*` routes behind the existing `infinity querier serve` process.
3. Update `docs/api/README.md` to describe the namespace split and provider API reference.
4. Update `docs/design/tdx-realtime-quotes.md` to record the HTTP boundary and non-persistence decision.
5. Add tests for request validation, route mapping, error mapping, and no ClickHouse writes.

Rollback:

- Remove or disable `/api/tdx/*` route registration. Existing `/api/v1` APIs and `marketd` CLI commands remain unaffected.

## Open Questions

- Should `/api/tdx/hq/quotes` accept only repeated `symbol` query parameters, or also comma-separated `symbols` for easier browser use?
- Which request-level limits should be enforced for quote sweeps over HTTP?
- Should server override parameters be accepted on all TDX provider routes, or only probe/debug routes?
- Should provider APIs be enabled by default or guarded by a config flag once long-running production use begins?
