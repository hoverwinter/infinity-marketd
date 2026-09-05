## 1. Storage Model

- [x] 1.1 Add normalized Go model types for limit events, summaries, relay rows, performance indices, breadth, and themes
- [x] 1.2 Add all six idempotent ClickHouse table definitions and schema tests
- [x] 1.3 Add batched Store insert methods and insert SQL tests for all six datasets

## 2. Migration And Corrections

- [x] 2.1 Implement the quman snapshot parser, normalization, date filtering, deduplication, and parser fixtures
- [x] 2.2 Implement the snapshot import orchestration with dry-run, ops metadata, watermarks, and count consistency issues
- [x] 2.3 Implement correction JSONL parsing and upsert-only import behavior with tests
- [x] 2.4 Implement normalized JSON import for performance-index and market-breadth rows with validation tests
- [x] 2.5 Add marketd CLI routes, flags, output, and command tests for the new imports

## 3. Query API

- [x] 3.1 Add review query DTOs, repository methods, normalization, and validation tests
- [x] 3.2 Implement all review ClickHouse queries in `internal/clickhouse/query.go` with SQL tests
- [x] 3.3 Add focused `/api/v1` routes and handlers with HTTP tests
- [x] 3.4 Add the one-day `/api/v1/limit-review` reconstruction response and tests

## 4. Documentation And Verification

- [x] 4.1 Update authoritative storage/API docs and reconcile the detailed design with repository invariants
- [x] 4.2 Run formatting, focused tests, full Go tests, build, and `openspec validate --all`

## 5. Online Collection And Completion

- [x] 5.1 Validate all six tables and correction-sensitive queries against isolated real ClickHouse databases
- [x] 5.2 Add THS online pool refresh with pagination/date validation and strict consecutive-board normalization
- [x] 5.3 Verify the three supported dedicated TDX index identities and add bounded online index history import; reject unverified non-ST substitution
- [ ] 5.4 Verify 880005 breadth semantics and enable only supported fields with captured evidence (awaiting independently verified same-day totals and >7% bucket values; official ADVANCE/DECLINE documentation rules out the naive total-count mapping)
- [x] 5.5 Make range relay/theme queries reflect final event corrections and add a bounded matrix API
- [x] 5.6 Update documentation and run complete tests, builds, live dry-runs, and schema validation

## 6. Workbench Write Integration

- [x] 6.1 Reuse correction validation for in-memory payloads and expose an opt-in authenticated console operation without changing the read API
- [x] 6.2 Cover validation, authentication, bounded requests, serialized execution, dry-run and real ClickHouse write/read correction flow
- [x] 6.3 Document gateway integration, failure/retry behavior and remaining provider/UI gaps

## 7. Production Snapshot Migration

- [x] 7.1 Freeze and hash inputs, select one snapshot per date, bootstrap missing tables and verify empty destination ranges through HTTP
- [x] 7.2 Normalize verified legacy placeholder zeros via explicit snapshot profiles, preserve input warnings, and test the conversion
- [x] 7.3 Import the selected 2016+ snapshots into production serially with preserved batch results
- [x] 7.4 Audit all normalized event fields and daily coverage through HTTP and record actual counts and remaining data gaps

## 8. Blogger Evidence Pilot

- [x] 8.1 Inspect upstream evidence and build bounded candidate plans in Infinity without adding market tables or OCR dependencies to marketd
- [x] 8.2 Publish visually checked complete-row enrichments through the existing Go HTTP operation and verify all fields
- [x] 8.3 Document production pilot results, source limitations and run integration regressions

## 9. Authority-First Historical Integration

- [x] 9.1 Enforce existing-event enrichment in Go; isolate explicit fact replacement and protect enrichment during provider refresh
- [x] 9.2 Process every available 2016+ evidence workspace with resumable per-date plans and explicit rejection reports
- [x] 9.3 Publish verified enrichments with complete readback and expose a usable date-by-date review inventory
- [x] 9.4 Verify regressions and document actual coverage and remaining unavailable or unverified facts without claiming completeness
- [ ] 9.5 Finish historical fact completeness: recover missing materials and independently verify pending attribution, missing events, historical names and board differences (full corpus processing alone does not satisfy this)
