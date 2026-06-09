## ADDED Requirements

### Requirement: Store mutable securities master data in MySQL
The system SHALL store mutable securities reference data in MySQL while keeping market and trading facts in ClickHouse.

#### Scenario: Bootstrap securities master schema
- **WHEN** the operator runs the schema bootstrap command with MySQL configured
- **THEN** the system creates the `securities`, `security_name_history`, `security_aliases`, and `security_refresh_runs` tables if they do not exist
- **AND** it does not create or rewrite any ClickHouse market fact table for securities metadata

#### Scenario: Keep fact storage in ClickHouse
- **WHEN** OHLCV, transaction, adjustment, or derived market data is imported or queried
- **THEN** the system continues to use ClickHouse fact tables for that data
- **AND** it does not store those facts in MySQL securities-master tables

#### Scenario: MySQL unavailable does not break bars
- **WHEN** MySQL is missing, unreachable, or not configured
- **THEN** commands and endpoints that require the securities master fail with a clear MySQL configuration or connection error
- **AND** existing ClickHouse-backed bar import and bar query behavior remains independent of MySQL

### Requirement: Represent current securities, history, aliases, and refresh runs
The system SHALL represent security identity data with one unified schema for `sh`, `sz`, and `bj`.

#### Scenario: Store current security metadata
- **WHEN** a refresh source provides a security row
- **THEN** the system can persist current metadata keyed by `(market, symbol)`
- **AND** the row includes exchange, current name, normalized current name, board, status, listing date, delisting date, lot size, price precision, source, manual lock state, created time, and updated time

#### Scenario: Store Beijing securities in the unified table
- **WHEN** a Beijing Stock Exchange security is persisted
- **THEN** the system stores it with `market` equal to `bj`
- **AND** it stores exchange and board metadata without requiring a separate Beijing-specific table

#### Scenario: Store name history
- **WHEN** a source or manual correction identifies an old or effective-dated security name
- **THEN** the system can persist a name-history segment for `(market, symbol, valid_from, name)`
- **AND** a manual override segment is protected from later source refresh overwrite

#### Scenario: Store searchable aliases
- **WHEN** a source or manual correction provides a searchable alias
- **THEN** the system can persist the alias with normalized text, alias type, priority, and source
- **AND** resolve can search aliases without changing the current security name

#### Scenario: Audit refresh execution
- **WHEN** a securities-master refresh command runs
- **THEN** the system records source, requested markets, timing, status, row counts, and error text when present in `security_refresh_runs`

### Requirement: Refresh securities master with explicit source and market selection
The system SHALL provide commands to refresh securities master data from an explicitly selected source and market set.

#### Scenario: Refresh from TDX for selected markets
- **WHEN** the operator runs a securities-master refresh with `--source tdx` and one or more `--market` values
- **THEN** the system fetches security-list data from TDX standard HQ for the requested markets
- **AND** it normalizes and upserts current metadata, aliases, and history into MySQL

#### Scenario: Refresh Beijing list from TDX
- **WHEN** the operator runs a securities-master refresh with `--source tdx --market bj`
- **THEN** the system uses the verified TDX standard HQ Beijing security count/list path
- **AND** it stores returned securities with `market` equal to `bj`

#### Scenario: Reject silent source fallback
- **WHEN** the selected source cannot fetch the requested market
- **THEN** the command reports that source failure
- **AND** it does not silently query another source to fill the result

#### Scenario: Dry run refresh
- **WHEN** the operator runs a securities-master refresh with `--dry-run`
- **THEN** the system fetches and normalizes source rows
- **AND** it reports row counts without writing current securities, aliases, name history, or refresh-run success records

#### Scenario: Preserve manual locks during refresh
- **WHEN** a refresh row matches a current security whose protected fields are manually locked
- **THEN** the system preserves the protected current metadata fields
- **AND** it may still record non-conflicting aliases or refresh audit data

#### Scenario: Import normalized source file
- **WHEN** the operator runs a securities-master refresh with a normalized file source
- **THEN** the system reads the declared source rows from the file
- **AND** it applies the same normalization, validation, upsert, dry-run, and manual-lock rules as online refresh sources

### Requirement: Query securities master through base APIs
The system SHALL expose exact lookup and resolve APIs for securities master data.

#### Scenario: Exact security lookup
- **WHEN** a client requests `GET /api/v1/securities?market=sh&symbol=600519`
- **THEN** the system returns the current metadata for `sh:600519` from MySQL
- **AND** it returns 404 when the security does not exist

#### Scenario: Resolve by code, name, history, or alias
- **WHEN** a client requests `GET /api/v1/securities/resolve?q=贵州茅台`
- **THEN** the system searches symbol, current name, historical names, and aliases
- **AND** it returns ranked candidate securities with match metadata

#### Scenario: Preserve ambiguity
- **WHEN** a resolve query matches more than one security
- **THEN** the system returns multiple candidates
- **AND** it does not silently choose a single canonical security for the caller

#### Scenario: Bars do not join securities metadata
- **WHEN** a client requests `/api/v1/bars`
- **THEN** the system reads bars from ClickHouse only
- **AND** it does not query MySQL or include joined security names in the bars response

### Requirement: Configure MySQL explicitly
The system SHALL configure MySQL through application configuration and environment/flag overrides without hard-coded DSNs.

#### Scenario: Load MySQL config
- **WHEN** the operator provides MySQL host, port, database, user, password, and connection-pool settings in config or supported overrides
- **THEN** commands and services that need the securities master use those settings to connect

#### Scenario: Missing MySQL config for securities command
- **WHEN** the operator runs a securities-master command without required MySQL config
- **THEN** the command fails before writing data
- **AND** the error identifies the missing MySQL configuration
