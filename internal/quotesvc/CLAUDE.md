# internal/quotesvc/

Long-running realtime quote service orchestration for `marketd quote-serve` / `quote-status`. Operational policy only — protocol construction/decoding stays in `internal/tdx`, and this package never opens ClickHouse directly.

- `pool.go` / `pools.go` — bounded per-server connection pools (reuse, heartbeat-before-reuse, idle expiry, max-age reconnect) and a round-robin registry with health tracking.
- `limiter.go` — token-bucket rate limiter (global + per-server) with a pure, testable `reserve` core.
- `sweep.go` — symbol discovery/explicit planning and stable batch partitioning.
- `executor.go` — bounded-concurrency batch execution, failure classification (`errors.go`), retry+backoff, decode-failure preservation, failure isolation, resume from durable state, and progress recording.
- `service.go` — wires config → pools/executor/discoverer; exposes `Health`.

Durable state goes through the `StateStore` interface (satisfied by `*clickhouse.Store`), which exposes **only** ops-plane run/batch writers — realtime snapshots never reach fact tables. Inject `now`/`sleep`/`Dialer` for deterministic tests (no real network/clock).
