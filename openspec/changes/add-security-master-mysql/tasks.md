## 1. Configuration and MySQL Storage

- [x] 1.1 Add MySQL configuration fields, environment overrides, and validation for securities-master users without changing ClickHouse defaults.
- [x] 1.2 Add a MySQL connection package or store wrapper for securities-master reads and writes.
- [x] 1.3 Add idempotent `CREATE TABLE IF NOT EXISTS` DDL for `securities`, `security_name_history`, `security_aliases`, and `security_refresh_runs`.
- [x] 1.4 Wire MySQL schema bootstrap into the existing bootstrap command path without destructive migrations.
- [x] 1.5 Add config and bootstrap tests for missing MySQL config, loaded MySQL config, and repeatable DDL generation.

## 2. Securities Master Domain and Repository

- [x] 2.1 Add model types for current securities, name-history segments, aliases, refresh runs, and resolve candidates.
- [x] 2.2 Implement repository methods for exact lookup, candidate resolve, current-row upsert, alias upsert, name-history upsert, and refresh-run audit updates.
- [x] 2.3 Implement normalization for market, symbol, names, aliases, exchange, board, status, listing dates, lot size, and price precision.
- [x] 2.4 Enforce manual protection rules for `securities.manual_locked` and `security_name_history.manual_override`.
- [x] 2.5 Add repository and normalization tests using a MySQL test fixture or isolated integration setup.

## 3. TDX Beijing Security Discovery

- [x] 3.1 Verify the TDX standard HQ Beijing security count/list packet path and record the tested market mapping and sample source in documentation or fixtures.
- [x] 3.2 Update TDX security count/list validation so verified `bj` discovery is accepted while unverified markets are still rejected.
- [x] 3.3 Update `quote-sweep --market bj` to discover Beijing securities through the verified security-list path.
- [x] 3.4 Update `/api/tdx/hq/securities?market=bj` to return Beijing security-list entries through the TDX provider path.
- [x] 3.5 Replace existing rejection tests with Beijing discovery success tests and keep mismatch/source-failure tests.

## 4. Refresh Commands and Source Selection

- [x] 4.1 Add a securities-master refresh command with `--source`, repeatable or comma-separated `--market`, `--dry-run`, and source-specific options.
- [x] 4.2 Implement the native `tdx` source adapter for `sh`, `sz`, and verified `bj` security-list rows.
- [x] 4.3 Implement a normalized file source adapter for offline rows produced by AkShare, mootdx, manual exports, or other upstream sources.
- [x] 4.4 Normalize refresh rows into current securities, aliases, and name-history segments before repository writes.
- [x] 4.5 Record refresh-run status, row counts, skipped rows, and errors for non-dry-run executions.
- [x] 4.6 Add command tests for source selection, market selection, dry run, failed selected source without fallback, and manual-lock preservation.

## 5. Query APIs

- [x] 5.1 Add read-plane repository interfaces for securities exact lookup and resolve without adding MySQL reads to ClickHouse bar query methods.
- [x] 5.2 Add `GET /api/v1/securities?market=...&symbol=...` with validation, 404 behavior, and JSON response shape.
- [x] 5.3 Add `GET /api/v1/securities/resolve?q=...` with ranked candidates from symbol, current name, historical name, and aliases.
- [x] 5.4 Add tests proving ambiguous resolve returns multiple candidates rather than choosing one.
- [x] 5.5 Add regression tests proving `/api/v1/bars` remains ClickHouse-only and does not include joined security names.

## 6. Documentation and Validation

- [x] 6.1 Update `configs/config.yaml` and config documentation with MySQL settings and no hard-coded DSN.
- [x] 6.2 Update `docs/storage/clickhouse.md` or companion storage docs to document that securities master lives in MySQL while facts stay in ClickHouse.
- [x] 6.3 Update `docs/api/README.md` with the exact lookup and resolve APIs and the bars boundary.
- [x] 6.4 Update TDX docs to describe Beijing security-list discovery support and remaining server limitations.
- [x] 6.5 Run `gofmt` on changed Go files.
- [x] 6.6 Run `go test ./...`.
- [x] 6.7 Run `openspec validate --all`.
