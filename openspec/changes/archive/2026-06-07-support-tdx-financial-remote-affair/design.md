## Context

`support-tdx-financial-data-import` closed the local parse/import path for already-downloaded `tdxfin.zip` and `tdxgp.zip`. `mootdx.affair` covers a different workflow: it lists remote `gpcwYYYYMMDD.zip` packages from `gpcw.txt`, fetches selected packages, and parses a single downloaded report package.

TDX also exposes the same `gpcw.txt` and `gpcwYYYYMMDD.zip` files through `https://data.tdx.com.cn/tdxfin/`. This change uses that file source for `marketd` operator commands while keeping the existing local import commands free of remote network dependencies.

## Goals / Non-Goals

**Goals:**

- Add commands equivalent to `Affair.files`, `Affair.fetch`, and `Affair.parse` for professional financial `gpcw` report packages.
- Parse the remote `gpcw.txt` manifest format: `filename,md5,size`.
- Verify fetched files against manifest MD5 and size when metadata is available.
- Reuse the existing `tdxfin` parser/dictionary and dry-run summary output for parse validation.
- Keep tests deterministic with local HTTP fixtures.

**Non-Goals:**

- Do not add ClickHouse schema, tables, or fact columns.
- Do not make `import-tdx-fin` download files implicitly.
- Do not implement the full TDX HQ `get_report_file_by_size` binary protocol in this change.
- Do not add financial wide tables or derived fundamental metrics.

## Decisions

### Use HTTPS file endpoints for remote packages

The default remote base URL will be `https://data.tdx.com.cn/tdxfin/`. `tdx-fin-files` downloads `<base>/gpcw.txt`, and `tdx-fin-fetch` downloads `<base>/<filename>`.

Alternative considered: implement the HQ binary `get_report_file_by_size` request used by `mootdx`. That would couple this work to a protocol command not yet present in `internal/tdx` and is unnecessary because the same files are available through stable HTTPS paths documented by TDX tooling.

### Keep remote fetching separate from local import

`tdx-fin-files`, `tdx-fin-fetch`, and `tdx-fin-parse` are explicit operator commands. `import-tdx-fin` continues to read local files only.

Alternative considered: add `--download` to `import-tdx-fin`. That would blur read/write behavior, make imports dependent on remote availability, and violate the existing local-import contract.

### Put manifest and download logic in `internal/tdx/finance`

The finance package already owns `gpcw` parsing and dictionary metadata. Adding manifest parsing, URL-safe filename validation, MD5 checks, and HTTP download helpers there keeps the protocol/file-format boundary in one place.

Alternative considered: put download logic in CLI. That would make testing parser/download behavior harder and leak TDX file semantics into command code.

### Parse command is dry-run only

`tdx-fin-parse --file <gpcwYYYYMMDD.zip>` prints the same summary style as `import-tdx-fin --dry-run` without opening ClickHouse. Operators can then run `import-tdx-fin --file <path>` for database writes when they intentionally want persistence.

Alternative considered: make parse an alias for import. That would make an Affair-like parse command unexpectedly able to write data, which is too risky for a download/inspection workflow.

## Risks / Trade-offs

- Remote `gpcw.txt` format changes -> Fail with a clear malformed-manifest error and keep parser tests pinned to fixture content.
- HTTPS endpoint availability changes -> Support `--base-url` so operators can point at mirrors or test servers.
- Partial downloads or stale local files -> Validate byte size and MD5 after download; skip only when an existing file matches known manifest metadata.
- Very large full-history downloads -> Fetch one file by default unless `--all` is explicitly requested; print per-file status.
