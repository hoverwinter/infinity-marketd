## ADDED Requirements

### Requirement: A-share limit review schema bootstrap
The system SHALL include normalized A-share limit-review tables in the existing idempotent ClickHouse bootstrap.

#### Scenario: Bootstrap adds review tables without replacing data
- **WHEN** an operator runs `marketd bootstrap` on a new or existing deployment
- **THEN** all limit-review tables are created with `CREATE TABLE IF NOT EXISTS`
- **AND** no existing table is dropped, detached, truncated, or destructively replaced

#### Scenario: Review facts follow data-plane invariants
- **WHEN** a limit-review fact table is created
- **THEN** it MUST NOT include source, source key, source file, version, or updated-at columns
- **AND** marketd resolves duplicate/conflicting input logical keys before insertion

### Requirement: Limit review operational metadata
The system SHALL reuse existing ops tables for review imports and refreshes.

#### Scenario: Review task visibility
- **WHEN** a non-dry-run review import succeeds, degrades, or fails
- **THEN** marketd records a task run naming the dataset, task type, target tables, input path/format, parameters, row counts, timing, and error

#### Scenario: Review quality issue visibility
- **WHEN** input rows are malformed, conflict on a logical key, or disagree with daily summary counts
- **THEN** marketd records a data-quality issue with the affected date and symbol when available
