## Context

The canonical `a_share_bars_1d` table already accepts raw daily OHLCV from local `.day` files and the standard HQ online K-line interface. Live quotes are a separate read path and must not create provisional canonical bars. The official broker `g4day` package is a small post-close cross-section: each market has a 150-byte code directory (`.cod`) and a same-length sequence of 512-byte quote records (`.md1`). The package date is encoded in all six entry names.

The verified 2026-09-04 package contains 52,477 directory records. Exact A-share code classification plus positive traded values yields 5,547 stock bars. Eighteen recognized equity codes have no traded bar and must not create synthetic rows. A broader Beijing `4/8/9` prefix rule would incorrectly include the traded indices `899050` and `899601`, so it is not used.

## Goals / Non-Goals

**Goals:**

- Import one official, finalized `g4day` package into the existing raw daily table.
- Support official HTTP download by explicit date and deterministic replay from a local ZIP.
- Validate the entire archive before any market row is written.
- Keep expected non-equity and no-trade records distinguishable from malformed package data.
- Record normal task, watermark, skipped-row, digest, and dry-run outcomes.

**Non-Goals:**

- Construct a daily bar from realtime quotes or write provisional intraday values to `a_share_bars_1d`.
- Import funds, bonds, indices, future listings, or other non-A-share records from `g4day`.
- Support `g3day`, schedule the command, expose a Console action, or refresh adjusted bars.
- Persist the downloaded ZIP or add schemas, tables, columns, dependencies, or a new binary.

## Decisions

### Parse the package in memory as one atomic input

The official ZIP is approximately 2.7 MB compressed and 35 MB expanded. `internal/tdx` will accept ZIP bytes, validate entry sizes before reading, pair each market's `.cod` and `.md1`, then produce a single normalized result. This avoids temporary extraction and path traversal concerns while allowing both HTTP and local-file sources to use exactly the same parser.

HTTP and local inputs are bounded by a compressed-package size limit. ZIP entries, record counts, and total expanded bytes are also bounded to reject accidental or hostile oversized inputs.

### Treat structural and eligible-row corruption as fatal

All `sh`, `sz`, and `bj` pairs are required. Entry dates must agree with each other and with `--date` when supplied. `.cod` length must be divisible by 150, `.md1` by 512, pair counts must match, and codes must be unique within a market. A violation rejects the package before `RunOnlineJob` writes rows.

Recognized A-share rows with all zero/non-traded values are expected and counted as skipped. A recognized row with non-finite values, contradictory high/low relationships, or partially valid traded data is corruption and rejects the package. This is stricter than accepting a partial market day because one atomic daily cross-section can be downloaded again or repaired through the existing HQ importer.

### Use an explicit equity classifier

The accepted code families are Shanghai `6xxxxx`; Shenzhen `000xxx`, `001xxx`, `002xxx`, `003xxx`, `300xxx`, and `301xxx`; and current Beijing stock codes `920xxx`. All other valid directory records are expected package content but outside the canonical A-share scope and count as skipped without quality issues.

For traded rows the parser reads open/high/low/close as little-endian `float64` values at offsets 12, 20, 28, and 36; volume as `uint64` at offset 56; and amount as `float64` at offset 72. The filename date becomes `trade_date` in `Asia/Shanghai`.

### Reuse the existing online job lifecycle

`internal/ingest` will fetch or read bytes, invoke the TDX parser, and pass all normalized bars to `RunOnlineJob`. The job uses dataset and target `a_share_bars_1d`, task type `tdx_g4_daily_import`, asset `all`, `Store.InsertDailyBars`, and existing daily watermark bounds. The summary exposes the source, SHA-256, package/record counters, rows, skips, and issues. Dry-run performs every read and validation step but supplies no store writes.

The official URL is resolved as `<base-url>/YYYYMMDD.zip`, with a test-overridable base URL and HTTP client. Remote mode requires `--date`; local mode derives the date from the archive and treats an optional `--date` as an equality assertion.

## Risks / Trade-offs

- **The binary layout is empirically derived rather than publicly versioned** -> Require exact entry and record shapes, validate all values, identify the input format as `tdx.g4day.cod150.md1-512.v1`, and fail closed on changes.
- **The URL can be republished under the same date** -> Report a SHA-256 digest for every run so operators can compare artifacts; canonical facts remain provider-metadata-free.
- **Code prefixes can evolve** -> Keep classification explicit and tested; extending it requires evidence rather than silently importing every directory record.
- **A package can be unavailable immediately after the close** -> Return the HTTP failure without synthesizing data; the operator can retry or use `import-tdx-hq-day`.
- **Repeated imports can overlap an existing logical key** -> Reuse the table's existing logical identity and insertion behavior, consistent with other daily import paths.

## Migration Plan

No migration is required. Build the existing `marketd` binary, run a remote or local dry-run for a known date, then run the same command without `--dry-run`. Rollback consists of stopping use of the command; no schema or existing records are changed by deployment itself.

## Open Questions

Scheduling and Console exposure remain separate product decisions after the manual command has operational history.
