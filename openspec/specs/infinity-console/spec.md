# infinity-console Specification

## Purpose
TBD - created by archiving change infinity-console. Update Purpose after archive.
## Requirements
### Requirement: Console frontend development workflow
The system SHALL provide a Node.js + Vite development workflow for the Infinity console frontend.

#### Scenario: Start Vite dev server
- **WHEN** a developer runs the documented console development command from the console frontend directory
- **THEN** Vite serves the console application for local development
- **AND** frontend API requests to `/api/*` are proxied to the Go querier server

#### Scenario: Build production assets
- **WHEN** a developer runs the documented console build command
- **THEN** Vite produces static production assets
- **AND** the build does not require ClickHouse, TDX network access, or a running Go server

### Requirement: Console static serving
The system SHALL provide a standalone `infinity-console` binary that serves built console assets and console API routes from a configured Vite output directory.

#### Scenario: Start standalone console
- **WHEN** an operator starts `infinity-console` with a console dist path
- **THEN** the HTTP server serves the console at `/console/`
- **AND** direct navigation to `/` redirects to `/console/`
- **AND** direct navigation to console routes returns the console `index.html`
- **AND** existing `/api/v1/*`, `/api/tdx/*`, and `/api/console/*` routes behave as API routes

#### Scenario: Serve configured console
- **WHEN** an operator starts `infinity querier serve` with a console dist path
- **THEN** the HTTP server serves the console at `/console/`
- **AND** direct navigation to console routes returns the console `index.html`
- **AND** existing `/api/v1/*` and `/api/tdx/*` routes continue to behave as API routes

#### Scenario: API-only default remains available
- **WHEN** an operator starts `infinity querier serve` without a console dist path
- **THEN** the server continues to expose the existing API routes
- **AND** it does not require Node.js or built console assets at runtime

### Requirement: Console operational summary
The system SHALL expose console API data for operator-visible system health and recent ops-plane state.

#### Scenario: View summary
- **WHEN** the console requests the operational summary
- **THEN** the response includes querier health, schema version, recent watermarks, recent task runs, recent data quality issue counts, and recent quote service run status
- **AND** the response does not include ClickHouse credentials

#### Scenario: View watermarks
- **WHEN** the console requests watermarks
- **THEN** the response lists recent watermark dataset, asset, status, updated time, and message values
- **AND** results are ordered by most recent update first

#### Scenario: View task runs
- **WHEN** the console requests task runs
- **THEN** the response lists recent task run identifiers, dataset, task type, status, target table, timing, row counts, and error text when present
- **AND** the request supports a bounded limit

#### Scenario: View data quality issues
- **WHEN** the console requests data quality issues
- **THEN** the response lists recent issue dataset, severity, issue type, market, symbol, logical key, observed time, message, and details
- **AND** the request supports a bounded limit

### Requirement: Console realtime quote visibility
The system SHALL expose console API data for realtime quote service and TDX HQ provider status.

#### Scenario: View quote service runs
- **WHEN** the console requests quote service runs
- **THEN** the response lists recent run status, planned batches, succeeded batches, failed batches, skipped batches, rows fetched, timing, and error fields

#### Scenario: Run HQ probe smoke check
- **WHEN** the console requests a TDX HQ probe smoke check with optional server candidates
- **THEN** the system probes the candidates through the existing TDX provider path
- **AND** the response includes each server, success status, latency, error text, and preferred marker

#### Scenario: Run quote smoke check
- **WHEN** the console requests a quote smoke check for one or more symbols
- **THEN** the system fetches quotes through the existing TDX provider path
- **AND** the response includes decoded quote fields without writing market fact tables

### Requirement: Console bestip status and refresh
The system SHALL expose HQ bestip cache status and explicit refresh through the console API.

#### Scenario: View bestip cache
- **WHEN** the console requests bestip status
- **THEN** the response includes cache path, generated time, expiration time, preferred server, probe results, and whether the cache is currently usable

#### Scenario: Refresh bestip cache
- **WHEN** the console requests a bestip refresh
- **THEN** the system probes candidate TDX HQ servers
- **AND** it writes the refreshed cache using the existing bestip cache workflow
- **AND** it returns the refreshed preferred server and probe results

### Requirement: Console safety boundary
The system SHALL keep first-version console operations non-destructive.

#### Scenario: No destructive ClickHouse operations
- **WHEN** an operator uses the console
- **THEN** the console does not expose actions that drop, truncate, detach, replace, or delete ClickHouse tables or rows

#### Scenario: No fact writes from smoke checks
- **WHEN** the console runs TDX provider probe or quote smoke checks
- **THEN** the checks do not write to canonical market fact tables
- **AND** the checks do not create task runs or advance watermarks

