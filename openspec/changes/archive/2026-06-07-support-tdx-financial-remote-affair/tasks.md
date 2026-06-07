## 1. Manifest And Download Core

- [x] 1.1 Add `gpcw.txt` manifest entry types, filename validation, and parser tests.
- [x] 1.2 Add HTTP remote client support for manifest listing and file downloads with configurable base URL.
- [x] 1.3 Add size/MD5 verification and matching-existing-file skip behavior.

## 2. Parse Validation

- [x] 2.1 Add parse-only orchestration for a fetched `gpcwYYYYMMDD.zip` or `.dat` file without ClickHouse writes.
- [x] 2.2 Reuse existing financial dictionary and summary output for parse-only results.

## 3. CLI Workflow

- [x] 3.1 Add `tdx-fin-files` command for remote manifest listing.
- [x] 3.2 Add `tdx-fin-fetch` command for selected-file and explicit full-manifest downloads.
- [x] 3.3 Add `tdx-fin-parse` command for parse-only validation.
- [x] 3.4 Add usage text and deterministic CLI tests with local fixture servers.

## 4. Documentation And Validation

- [x] 4.1 Update financial data docs and library comparison docs with the new remote workflow.
- [x] 4.2 Run focused Go tests for finance, ingest, and CLI.
- [x] 4.3 Run `openspec validate support-tdx-financial-remote-affair --strict`.
- [x] 4.4 Run broader repository validation or document any blocker.
