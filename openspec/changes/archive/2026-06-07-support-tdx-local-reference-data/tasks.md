## 1. Client-Local Fixtures And Format Confirmation

- [x] 1.1 Collect minimal client-local `gbbq` fixtures with at least one dividend event, one capital-change event, and one unsupported/unknown category sample.
- [x] 1.2 Collect minimal client-local system block fixtures for `block.dat` and at least one of `block_zs.dat`, `block_fg.dat`, or `block_gn.dat`.
- [x] 1.3 Collect minimal client-local custom block fixtures from `T0002/blocknew` that include multiple blocks, GBK/GB18030 names, duplicate member handling, and an empty block.
- [x] 1.4 Collect minimal client-local `ex_daily` fixtures for at least one extension market, including the user's observed `Lxxx` directory form if available, and document how market id and code are inferred or supplied.
- [x] 1.5 Add fixture documentation under `docs/tdx-data/` describing client-local file locations, record assumptions, and fields verified by tests.

## 2. Parsers

- [x] 2.1 Implement `gbbq` parser types that emit market, symbol, event_date, category, event_seq, event_name, and normalized numeric fields.
- [x] 2.2 Add `gbbq` parser tests for valid events, invalid dates, unknown categories, duplicate logical keys, and zero valid rows.
- [x] 2.3 Implement local system block parser with GBK/GB18030 decoding, block definition extraction, member extraction, market inference, and deterministic normalized snapshot hashing.
- [x] 2.4 Add system block parser tests for block names, block type, member order, member code, inferred market/symbol, malformed file length, and deterministic snapshot id.
- [x] 2.5 Implement local custom block parser with custom block ids, names, member order, member code, inferred market/symbol, and unsupported-variant quality issues.
- [x] 2.6 Add custom block parser tests for multiple blocks, empty blocks, duplicate members, unsupported symbols, malformed files, and deterministic snapshot id.
- [x] 2.7 Implement `ex_daily` parser types that emit ex_market, code, trade_date, OHLC, position, trade, price, amount, and settlement_price where present without assuming A-share `vipdoc/sh|sz|bj` paths.
- [x] 2.8 Add `ex_daily` parser tests for valid daily bars, invalid dates, invalid OHLC, trailing bytes, duplicate logical keys, and unsupported format variants.

## 3. ClickHouse Schema And Store

- [x] 3.1 Add additive bootstrap DDL for `infinity_market.a_share_capital_change_events`.
- [x] 3.2 Add additive bootstrap DDL for `infinity_market.tdx_block_snapshots`.
- [x] 3.3 Add additive bootstrap DDL for `infinity_market.tdx_block_definitions`.
- [x] 3.4 Add additive bootstrap DDL for `infinity_market.tdx_block_memberships`.
- [x] 3.5 Add additive bootstrap DDL for `infinity_market.tdx_ex_bars_1d`.
- [x] 3.6 Add store insert methods for capital-change events, block snapshots, block definitions, block memberships, and extension daily bars.
- [x] 3.7 Add schema tests for table names, partition keys, order keys, and absence of source/source_file/version/updated_at columns on fact tables.
- [x] 3.8 Add store tests or fakes verifying empty batches are no-ops and populated batches use the expected target tables.

## 4. Import Orchestration

- [x] 4.1 Add `ImportTDXGBBQ` orchestration with explicit file input, dry-run, filters where applicable, quality issues, task runs, and watermarks.
- [x] 4.2 Add `ImportTDXBlock` orchestration for `--scope system` and `--scope custom` with snapshot, definition, and membership writes.
- [x] 4.3 Add `ImportTDXExDaily` orchestration with explicit `--market` and `--code` when path inference is ambiguous.
- [x] 4.4 Ensure all new client-local import orchestration reads only local files and ClickHouse and never connects to remote TDX servers.
- [x] 4.5 Resolve duplicate and conflicting logical keys before ClickHouse insert and report conflicts through `data_quality_issues`.
- [x] 4.6 Buffer writes by table and partition to avoid small per-record inserts.
- [x] 4.7 Add import tests for dry-run behavior, missing files, zero valid rows, degraded imports, and successful non-dry-run store calls.
- [x] 4.8 Add tests that reject or route downloaded offline packages such as `hsjday.zip`, `tdxfin.zip`, and `tdxgp.zip` away from client-local import commands.

## 5. Custom Block Write

- [x] 5.1 Implement a normalized custom block edit model for add/remove/replace operations without touching files.
- [x] 5.2 Add validation for requested symbols, duplicate members, missing block ids, unsupported custom block variants, and empty write plans.
- [x] 5.3 Implement `--dry-run` custom block write output that prints the planned normalized result without modifying files.
- [x] 5.4 Implement guarded file write with backup creation, temp-file write, atomic rename, and post-write re-read validation.
- [x] 5.5 Add custom block write tests for successful add/remove, backup failure, parse failure, unsupported symbol, post-write validation failure, and no partial target write.

## 6. CLI

- [x] 6.1 Add `marketd import-tdx-gbbq --file ... --dry-run`.
- [x] 6.2 Add `marketd import-tdx-block --file ... --scope system|custom --dry-run`.
- [x] 6.3 Add `marketd import-tdx-ex-daily --file ... --market ... --code ... --dry-run`; defer `--root` discovery until `Lxxx` or other extension path variants are covered by tests.
- [x] 6.4 Add `marketd write-tdx-custom-block --file ... --block-id ... --add ... --remove ... --dry-run`.
- [x] 6.5 Update command help and CLI tests for required flags, invalid flags, dry-run summaries, and JSON/error output shape.

## 7. Documentation And Validation

- [x] 7.1 Update `docs/storage/clickhouse.md` with the new table schemas, logical keys, partition choices, and latest block snapshot query guidance.
- [x] 7.2 Update `docs/tdx-data/通达信数据格式.md` with confirmed client-local `gbbq`, block/custom block, and `ex_daily` parser contracts.
- [x] 7.3 Update `docs/design/tdx-server-capabilities.md` to clarify that online `hq-xdxr`, `hq-block`, and `exquote-bars` remain provider reads and do not persist local reference data.
- [x] 7.4 Add operator examples for importing `gbbq`, importing system/custom blocks, importing `ex_daily`, and safely writing custom blocks.
- [x] 7.5 Run `go test ./...`.
- [x] 7.6 Run `openspec validate --all`.
