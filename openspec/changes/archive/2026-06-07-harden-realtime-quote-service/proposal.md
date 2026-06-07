## Why

`marketd quote` is moving from manual, one-shot realtime checks toward operational quote collection. Long-running quote services need explicit connection lifecycle management, health checks, recovery, rate control, batch progress, and observable run state before operators can trust them for full-market realtime sweeps.

## What Changes

- Add a long-running realtime quote service mode with managed TDX HQ connections instead of one TCP connection per request.
- Introduce connection pools, heartbeat checks, stale-connection retirement, and periodic reconnects for quote servers.
- Add bounded full-market quote sweep execution with configurable rate limits, batch sizes, backoff, retries, and failure isolation.
- Record per-run status, batch progress, counters, and failure summaries in the ops plane so interrupted sweeps can be inspected and resumed safely.
- Expose operator-facing status and health reporting for the realtime quote service.
- Keep realtime snapshots out of canonical ClickHouse fact tables in this change unless a separate storage contract is accepted.

## Capabilities

### New Capabilities

- `marketd-realtime-quote-service`: Long-running realtime quote service operations, including connection lifecycle, health checks, bounded full-market sweeps, failure recovery, progress recording, and status reporting.

### Modified Capabilities

## Impact

- New `marketd` command or service entrypoint for long-running realtime quote collection.
- New internal service package coordinating quote clients, server selection, rate limiting, batch scheduling, and recovery.
- Changes to `internal/tdx` to support reusable managed connections while keeping protocol decoding isolated.
- New ops-plane tables or records for realtime quote run state, batch progress, and failure summaries; no destructive ClickHouse commands or fact-table schema changes.
- Configuration additions for server pools, heartbeat intervals, reconnect policy, rate limits, retry policy, and sweep batch sizing.
- Tests for connection lifecycle behavior, limiter/backoff behavior, resumable sweep progress, and status reporting.
