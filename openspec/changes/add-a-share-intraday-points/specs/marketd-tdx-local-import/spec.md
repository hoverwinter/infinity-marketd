## ADDED Requirements

### Requirement: Local minute imports do not populate intraday points
The system SHALL keep local TDX 1-minute OHLCV imports separate from persisted TDX standard HQ intraday point imports.

#### Scenario: One-minute local import writes only bars
- **WHEN** an operator runs `marketd import-tdx-1m`
- **THEN** marketd writes valid records to `infinity_market.a_share_bars_1m`
- **AND** it MUST NOT write rows to `infinity_market.a_share_intraday_points`

#### Scenario: Five-minute local import does not write intraday points
- **WHEN** an operator runs `marketd import-tdx-5m`
- **THEN** marketd writes valid records to `infinity_market.a_share_bars_5m`
- **AND** it MUST NOT write rows to `infinity_market.a_share_intraday_points`

#### Scenario: No implicit point derivation from bars
- **WHEN** local minute OHLCV bars are imported
- **THEN** marketd MUST NOT derive intraday points from bar close and volume as an import side effect
