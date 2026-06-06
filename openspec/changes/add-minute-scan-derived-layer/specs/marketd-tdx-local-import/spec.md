## MODIFIED Requirements

### Requirement: Local 1-minute import
The system SHALL parse local TDX `.lc1` and `.1` files and import 1-minute OHLCV bars into canonical facts only.

#### Scenario: One-minute normalization
- **WHEN** a valid 1-minute record is normalized
- **THEN** bar_time is decoded in `Asia/Shanghai` trading time
- **AND** trade_date is derived from bar_time
- **AND** `.1` prices are divided by `100.0`
- **AND** the record is mapped to `infinity_market.a_share_bars_1m`
- **AND** the logical key is `(market, symbol, bar_time)`
- **AND** offline import MUST NOT generate rows in `a_share_bars_1m_scan` by default

### Requirement: Local 5-minute import
The system SHALL parse local TDX `.lc5` and `.5` files and import 5-minute OHLCV bars into canonical facts only.

#### Scenario: Five-minute normalization
- **WHEN** a valid 5-minute record is normalized
- **THEN** bar_time is decoded in `Asia/Shanghai` trading time
- **AND** trade_date is derived from bar_time
- **AND** `.5` prices are divided by `100.0`
- **AND** the record is mapped to `infinity_market.a_share_bars_5m`
- **AND** the logical key is `(market, symbol, bar_time)`
- **AND** offline import MUST NOT generate rows in `a_share_bars_5m_scan` by default
