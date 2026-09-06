## 1. Contracts

- [x] 1.1 Implement capability interfaces, provider registry, range/row validation and typed errors.

## 2. Source adapters

- [x] 2.1 Implement bounded THS transport, industry/concept catalogs and quotation-code resolution.
- [x] 2.2 Implement THS annual index daily history with strict parsing and range handling.
- [x] 2.3 Adapt existing TDX security/index clients to common bars with bounded pagination.

## 3. Access and documentation

- [x] 3.1 Add provider HTTP routes and HTTP client methods; preserve legacy route isolation.
- [x] 3.2 Add thin CLI commands and startup cookie configuration.
- [x] 3.3 Document architecture, supported capabilities, units, limits and Eastmoney extension steps.

## 4. Verification

- [x] 4.1 Add meaningful contract, fixture, pagination, HTTP/CLI isolation and cancellation tests.
- [x] 4.2 Run Go tests/build, OpenSpec validation and opt-in live THS/TDX probes; record coverage and limitations.

Verification (2026-09-06): `go test ./...`, `go build ./cmd/infinity ./cmd/marketd`, `go vet ./...`, race tests for marketdata/ths/tdx/querier/infinitycli, and `openspec validate --all` passed (26 items). Opt-in THS and TDX live probes passed; observed catalogs, instruments and date coverage are recorded in `docs/api/providers.md`.

## 5. Complete three-source integration

- [x] 5.1 Extend proposal/design/specs with concrete Eastmoney protocols, capability matrix and acceptance criteria.
- [x] 5.2 Implement Eastmoney bounded transport, complete board pagination and category-checked resolution.
- [x] 5.3 Implement Eastmoney daily history with response identity, field-order and chunk validation.
- [x] 5.4 Register all three sources and preserve registry entries when configuring THS; update CLI help and usage docs.
- [x] 5.5 Add fixture, pagination, three-source HTTP/CLI, failure and cancellation tests; run regression/build/vet/race/OpenSpec checks and record live probe outcomes separately.
- [x] 5.6 Pass Eastmoney live catalog and daily-history acceptance when upstream connectivity is available; fixture success alone does not complete live acceptance.

Three-source verification (2026-09-06): all Go tests, infinity/marketd builds, `go vet ./...`, race tests for all six affected source/API packages and OpenSpec validation passed. The three real adapters passed common HTTP/CLI tests using simulated upstream transport. Initial Eastmoney live attempts failed when its public API closed connections. On the subsequent apply run, `MARKETD_EASTMONEY_PROBE=1 go test ./internal/eastmoney -run TestLiveEastmoney -count=1 -v` passed using the default Go client and endpoints: 496 industry boards and 504 concept boards, with BK1027 and BK0715 each returning two daily bars spanning 2026-09-03 through 2026-09-04. Independent catalog requests still sometimes disconnected; this live acceptance proves the observed requests succeeded, not continuous availability or exhaustive historical coverage. Details and repeatable probe commands are in `docs/api/providers.md`.
