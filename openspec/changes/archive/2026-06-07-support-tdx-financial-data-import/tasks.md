## 1. Metadata And Fixtures

- [x] 1.1 Add version-controlled `tdxfin` financial item dictionary metadata sourced from mootdx field mapping.
- [x] 1.2 Add initial `tdxgp` metric dictionary metadata for confirmed metric types.
- [x] 1.3 Add dictionary loader code with validation for duplicate ids, missing names, invalid status, and unsupported units/value kinds.
- [x] 1.4 Add small deterministic fixtures extracted from `tdxfin.zip` and `tdxgp.zip` for parser tests.

## 2. Parsers

- [x] 2.1 Implement `gpcwYYYYMMDD.dat` parser that emits market, symbol, report_date, item_id, and value.
- [x] 2.2 Implement `tdxfin.zip` traversal, nested `.zip`/`.dat` handling, and `gpcw.txt` manifest validation.
- [x] 2.3 Implement `gp{market}{symbol}.dat` parser that emits market, symbol, metric_type, event_date, value1, and value2.
- [x] 2.4 Implement `tdxgp.zip` traversal and `gpszsh.txt` / `gpszsh.local` validation for interpretable manifest fields.
- [x] 2.5 Add parse quality issues for invalid date, unsupported market/symbol, trailing bytes, checksum mismatch, unknown dictionary id, and zero valid rows.

## 3. ClickHouse Schema And Store

- [x] 3.1 Add bootstrap DDL for `infinity_market.a_share_financial_raw_items`.
- [x] 3.2 Add bootstrap DDL for `infinity_market.a_share_gp_metric_values`.
- [x] 3.3 Add bootstrap DDL for `infinity_market.tdx_financial_item_dictionary`.
- [x] 3.4 Add bootstrap DDL for `infinity_market.tdx_gp_metric_dictionary`.
- [x] 3.5 Add store insert methods for financial raw rows, stock metric rows, and dictionary rows.
- [x] 3.6 Add schema and store tests for table names, order keys, partitions, and no source/version/updated_at columns on raw fact tables.

## 4. Import Orchestration

- [x] 4.1 Add `ImportTDXFinancial` orchestration for `tdxfin.zip` with dry-run support.
- [x] 4.2 Add `ImportTDXGP` orchestration for `tdxgp.zip` with dry-run support.
- [x] 4.3 Sync dictionary tables before writing raw facts in non-dry-run imports.
- [x] 4.4 Buffer writes by partition to avoid small per-file inserts.
- [x] 4.5 Record task runs, watermarks, row counts, skipped rows, and quality issues through existing ops tables.
- [x] 4.6 Ensure imports read only local files and ClickHouse and never connect to remote TDX servers.

## 5. CLI

- [x] 5.1 Add `marketd import-tdx-fin --file ... --dry-run`.
- [x] 5.2 Add `marketd import-tdx-gp --file ... --dry-run`.
- [x] 5.3 Print import summaries with discovered files, rows written, rows skipped, dictionary count, manifest issues, and quality issue count.
- [x] 5.4 Add CLI tests for dry-run, missing file, invalid dictionary id, and zero valid rows.

## 6. Documentation And Validation

- [x] 6.1 Update `docs/tdx-data/专业财务数据.md` with confirmed parser formats and command examples.
- [x] 6.2 Update `docs/storage/clickhouse.md` with raw financial tables and dictionary tables.
- [x] 6.3 Document that derived financial wide tables are out of scope and will be refreshed explicitly in a later change.
- [x] 6.4 Document that `pytdx.trade` / `trade.dll` trading capability is out of scope for `marketd`.
- [x] 6.5 Run `go test ./...`.
- [x] 6.6 Run `openspec validate --all`.
