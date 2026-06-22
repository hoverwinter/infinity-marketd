## ADDED Requirements

### Requirement: Live online adjustment stays outside persisted adjusted queries
The system SHALL keep one-shot online adjusted bars separate from persisted `/api/v1/bars` adjusted queries.

#### Scenario: Persisted adjusted query remains ClickHouse-backed
- **WHEN** a client requests `/api/v1/bars?adjust=qfq` or `/api/v1/bars?adjust=hfq`
- **THEN** the system SHALL read raw bars and adjustment factors from ClickHouse
- **AND** it SHALL NOT call live TDX HQ bar, XDXR, or online adjusted-bar provider paths

#### Scenario: Online adjusted provider does not replace factor refresh
- **WHEN** a caller requests one-shot online adjusted bars from `/api/tdx/*` or the matching `marketd` provider command
- **THEN** the system SHALL compute adjustment for that response in memory
- **AND** it SHALL NOT persist adjustment factors
- **AND** it SHALL NOT satisfy later `/api/v1/bars?adjust=...` queries unless the normal persisted factor refresh has run
