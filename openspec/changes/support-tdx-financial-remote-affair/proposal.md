## Why

The local financial import change now parses and imports already-downloaded `tdxfin.zip` and `tdxgp.zip`, but operators still need an external tool such as `mootdx.affair` to discover and fetch remote professional financial files. Closing that gap lets `marketd` own the complete TDX financial package workflow from remote file listing to parse validation and eventual import.

## What Changes

- Add an Affair-like remote financial package workflow for `gpcwYYYYMMDD.zip` files:
  - list available remote financial report packages;
  - fetch one or more package files into an operator-selected local directory;
  - parse a fetched package and print the same financial summary used by local dry-run import.
- Add deterministic validation for remote list parsing and downloads without relying on live TDX servers in tests.
- Document the new CLI workflow and keep it separate from local `import-tdx-fin` / `import-tdx-gp` import semantics.
- No breaking changes.

## Capabilities

### New Capabilities
- `marketd-tdx-financial-remote-affair`: Remote TDX professional financial package listing, fetching, and parse validation modeled after `mootdx.affair` but implemented in `marketd`.

### Modified Capabilities

None.

## Impact

- Affected code:
  - `internal/tdx/finance`: remote package list parsing, download client, and package metadata helpers.
  - `internal/ingest`: parse-only orchestration may reuse the existing financial dry-run path.
  - `internal/cli`: new operator commands for financial package files/fetch/parse.
  - `docs/tdx-data/专业财务数据.md` and reference docs for the end-to-end workflow.
- No ClickHouse schema changes.
- No new third-party dependencies.
- Live downloads are operator-triggered only; parser and import tests remain local and deterministic.
