## Context

`marketd` already imports local TDX OHLCV files and records task runs, watermarks, and quality issues. The TDX professional financial page provides two local packages that are different from OHLCV bars:

```text
tdxfin.zip
  gpcwYYYYMMDD.dat / .zip
  full-market report-period financial values

tdxgp.zip
  gp{market}{symbol}.dat
  per-symbol dated stock metric values
```

The public mootdx documentation and source include the professional financial field mapping. That mapping should be treated as version-controlled metadata in this repository, not inferred at import time and not stored only as mutable database state.

## Goals / Non-Goals

**Goals:**

- Import `tdxfin.zip` into a raw financial item fact table.
- Import `tdxgp.zip` into a raw stock metric fact table.
- Store financial and stock-metric dictionaries as version-controlled metadata files.
- Sync the dictionaries into ClickHouse lookup tables during import/bootstrap workflows.
- Reuse existing local import patterns: dry-run, quality issues, task runs, watermarks, batch writes, and no remote TDX dependency.
- Keep raw facts normalized and rebuildable.

**Non-Goals:**

- No financial wide/curated derived tables in this change.
- No financial query API in this change.
- No download client for remote TDX financial packages in this change; imports read local files.
- No dependency on Python, pandas, pytdx, or mootdx at runtime.
- No trading capability. `pytdx.trade` wraps `trade.dll`, but `marketd` is a market data daemon and this change MUST NOT add order submission, account queries, positions, fills, broker login, or `trade.dll` integration.
- No change to OHLCV fact table schemas.

## Decisions

### Use Raw Facts Plus Dictionary, Not a Wide Table

Financial imports write normalized facts:

```text
a_share_financial_raw_items:
  market + symbol + report_date + item_id -> value

a_share_gp_metric_values:
  market + symbol + metric_type + event_date -> value1/value2
```

Rationale:

- It preserves all fields without forcing business-column choices up front.
- It lets dictionary mapping corrections happen without reimporting packages.
- It avoids building a large derived wide table before there is a query contract.

Alternative considered: import directly into `a_share_financial_reports` with named columns. Rejected because it couples parsing, dictionary semantics, and product query shape too early.

### Keep Dictionary Source In The Repository

Dictionary metadata lives in version-controlled files, for example:

```text
internal/tdx/finance/metadata/financial_items.yaml
internal/tdx/finance/metadata/gp_metrics.yaml
```

The ClickHouse dictionary tables are synchronized lookup copies, not the source of truth.

Rationale:

- Dictionary changes need code review and commit history.
- Tests can load the same metadata used by production imports.
- Imports can validate ids without requiring a pre-existing database dictionary.

Alternative considered: maintain dictionary rows only in ClickHouse. Rejected because it makes field meaning mutable outside code review and complicates deterministic tests.

### Validate Unknown IDs

`tdxfin` import treats unknown `item_id` as a quality issue and degrades or fails according to import policy. `tdxgp` import does the same for unknown `metric_type` when that metric dictionary is declared complete.

Rationale:

- The user has confirmed that `tdxfin` field ids can be determined from mootdx documentation.
- Silent unknown fields would undermine downstream derived tables.

### Use Existing Ops Tables

Each import records:

- `task_runs` for the package import;
- `watermarks` by dataset and asset scope;
- `data_quality_issues` for checksum mismatch, invalid record length, unknown dictionary id, invalid date, unsupported market/symbol, and zero valid rows.

Rationale:

- This matches local OHLCV import behavior.
- Operators can inspect financial imports through existing `marketd status` conventions.

### Do Not Store File Source In Raw Fact Tables

Raw financial tables follow the canonical fact table rule: no `source`, `version`, or `updated_at` columns in market facts. Package paths, manifest checksums, and run metadata live in `infinity_ops`.

Rationale:

- Keeps facts normalized and logical-key based.
- Avoids embedding operational provenance in every row.

## Risks / Trade-offs

- Field mapping drift between mootdx versions -> Keep dictionary metadata versioned and cite the upstream source in docs.
- `tdxgp` metric meanings may be less complete than `tdxfin` -> Start with confirmed metrics and make unknown handling explicit.
- Large package imports can create many small inserts -> Buffer by report period/market for `tdxfin` and by market/date partition for `tdxgp`.
- Dictionary table synchronization could be stale -> Import commands sync dictionary rows before writing facts and tests verify dictionary coverage.
- Future derived tables may need different types than raw Float64 -> Build derived refresh later from raw facts and dictionary metadata rather than changing raw storage.
- TDX Python libraries include trading wrappers -> Treat `pytdx.trade` / `trade.dll` as outside the project boundary because it belongs to broker transaction workflows, not the market data ingest daemon.

## Migration Plan

1. Add non-destructive bootstrap DDL for the new market tables.
2. Add metadata files and tests for dictionary loading.
3. Add parsers with small fixtures extracted from local packages.
4. Add dry-run imports and quality reporting.
5. Add ClickHouse insert paths and CLI commands.
6. Update docs for package structure, schema, and operator commands.

Rollback is non-destructive: stop using the new import commands. Existing OHLCV tables and online TDX provider APIs are unaffected.

## Open Questions

- Should `tdxgp` dictionary completeness be enforced from the first implementation, or should unknown metric types be allowed as degraded imports until the mapping is fully confirmed?
- Should dictionary lookup tables live in `infinity_market` with the raw facts, or in a future `infinity_ref` database if more reference metadata accumulates?
