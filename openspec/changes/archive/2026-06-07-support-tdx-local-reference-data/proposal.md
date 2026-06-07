## Why

`marketd` can import core TDX A-share OHLCV files, but its `client-local` reader coverage is still incomplete. Operators currently have to rely on the TDX desktop client or Python readers for client-maintained `gbbq`, local block files, custom blocks, and local extension-market daily bars even though these data are needed for复权, turnover, market-cap history, block membership scans, and extension-market research.

## What Changes

- Add `client-local` `gbbq` parsing and import for A-share capital-change and corporate-action events.
- Add `client-local` TDX system block file parsing and import for vendor/system block definitions and memberships.
- Add `client-local` custom block parsing and import for user-defined block definitions and memberships.
- Add guarded custom block write support that updates client-local TDX user files only through explicit operator commands, backup files, atomic replacement, and validation.
- Add `client-local` extension-market daily bar (`ex_daily`) parsing and import for non-A-share TDX extension markets.
- Add ClickHouse schemas for:
  - A-share capital-change events;
  - TDX block definitions;
  - TDX block memberships;
  - extension-market daily OHLCV bars.
- Reuse existing local import behavior: no remote TDX dependency, `--dry-run`, task runs, watermarks, quality issues, and conflict resolution before inserting facts.
- Treat `client-local`, `offline-package`, and `online-provider` as separate source classes; this change covers only `client-local`.
- Keep online `hq-xdxr`, `hq-block`, and `exquote-bars` as provider reads; they may be used for validation, but they do not replace local file import or write to these tables in this change.
- Keep offline package import (`hsjday.zip`, `tdxfin.zip`, `tdxgp.zip`, and similar downloaded packages) out of scope unless a command explicitly reads client-local files extracted into a TDX client directory. Professional financial package import remains covered by `support-tdx-financial-data-import`.

## Capabilities

### New Capabilities

- `marketd-tdx-local-reference-data`: Client-local TDX `gbbq`, system block/custom block, custom block write, and extension-market daily file support.

### Modified Capabilities

- `marketd-clickhouse-data-plane`: Bootstrap creates the reference, membership, and extension-market tables required by local reference-data imports.
- `marketd-tdx-local-import`: Client-local import behavior expands beyond A-share OHLCV files while preserving no-remote-dependency, dry-run, and quality handling contracts.

## Impact

- New parser code and fixtures under `internal/tdx`.
- New import orchestration under `internal/ingest`.
- New ClickHouse DDL and store insert methods.
- New `marketd` CLI commands for dry-run/import and guarded custom block write.
- Updates to `docs/tdx-data/`, `docs/storage/clickhouse.md`, and command help.
- No changes to existing A-share OHLCV fact table schemas.
- No destructive ClickHouse migration; bootstrap uses additive `CREATE TABLE IF NOT EXISTS`.
