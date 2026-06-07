## Context

`marketd` currently has a narrow, reliable import path for TDX A-share OHLCV files: `.day`, `.lc1`, `.lc5`, `.1`, and `.5` under standard A-share `vipdoc/sh|sz|bj` conventions. That path can read files from a client directory or extracted offline package, but the missing scope in this change is specifically `client-local`: files maintained by an installed TDX desktop client.

TDX sources are treated as three separate classes:

- `client-local`: installed client state such as `vipdoc/`, `T0002/blocknew/`, local `gbbq`, and extension-market directories such as `Lxxx`.
- `offline-package`: downloaded ZIP/`.dat` packages such as `hsjday.zip`, `tdxfin.zip`, and `tdxgp.zip`.
- `online-provider`: live request/response reads from TDX `hq` / `exhq` servers.

The missing `client-local` reader surface is not more of the same bar data. It includes:

- `gbbq` capital-change and corporate-action events;
- vendor/system block files and user custom block files;
- local extension-market daily files (`ex_daily`), whose local paths are not the A-share `vipdoc/sh|sz|bj/lday` convention and may appear as explicit files such as `29#A1801.day` under extension-market directories like `Lxxx` or `vipdoc/ds/lday`;
- write operations for user custom block files.

Online provider reads already exist for overlapping data (`hq-xdxr`, `hq-block`, `exquote-bars`), but those are live upstream requests. Offline package imports also exist or are proposed for downloaded packages. Neither source class provides deterministic reading of the user's installed TDX client state, ClickHouse storage for that state, watermarks, or product query contracts.

## Goals / Non-Goals

**Goals:**

- Parse and import client-local `gbbq` into an A-share capital-change event table.
- Parse and import client-local TDX system block and custom block files into snapshot-based block definition and membership tables.
- Add guarded custom block write support for local TDX user files.
- Parse and import client-local extension-market daily bars into a separate extension-market table.
- Reuse existing import behavior: dry-run, task runs, watermarks, quality issues, local-only reads, batch writes, and duplicate/conflict resolution before ClickHouse insert.
- Keep schemas additive and non-destructive.

**Non-Goals:**

- No replacement of online `hq-xdxr`, `hq-block`, or `exquote-bars` provider APIs.
- No hidden writes from online provider APIs into the new tables.
- No offline package import; `hsjday.zip`, `tdxfin.zip`, `tdxgp.zip`, and similar downloaded packages remain separate package-import work.
- No professional financial package import; `tdxfin.zip` and `tdxgp.zip` remain covered by `support-tdx-financial-data-import`.
- No adjustment-factor, turnover, market-cap, or block-index derived tables in this change.
- No destructive ClickHouse migration.
- No automatic mutation of custom block files during import commands.

## Decisions

### Store Client-Local `gbbq` As A-Share Capital-Change Events

Add `infinity_market.a_share_capital_change_events`:

```sql
CREATE TABLE IF NOT EXISTS infinity_market.a_share_capital_change_events
(
    market LowCardinality(String),
    symbol String,
    event_date Date,
    category UInt8,
    event_seq UInt16,
    event_name LowCardinality(String),
    cash_dividend Nullable(Float64),
    allotment_price Nullable(Float64),
    bonus_shares Nullable(Float64),
    allotment_shares Nullable(Float64),
    shrink_shares Nullable(Float64),
    pre_float_shares Nullable(Float64),
    post_float_shares Nullable(Float64),
    pre_total_shares Nullable(Float64),
    post_total_shares Nullable(Float64),
    ratio_denominator Nullable(Float64),
    exercise_price Nullable(Float64)
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(event_date)
ORDER BY (market, symbol, event_date, category, event_seq);
```

Rationale:

- `gbbq` is client-local event data, not a bar or financial-report wide table.
- The schema mirrors the fields already understood by the online `hq-xdxr` decoder where possible.
- `event_seq` allows multiple same-day events with the same category without inventing a source/version column.
- Rebuildable derived data such as复权 factors and turnover can be added later as explicit refresh jobs.

Alternative considered: store all event fields as JSON. Rejected because core fields are known, queryable, and needed for downstream calculations.

### Store Client-Local Blocks As Content Snapshots, Not A Mutable Current-State Table

Add three tables:

```sql
CREATE TABLE IF NOT EXISTS infinity_market.tdx_block_snapshots
(
    snapshot_id String,
    block_scope LowCardinality(String),
    snapshot_time DateTime64(3, 'Asia/Shanghai'),
    content_hash String,
    block_count UInt32,
    member_count UInt32
)
ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(snapshot_time)
ORDER BY (block_scope, snapshot_time, snapshot_id);

CREATE TABLE IF NOT EXISTS infinity_market.tdx_block_definitions
(
    snapshot_id String,
    block_scope LowCardinality(String),
    block_kind LowCardinality(String),
    block_id String,
    block_name String,
    block_type UInt16,
    display_order UInt32,
    member_count UInt32
)
ENGINE = ReplacingMergeTree
ORDER BY (snapshot_id, block_scope, block_id);

CREATE TABLE IF NOT EXISTS infinity_market.tdx_block_memberships
(
    snapshot_id String,
    block_scope LowCardinality(String),
    block_id String,
    member_order UInt32,
    code String,
    market LowCardinality(String),
    symbol String
)
ENGINE = ReplacingMergeTree
ORDER BY (snapshot_id, block_scope, block_id, market, symbol, member_order);
```

`block_scope` is `system` or `custom`. `block_kind` distinguishes TDX families such as `block`, `block_zs`, `block_fg`, `block_gn`, and `custom`. `snapshot_id` is a deterministic hash of the normalized snapshot content, not a wall-clock version.

Rationale:

- A mutable current membership table cannot represent removals without deletes or table replacement.
- Snapshot storage is append-only and works with the project rule against destructive assistant migrations.
- Queries can select the latest snapshot per scope or a specific snapshot for reproducibility.

Alternative considered: use one current table keyed by `(block_id, market, symbol)`. Rejected because removed members would remain stale unless the importer performed destructive replacement.

### Keep Custom Block Writes Explicit And Guarded

Custom block writes use a separate command from import, for example:

```text
marketd write-tdx-custom-block --file ... --block-id watch --add sh:600519 --remove sz:000001
```

The command must:

- require an explicit target file;
- parse and validate the existing file before writing;
- write a backup before replacement;
- write to a temporary file and atomically rename it;
- re-read the final file and compare normalized content to the requested operation;
- support `--dry-run` that prints the planned normalized result without modifying files.

Rationale:

- Custom block files are user-owned local state, not market facts.
- Silent mutation from an import command would be surprising and risky.

Alternative considered: automatically sync ClickHouse custom block rows back into local TDX. Rejected because there is no current ClickHouse-side editing contract and it risks corrupting user files.

### Keep Client-Local Extension-Market Daily Bars Separate From A-Share Bars

Add `infinity_market.tdx_ex_bars_1d`:

```sql
CREATE TABLE IF NOT EXISTS infinity_market.tdx_ex_bars_1d
(
    ex_market UInt16,
    code String,
    trade_date Date,
    open Float64,
    high Float64,
    low Float64,
    close Float64,
    position Int64,
    trade Int64,
    price Nullable(Float64),
    amount Nullable(Float64),
    settlement_price Nullable(Float64)
)
ENGINE = ReplacingMergeTree
PARTITION BY toYear(trade_date)
ORDER BY (ex_market, code, trade_date);
```

Rationale:

- Extension markets use numeric market ids and instrument codes, not A-share `sh` / `sz` / `bj` symbols.
- The table shape follows the existing `ExBar` model instead of forcing extension data into A-share OHLCV tables.
- Local `ex_daily` import can later be cross-checked against online `exquote-bars`, but online reads remain non-persistent provider calls.
- The first implementation should require explicit `--file`, `--market`, and `--code` unless fixture-backed discovery for a concrete local path family, such as `Lxxx` or `vipdoc/ds/lday`, has been added.

Alternative considered: add extension rows to `a_share_bars_1d` with synthetic markets. Rejected because it would break the A-share table contract and market identity semantics.

### Use Existing Ops Tables For Provenance And Quality

Input paths, file hashes, parser formats, row counts, and failures belong in `infinity_ops.task_runs`, `watermarks`, and `data_quality_issues`. The new market/reference tables store normalized domain rows and avoid operational source columns in fact tables.

## Risks / Trade-offs

- `gbbq` file variants may differ between TDX builds -> implement parser fixtures from real files and compare a sample against `hq-xdxr`.
- Local block/custom block formats may differ by client version -> start with documented/common formats, record unsupported format quality issues, and avoid partial writes.
- Snapshot block storage grows over time -> deduplicate by normalized `snapshot_id` and document latest-snapshot query patterns.
- Custom block write can corrupt user files -> require explicit file path, backup, temp-file rename, and post-write validation.
- ExHQ local daily formats and directories may vary across clients -> require explicit `--file`, `--market`, and `--code` first; add `Lxxx` or `vipdoc/ds/lday` discovery only after fixture-backed confirmation, and preserve unknown optional fields as quality issues rather than guessing.

## Migration Plan

1. Add non-destructive bootstrap DDL for the new tables.
2. Add parser fixtures and decoder tests before wiring imports.
3. Add dry-run import commands for `gbbq`, blocks, custom blocks, and `ex_daily`.
4. Add store insert paths and non-dry-run import orchestration.
5. Add guarded custom block write command after read/parse coverage is stable.
6. Update docs and run `go test ./...` plus `openspec validate --all`.

Rollback is non-destructive: stop using the new commands. Existing A-share OHLCV tables and online provider APIs are unaffected.

## Open Questions

- Which exact local `gbbq` filename/location conventions should the first implementation discover by `--root`, versus requiring explicit `--file`?
- Which local custom block variants should be writable in the first release, and which should be read-only until fixture coverage exists?
- Which `ex_daily` local directory family should be treated as canonical for discovery in this repo: the user's observed `Lxxx` directories, pytdx-style explicit files such as `29#A1801.day`, `vipdoc/ds/lday`, or all of them behind separate tested discovery rules?
- Should latest block snapshot query helpers be exposed through `/api/v1` in a later change, or should initial consumers query ClickHouse directly?
