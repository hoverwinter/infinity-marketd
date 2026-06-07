# marketd-realtime-quote-service Specification

## Purpose
TBD - created by archiving change harden-realtime-quote-service. Update Purpose after archive.
## Requirements
### Requirement: Long-running quote service mode
The system SHALL provide a long-running realtime quote service mode for TDX standard行情 quote collection.

#### Scenario: Start service with configured servers
- **WHEN** the operator starts the realtime quote service with configured TDX HQ server candidates
- **THEN** the service initializes managed quote clients for the configured servers and reports service status as running

#### Scenario: Stop service gracefully
- **WHEN** the operator stops the realtime quote service
- **THEN** the service stops accepting new sweep work, finishes or cancels active batches according to the shutdown deadline, closes managed connections, and records the final run status

### Requirement: Managed TDX connection lifecycle
The system SHALL manage reusable TDX HQ connections with bounded pools, heartbeat checks, idle expiry, and periodic reconnects.

#### Scenario: Reuse healthy connection
- **WHEN** a quote batch is scheduled for a server with an available healthy connection
- **THEN** the service reuses that connection without repeating the setup handshake

#### Scenario: Replace failed heartbeat connection
- **WHEN** a pooled connection fails its heartbeat check before reuse
- **THEN** the service closes the failed connection, opens a replacement connection within pool limits, and records the heartbeat failure

#### Scenario: Periodic reconnect
- **WHEN** a pooled connection exceeds the configured maximum connection age
- **THEN** the service retires the connection and creates a fresh connection before assigning more quote work to it

### Requirement: Bounded full-market sweep execution
The system SHALL execute full-market realtime quote sweeps using explicit symbol discovery or symbol input, bounded batch sizes, and bounded request concurrency.

#### Scenario: Sweep discovered symbols
- **WHEN** the operator starts a full-market sweep for supported markets
- **THEN** the service discovers or loads the target symbols, partitions them into configured batch sizes, and creates a sweep run with planned batch counts

#### Scenario: Limit concurrent batches
- **WHEN** a sweep has more pending batches than the configured concurrency limit
- **THEN** the service runs no more than the configured number of quote batches at the same time

### Requirement: Quote request rate limiting
The system SHALL apply configurable global and per-server rate limits to realtime quote requests.

#### Scenario: Enforce global rate limit
- **WHEN** pending quote batches would exceed the configured global request rate
- **THEN** the service delays batch execution until rate budget is available

#### Scenario: Enforce per-server rate limit
- **WHEN** a single TDX HQ server reaches its configured request rate
- **THEN** the service delays or routes eligible work to another healthy server without exceeding the server limit

### Requirement: Batch failure recovery
The system SHALL isolate quote batch failures and retry recoverable failures according to configured retry and backoff policy.

#### Scenario: Retry recoverable transport failure
- **WHEN** a quote batch fails because of a timeout, connection failure, or server selection failure
- **THEN** the service retries the batch within the configured retry budget using backoff and an eligible healthy server

#### Scenario: Preserve decode failure
- **WHEN** a quote batch fails because the response cannot be decoded as the expected protocol payload
- **THEN** the service records the decode failure with batch context and does not hide it as a transport retry success

#### Scenario: Continue after failed batch
- **WHEN** a batch exhausts its retry budget
- **THEN** the service marks that batch failed and continues processing other pending batches in the same sweep

### Requirement: Durable run and batch progress
The system SHALL record realtime quote service runs and batch progress in the ops plane.

#### Scenario: Record run start
- **WHEN** a realtime quote sweep starts
- **THEN** the system records a run identifier, run parameters, target markets, planned symbol count, planned batch count, status, and start time

#### Scenario: Record batch transitions
- **WHEN** a batch moves through pending, running, succeeded, failed, or skipped state
- **THEN** the system records the batch number, symbol range or symbol count, attempt count, status, timing, row count, and error summary

#### Scenario: Summarize run completion
- **WHEN** all batches in a sweep are succeeded, failed, or skipped
- **THEN** the system records final run counters, finish time, duration, and final status

### Requirement: Resumable sweep execution
The system SHALL support resuming an interrupted realtime quote sweep from durable batch state.

#### Scenario: Resume failed run
- **WHEN** the operator resumes a sweep run that has failed or interrupted batches
- **THEN** the service schedules only batches that are not recorded as succeeded unless the operator explicitly requests a full rerun

#### Scenario: Reject incompatible resume
- **WHEN** the resume request changes the original run markets, symbol source, or batch sizing in a way that would make recorded batch state ambiguous
- **THEN** the service rejects the resume request with a clear incompatibility error

### Requirement: Service health and run status reporting
The system SHALL expose realtime quote service health, connection state, limiter state, and sweep run status to operators.

#### Scenario: Report current health
- **WHEN** the operator requests service health
- **THEN** the system reports service state, configured servers, healthy server count, pooled connection counts, heartbeat failures, and last successful quote time

#### Scenario: Report active run status
- **WHEN** the operator requests status for an active sweep
- **THEN** the system reports run status, planned batches, completed batches, failed batches, retry counts, request rate, and current batch progress

### Requirement: No implicit quote snapshot persistence
The system SHALL NOT write realtime quote snapshots to canonical ClickHouse market fact tables as part of the long-running quote service.

#### Scenario: Sweep without snapshot storage
- **WHEN** the realtime quote service completes a sweep without an accepted snapshot storage capability
- **THEN** the system records ops-plane run and batch status only and does not write quote snapshots to canonical fact tables

