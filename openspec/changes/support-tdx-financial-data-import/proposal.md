## Why

TDX professional financial packages (`tdxfin.zip`) and stock metric packages (`tdxgp.zip`) are already downloaded and documented, but `marketd` cannot parse or persist them. This leaves fundamental data dependent on external Python tools even though the project now owns local TDX binary ingestion and ClickHouse writes.

## What Changes

- Add local import support for `tdxfin.zip`:
  - parse `gpcwYYYYMMDD.dat` or the matching compressed entries inside the ZIP;
  - normalize rows into a raw financial item table keyed by report period, market, symbol, and field item id.
- Add local import support for `tdxgp.zip`:
  - parse `gp{market}{symbol}.dat` files;
  - normalize rows into a raw stock metric table keyed by market, symbol, metric type, and event date.
- Add version-controlled financial dictionary metadata:
  - `tdxfin` item dictionary sourced from the public mootdx field mapping;
  - `tdxgp` metric dictionary for confirmed metric type semantics.
- Add ClickHouse dictionary tables as query-time synchronized copies of the version-controlled dictionaries.
- Add bootstrap DDL for the raw financial tables and dictionary tables.
- Add import commands:
  - `marketd import-tdx-fin --file data/tdxfin.zip`;
  - `marketd import-tdx-gp --file data/tdxgp.zip`;
  - both support `--dry-run`.
- Preserve existing ops behavior:
  - write task runs, watermarks, and quality issues;
  - validate checksums/manifests when available;
  - fail or degrade when parsed ids are missing from the configured dictionary.
- Keep derived financial wide tables out of scope for this change. They will be designed as explicit refreshable derived data later.
- Keep trading capability out of scope. `pytdx.trade` wraps `trade.dll`, but `marketd` remains a market data daemon and MUST NOT add order, account, position, or broker trading integration.

## Capabilities

### New Capabilities

- `marketd-tdx-financial-import`: Local TDX professional financial and stock metric package parsing, dictionary synchronization, and raw ClickHouse import.

### Modified Capabilities

- `marketd-clickhouse-data-plane`: Bootstrap creates the raw financial tables and dictionary tables required by financial imports.

## Impact

- New TDX financial parser code and fixtures.
- New version-controlled dictionary metadata files.
- New ClickHouse market tables for raw financial items, raw stock metrics, and dictionary lookup.
- New `marketd` import commands and tests.
- New documentation in `docs/tdx-data/` and `docs/storage/`.
- No changes to existing OHLCV fact table contracts.
- No derived financial wide tables, factor tables, query API, or trading capability in this change.
