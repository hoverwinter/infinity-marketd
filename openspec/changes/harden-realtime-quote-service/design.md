## Context

`marketd` has an on-demand realtime quote path built around TDX standard行情 requests. Recent quote enhancements add server probing, retries, batch reuse, symbol discovery, and quote sweeps, but those workflows still behave like bounded commands: start, fetch, exit.

A long-running quote service has different failure modes. Public TDX HQ servers can throttle, close idle sockets, time out, or return protocol errors after a connection has appeared healthy. Full-market realtime sweeps also need operator-visible progress because one failed batch must not make the whole run opaque.

This change focuses on operationalizing realtime quote collection. It does not change canonical market fact tables and does not introduce snapshot persistence as market data storage. Any persisted records in this change are ops records: service runs, batch progress, health state, and failure summaries.

## Goals / Non-Goals

**Goals:**
- Run a long-lived realtime quote service with managed TDX HQ server connections.
- Pool and reuse connections by server while bounding open sockets and request concurrency.
- Detect unhealthy or stale connections through heartbeat checks, idle expiry, and periodic reconnects.
- Execute full-market sweeps with configurable batch size, request rate, retry budget, backoff, and failure isolation.
- Persist run and batch state in `infinity_ops` so operators can inspect active, completed, failed, and resumable sweeps.
- Expose service health and run status without requiring ad-hoc ClickHouse reads from CLI code.
- Keep TDX protocol parsing isolated in `internal/tdx`.

**Non-Goals:**
- No trading or order-entry support.
- No realtime streaming subscription protocol.
- No implicit ClickHouse persistence of quote snapshots.
- No destructive schema migration.
- No replacement of the existing on-demand quote CLI.
- No dependency on Python realtime quote libraries.

## Decisions

- Introduce a service-level orchestration package separate from `internal/tdx`.
  - Rationale: `internal/tdx` should remain protocol construction and decoding. Long-running lifecycle, scheduling, rate limits, retries, and status storage are service concerns.
  - Alternative considered: put connection pooling directly in `internal/tdx`. That would mix protocol code with operational policy and make one-shot tests harder to keep simple.

- Manage a bounded connection pool per selected TDX HQ server.
  - Rationale: full-market sweeps need reuse, but public servers should not be hit with unbounded sockets. A per-server pool allows max connections, max in-flight requests, idle TTL, and reconnect policy to be configured together.
  - Alternative considered: one global pool across all servers. That makes fallback and per-server health harder to reason about when some servers are degraded.

- Use heartbeat probes and periodic reconnects rather than trusting idle TCP sockets.
  - Rationale: TDX HQ servers can close idle sockets or become half-open. A lightweight heartbeat before reuse and a max connection age reduce failures during large sweeps.
  - Alternative considered: only reconnect after request failure. That keeps implementation smaller but causes avoidable batch failures and noisy retries.

- Keep retries at the batch orchestration layer.
  - Rationale: the service can classify connection errors, server timeouts, limiter delays, and decode errors with run/batch context. Low-level protocol functions should still surface parser regressions clearly.
  - Alternative considered: retry inside packet send/decode helpers. That hides the failing server and makes progress accounting inaccurate.

- Use explicit token-bucket style rate limiting with no new dependency unless implementation shows the standard-library version is insufficient.
  - Rationale: rate control is simple enough for this service: global requests per second, optional per-server requests per second, and a burst cap. Avoiding a dependency keeps the daemon small.
  - Alternative considered: add `golang.org/x/time/rate`. It is proven and acceptable if the in-house limiter becomes awkward, but it is not required for the proposal.

- Persist sweep state as ops-plane records, not quote facts.
  - Rationale: operators need visibility into jobs and batches even when quote snapshots are not stored. Ops tables can use logical keys such as `run_id` and `(run_id, batch_no)` with `ReplacingMergeTree(updated_at)`.
  - Alternative considered: reuse `task_runs` only. That records coarse import-like task history, but it cannot answer which symbols/batches are complete, failed, retried, or resumable.

- Expose service status through the running service and future querier repository paths, not direct CLI ClickHouse reads.
  - Rationale: repository rules keep ClickHouse read SQL centralized. A `marketd` status command can query the service admin endpoint; persisted ops reads can be added to `internal/clickhouse/query.go` and the querier API when needed.
  - Alternative considered: let CLI commands query `infinity_ops` directly. That violates the read-plane boundary and spreads SQL.

- Resume sweeps by run state rather than by inferring from quote output.
  - Rationale: quote snapshots may not be persisted. Resume must be based on durable batch states and the original sweep parameters.
  - Alternative considered: compare against stored quote rows. That only works after a separate snapshot storage contract exists.

## Risks / Trade-offs

- Public TDX servers may throttle or ban aggressive clients -> enforce conservative defaults, per-server limits, bounded concurrency, and jittered backoff.
- Heartbeats add request volume -> make intervals configurable and skip heartbeat immediately after successful use.
- Batch state can grow for full-market sweeps -> partition ops tables by run start month and keep batch payloads compact.
- Resume can repeat a successfully fetched quote if failure happens after fetch but before status update -> record batch transitions before and after execution; this is acceptable because quote snapshots are not canonical facts in this change.
- Protocol decode errors may be transient for unusual instruments -> classify decode failures separately from transport failures and include sample identity in batch failure summaries.
- Service-local status can diverge briefly from persisted ops state -> treat in-memory status as current runtime state and ops records as durable audit/resume state.

## Migration Plan

1. Add new `CREATE TABLE IF NOT EXISTS` DDL for realtime ops tables during bootstrap. Do not drop or rewrite existing tables.
2. Add config fields with safe defaults so existing `marketd quote` and import commands continue to run unchanged.
3. Implement the long-running service behind a new command or subcommand; keep current one-shot quote commands available.
4. Deploy with low concurrency and a limited explicit symbol list before enabling full-market discovery sweeps.
5. Roll back by stopping the service command. Existing ops records can remain in `infinity_ops` and do not affect historical imports or the querier.

## Open Questions

- Should service status be exposed only as a local HTTP admin endpoint, or also through the existing `infinity` querier API?
- What default request rate is acceptable for commonly reachable public TDX HQ servers?
- Should resume continue the same `run_id` or create a child run linked to the failed run for clearer audit history?
- What retention policy should operators use for realtime service run and batch records?
