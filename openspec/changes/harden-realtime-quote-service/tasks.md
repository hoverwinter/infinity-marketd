## 1. Configuration and Ops Schema

- [ ] 1.1 Add realtime quote service configuration for server pools, heartbeat interval, idle timeout, max connection age, concurrency, rate limits, retry budget, backoff, and batch size.
- [ ] 1.2 Add `CREATE TABLE IF NOT EXISTS` bootstrap DDL for realtime quote run records in `infinity_ops`.
- [ ] 1.3 Add `CREATE TABLE IF NOT EXISTS` bootstrap DDL for realtime quote batch progress records in `infinity_ops`.
- [ ] 1.4 Add model types and write helpers for realtime run and batch status records.
- [ ] 1.5 Update storage documentation with the new ops-plane tables and non-destructive migration guidance.

## 2. Connection Lifecycle

- [ ] 2.1 Introduce a service-level package for realtime quote orchestration outside `internal/tdx`.
- [ ] 2.2 Implement a bounded per-server TDX HQ connection pool with reusable setup connections.
- [ ] 2.3 Add heartbeat checks before stale connection reuse and record heartbeat failures.
- [ ] 2.4 Add idle connection expiry and max-age periodic reconnect behavior.
- [ ] 2.5 Add tests for connection reuse, pool limits, heartbeat replacement, idle expiry, and max-age reconnect.

## 3. Sweep Scheduling and Rate Control

- [ ] 3.1 Implement full-market sweep planning from discovered symbols or explicit symbol input.
- [ ] 3.2 Partition sweep symbols into stable batches with recorded batch numbers and symbol counts.
- [ ] 3.3 Implement bounded batch concurrency for active sweeps.
- [ ] 3.4 Implement configurable global and per-server request rate limiting.
- [ ] 3.5 Add tests for batch planning, concurrency limits, global limiter behavior, and per-server limiter behavior.

## 4. Failure Recovery and Resume

- [ ] 4.1 Classify transport, timeout, server selection, rate-limit, and decode failures in batch execution.
- [ ] 4.2 Retry recoverable batch failures with configured retry budget and backoff.
- [ ] 4.3 Preserve decode/protocol failures as explicit batch errors rather than hiding them behind successful retries.
- [ ] 4.4 Continue a sweep after an individual batch exhausts retries.
- [ ] 4.5 Implement resume from durable batch state, scheduling only non-succeeded batches by default.
- [ ] 4.6 Reject resume requests that change original run parameters in a way that makes batch state ambiguous.
- [ ] 4.7 Add tests for retry, failure isolation, exhausted retries, successful resume, and incompatible resume.

## 5. Service Status and Operator Interface

- [ ] 5.1 Add a long-running `marketd` realtime quote service command or subcommand.
- [ ] 5.2 Add graceful shutdown handling that stops new work, handles active batches according to deadline, closes pools, and records final state.
- [ ] 5.3 Expose service health including server health, connection counts, heartbeat failures, limiter state, and last successful quote time.
- [ ] 5.4 Expose active and historical sweep status without direct CLI ClickHouse reads.
- [ ] 5.5 Add tests for start, graceful shutdown, health output, and run status output.

## 6. Validation

- [ ] 6.1 Verify the long-running service does not write realtime quote snapshots to canonical market fact tables.
- [ ] 6.2 Add integration-style tests using fake TDX servers for heartbeat, reconnect, retry, and sweep progress behavior.
- [ ] 6.3 Update realtime quote design documentation with service operation, defaults, and known limits.
- [ ] 6.4 Run `go test ./...`.
- [ ] 6.5 Run `openspec validate --all`.
