## Why

`marketd` already implements part of the TDX standard行情 (`hq`) read path for A-share realtime quote snapshots and security discovery. Operators now need a complete Go-native, pytdx-compatible standard行情 read surface so A-share online data can be inspected, backfilled, and validated without Python.

## What Changes

- Add direct TDX standard行情 protocol support for the remaining documented `hq` read APIs:
  - security and index K-line data;
  - current-day and historical minute-time data;
  - current-day and historical transaction data;
  - company/F10 category and content data;
  - xdxr corporate-action data;
  - finance info data;
  - block metadata and block membership data.
- Preserve and fold existing standard行情 support into the same contract:
  - realtime quote snapshots;
  - server probing and retry;
  - security count and security list discovery.
- Add operator-facing `marketd` CLI commands for each read API, emitting deterministic JSON.
- Enforce protocol windows such as K-line `count <= 800`, transaction page sizing, and per-day historical minute requests.
- Decode TDX GBK/GB18030 text fields where applicable.
- Document server behavior, pagination, date semantics, and unsupported/empty-response cases.

## Non-Goals

- Do not persist realtime quote snapshots, K-line data, minute-time data, transaction data, F10 data, xdxr data, finance data, or block data to ClickHouse in this change.
- Do not add or modify ClickHouse market fact schemas.
- Do not implement trading, order entry, authenticated Level-2 feeds, or private broker APIs.
- Do not merge standard行情 `hq` with extended行情 `exhq`; extended market APIs remain separate.
- Do not replace local TDX file imports.

## Capabilities

### New Capabilities
- `marketd-tdx-hq-data-apis`: Fetch the documented TDX standard行情 A-share read APIs through Go-native protocol calls and read-only CLI commands.

### Modified Capabilities

## Impact

- New protocol packet builders, response decoders, validation helpers, and client methods under `internal/tdx`.
- New CLI routing, argument validation, JSON output, and tests under `internal/cli`.
- Documentation updates in `docs/design/tdx-server-capabilities.md`, `docs/reference/tdx-python-libraries.md`, and `docs/tdx-data/通达信数据格式.md`.
- No ClickHouse writes, schema changes, migrations, or destructive database operations.
