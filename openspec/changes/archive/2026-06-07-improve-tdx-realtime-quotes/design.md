## Context

`marketd quote` currently performs an on-demand TDX standard行情 request for `sh` and `sz` symbols. The implementation opens a fresh TCP connection, sends standard setup packets, fetches a snapshot, decodes known fields, and prints JSON. This is enough for manual smoke tests, but operational use needs reliable server selection, retries, efficient batch behavior, and clear decisions about market coverage and storage.

Public TDX HQ servers are not stable from all networks. Some nodes time out, some respond quickly, and reachability can change. The next phase should treat server selection as a first-class operator workflow before expanding into full-market quote jobs or persisted quote snapshots.

## Goals / Non-Goals

**Goals:**
- Provide a deterministic server probing and best-server selection path for TDX standard行情 servers.
- Retry quote requests across configured candidate servers when a node is unreachable or returns protocol errors.
- Reuse connections for batch quote sweeps where the same server serves many quote requests.
- Support symbol discovery through online security lists before full-market quote sweeps.
- Track and isolate future support for Beijing market and `exhq` extended-market quotes.
- Decide whether realtime snapshots belong in a dedicated ClickHouse table, and define the schema only after that decision.
- Improve timestamp semantics by carrying trade date and `Asia/Shanghai` interpretation where possible.

**Non-Goals:**
- No trading or order-entry support.
- No destructive schema migration.
- No implicit persistence of realtime quote snapshots until a storage decision is accepted.
- No conversion of the current CLI into a long-running service in this change.
- No dependency on pytdx, mootdx, pandas, or Python runtime.

## Decisions

- Treat server probing as the next prerequisite.
  - Rationale: the current main operational failure is server timeout. Reliable server choice improves every quote workflow.
  - Alternative considered: implement full-market quote sweeps first. That would amplify timeout problems and make failures harder to diagnose.

- Keep standard行情 and `exhq` as separate protocol clients.
  - Rationale: `hq` and `exhq` use different markets, ports, packets, and field encodings. Combining them would blur validation and error handling.
  - Alternative considered: a single generic quote client. It would add abstraction before the two protocol surfaces are both implemented.

- Add retries at the server-candidate layer, not inside low-level packet decoding.
  - Rationale: decode errors should remain visible; retry policy should decide when to try another endpoint.
  - Alternative considered: retry every failed packet automatically. That can hide protocol regressions.

- Use connection reuse only for explicit batch workflows.
  - Rationale: one-shot CLI behavior stays simple, while quote sweeps avoid paying setup cost for every batch.
  - Alternative considered: global connection pooling. That requires lifecycle management and is premature before a daemon mode exists.

- Defer ClickHouse snapshot storage until a table contract is proposed.
  - Rationale: quote snapshots are high-frequency market data with retention and deduplication questions. Persisting them needs an explicit schema and operator policy.
  - Alternative considered: write snapshots immediately to an ad hoc table. That risks schema churn and uncontrolled data volume.

## Risks / Trade-offs

- Public TDX servers may behave differently by network or time of day -> keep candidate lists configurable and expose probe results.
- Server probing can be mistaken for health guarantees -> report latency and success at probe time only.
- Batch quote sweeps may hit server throttling -> add bounded batch sizes and retry backoff.
- Beijing market support may require different market mappings or endpoints -> isolate it behind explicit tests and validation.
- Snapshot storage can create large data volume quickly -> require retention, partitioning, and ingestion-rate decisions before implementation.
- `server_time` may not include a full date -> combine with operator-supplied or current `Asia/Shanghai` trade date only when the semantics are explicit.
