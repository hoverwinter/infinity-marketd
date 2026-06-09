## Context

`marketd` currently treats `market + symbol` as the identity for market facts and keeps OHLCV, adjustment, and derived data in ClickHouse. That is the right storage model for行情 and trading facts, but it leaves mutable reference data outside the system: current names, aliases, historical names, listing status, manual corrections, and code-to-name lookup.

The existing TDX standard HQ path can discover `sh` and `sz` security lists and can fetch explicit `bj` realtime quotes, but Beijing Stock Exchange online security-list discovery is still explicitly disabled. The securities master should close both gaps without changing bar storage or making `/api/v1/bars` perform metadata joins.

Local inspection also shows that MySQL is not installed as a local daemon on this developer machine, while the environment has remote MySQL usage patterns in other projects. This change therefore designs MySQL as an explicit configured dependency rather than assuming a local default DSN.

## Goals / Non-Goals

**Goals:**
- Store mutable securities reference data in MySQL.
- Keep market facts, trading facts, and derived market data in ClickHouse.
- Provide durable current securities metadata, name history, and aliases for `sh`, `sz`, and `bj`.
- Support source-selected refresh commands with market selection and dry-run behavior.
- Support TDX HQ `bj` security-list discovery once the standard HQ count/list path is verified.
- Expose two base read APIs: exact security lookup and resolve/search.
- Keep `/api/v1/bars` ClickHouse-only and free of joined security names.

**Non-Goals:**
- No migration of OHLCV, tick, transaction, adjustment, or derived market facts into MySQL.
- No bars-plus-name API and no implicit MySQL lookup inside bar queries.
- No business-specific composition layer; callers compose bars and metadata when they need both.
- No source raw-payload warehouse in the first version.
- No hard-coded global source priority such as always preferring AkShare over mootdx.
- No mandatory Python runtime dependency inside the Go daemon for the first implementation.

## Decisions

- Use MySQL only for mutable securities reference data.
  - Rationale: names, aliases, listing status, and manual corrections change and need transactional upserts plus small indexed lookups. ClickHouse remains optimized for append/query fact data.
  - Alternative considered: store securities metadata in ClickHouse. Rejected because frequent small corrections and point lookups are a poor fit for the existing fact-store boundary.

- Keep the schema small: `securities`, `security_name_history`, `security_aliases`, and `security_refresh_runs`.
  - Rationale: these tables cover current lookup, historical names, search aliases, and operational audit. Additional source snapshot tables can be added later if reconciliation requires them.
  - Alternative considered: separate source snapshots and per-source item tables now. Rejected as overbuilt for the first durable master-data capability.

- Use one unified securities table for `sh`, `sz`, and `bj`.
  - Rationale: Beijing securities share the same identity shape: `market`, `symbol`, exchange metadata, name, status, and aliases. A separate BJ table would duplicate rules and complicate resolve.
  - Detail: `market='bj'`, `exchange='BSE'`, and `board='bse'` represent Beijing rows.

- Model source provenance at the current-row and refresh-run level, not as raw snapshots.
  - Rationale: operators need to know where the current value came from and whether refreshes succeeded. They do not need a full source warehouse in this change.
  - Detail: `source` is allowed in MySQL reference tables. The existing rule that fact tables must not add `source`, `version`, or `updated_at` remains unchanged.

- Make refresh source selection explicit and source-specific.
  - Rationale: AkShare, mootdx, TDX, local files, and manual patches have different coverage and failure modes. The command contract should let operators choose a source instead of hiding two queries inside an API.
  - Detail: first implementation should support native `tdx` discovery and a normalized `file` import path. AkShare or mootdx can feed the normalized file path, or later become direct adapters without changing the MySQL schema.

- Preserve manual corrections.
  - Rationale: names, aliases, and listing metadata sometimes need operator fixes. Refreshes must not overwrite protected values.
  - Detail: `securities.manual_locked` protects current-row fields; `security_name_history.manual_override` protects history segments.

- Keep query APIs narrow.
  - Rationale: this project provides base data capabilities. Business callers decide whether and when to query metadata separately from bars.
  - Detail: exact lookup requires `market + symbol`; resolve returns candidates for code/name/alias input and never silently chooses one ambiguous security.

- Add MySQL config beside ClickHouse config.
  - Rationale: existing config loading and command patterns already centralize environment and flag overrides. MySQL must be explicit and documented, not hard-coded.
  - Detail: missing MySQL config should fail only commands and endpoints that require the securities master; existing ClickHouse import/query flows remain independent.

- Enable TDX HQ `bj` security discovery only through the verified standard HQ count/list path.
  - Rationale: explicit Beijing quotes already work, but list discovery was previously disabled because live market byte behavior was not verified. This change should verify and test the list path, not silently substitute another source for `--source tdx --market bj`.

## Storage Format

The first MySQL schema uses `utf8mb4` and stores times in UTC or explicit application timestamps. Date fields use exchange-calendar dates, not ingestion dates.

`securities`:
- `market` varchar: canonical market key, one of `sh`, `sz`, `bj`.
- `symbol` varchar: six-digit security code.
- `exchange` varchar: `SSE`, `SZSE`, or `BSE`.
- `current_name` varchar: current display name from the selected source or manual correction.
- `current_name_norm` varchar: normalized search key.
- `board` varchar: board segment such as `main`, `kcb`, `cyb`, `bse`, or source-known equivalent.
- `status` varchar: `listed`, `suspended`, `delisted`, or `unknown`.
- `listing_date` date nullable.
- `delisting_date` date nullable.
- `lot_size` int nullable.
- `price_precision` tinyint nullable.
- `source` varchar: source that last updated the unprotected current row.
- `manual_locked` bool: prevents refresh overwrite of protected current fields.
- `created_at`, `updated_at` datetime.
- Primary key: `(market, symbol)`.
- Indexes: `(current_name_norm)`, `(status)`, `(exchange, board)`.

`security_name_history`:
- `id` bigint auto-increment primary key.
- `market`, `symbol`.
- `name`, `name_norm`.
- `valid_from` date.
- `valid_to` date nullable.
- `source` varchar.
- `manual_override` bool.
- `created_at`, `updated_at` datetime.
- Unique key: `(market, symbol, valid_from, name_norm)`.
- Indexes: `(name_norm)`, `(market, symbol, valid_from)`.

`security_aliases`:
- `id` bigint auto-increment primary key.
- `market`, `symbol`.
- `alias`, `alias_norm`.
- `alias_type` varchar: `name`, `pinyin`, `english`, `old_name`, `manual`, or `source`.
- `priority` int.
- `source` varchar.
- `created_at`, `updated_at` datetime.
- Unique key: `(market, symbol, alias_norm, alias_type)`.
- Indexes: `(alias_norm, priority)`, `(market, symbol)`.

`security_refresh_runs`:
- `id` bigint auto-increment primary key.
- `source` varchar.
- `markets` varchar: comma-separated canonical markets requested by the command.
- `started_at`, `finished_at` datetime nullable.
- `status` varchar: `running`, `succeeded`, `failed`, or `dry_run`.
- `rows_seen`, `rows_upserted`, `rows_skipped`, `aliases_upserted`, `history_upserted` int.
- `error` text nullable.
- Indexes: `(started_at)`, `(source, status)`.

## API Shape

- `GET /api/v1/securities?market=sh&symbol=600519` returns the current security metadata row or 404.
- `GET /api/v1/securities/resolve?q=贵州茅台` returns ranked candidates matched by symbol, current name, historical name, or alias.
- `/api/v1/bars` remains backed only by ClickHouse repository methods and does not query MySQL.

## Migration Plan

1. Add MySQL config fields and environment overrides without changing existing ClickHouse config defaults.
2. Add idempotent MySQL bootstrap DDL for the four securities-master tables.
3. Add repository interfaces and implementations for securities writes and reads.
4. Add source-selected refresh commands and start with `tdx` plus normalized `file` import.
5. Verify TDX HQ `bj` count/list behavior with fixture and live-sample documentation, then enable `bj` discovery.
6. Add exact lookup and resolve endpoints in the read plane without changing bars.
7. Update storage, API, and TDX docs.

## Risks / Trade-offs

- MySQL can be unavailable while ClickHouse is healthy -> securities commands/endpoints fail with clear errors, but bar import/query flows continue.
- External sources disagree or drift -> refresh runs record source and counts; manual locks prevent overwriting operator corrections.
- TDX `bj` list behavior can vary by public server -> `--source tdx --market bj` reports a source failure when the verified path fails; it does not silently fall back to another source.
- Resolve can return ambiguous matches -> API returns candidates with match metadata and leaves final choice to the caller.
- Normalized file import adds an intermediate step for AkShare/mootdx -> avoids embedding Python dependencies in the Go service while keeping the schema and command contract source-neutral.

## Open Questions

- What concrete MySQL database name and user should production use?
- Which normalized file formats should be accepted first: CSV only, JSON Lines only, or both?
- Should direct AkShare/mootdx adapters be implemented later in this repository, or should they remain external producers of normalized files?
