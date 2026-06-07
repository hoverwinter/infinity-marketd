## ADDED Requirements

### Requirement: Local OHLCV imports remain raw-only
The system SHALL keep local TDX OHLCV imports independent from adjusted bar factor refreshes.

#### Scenario: Daily import does not refresh adjustment factors
- **WHEN** an operator runs `marketd import-tdx-day`
- **THEN** the command writes only canonical raw daily OHLCV facts and import quality metadata
- **AND** it MUST NOT fetch xdxr events
- **AND** it MUST NOT refresh qfq or hfq adjustment factors as a hidden side effect

#### Scenario: Minute imports do not refresh adjustment factors
- **WHEN** an operator runs `marketd import-tdx-1m` or `marketd import-tdx-5m`
- **THEN** the command writes only canonical raw minute OHLCV facts and import quality metadata
- **AND** it MUST NOT fetch xdxr events
- **AND** it MUST NOT refresh qfq or hfq adjustment factors as a hidden side effect
