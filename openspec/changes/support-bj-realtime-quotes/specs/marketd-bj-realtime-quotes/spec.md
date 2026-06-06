## ADDED Requirements

### Requirement: Verify Beijing TDX standard quote mapping
The system SHALL verify Beijing Stock Exchange realtime quote behavior before accepting `bj` quote requests.

#### Scenario: Mapping verified before support
- **WHEN** Beijing realtime quote support is implemented
- **THEN** the implementation records the verified TDX market mapping and sample source in documentation

#### Scenario: Mapping not verified
- **WHEN** the Beijing TDX standard行情 mapping cannot be verified
- **THEN** `bj` realtime quote requests remain rejected with a clear unsupported-market error

### Requirement: Fetch explicit Beijing realtime quotes
The system SHALL fetch realtime quote snapshots for explicit Beijing symbols when the verified TDX standard行情 path supports them.

#### Scenario: Explicit Beijing quote
- **WHEN** the operator runs `marketd quote --symbol bj:920001`
- **THEN** the command returns a JSON quote snapshot with `market` equal to `bj`

#### Scenario: Beijing quote fields
- **WHEN** a valid Beijing quote response is decoded
- **THEN** the returned quote includes price, last_close, open, high, low, volume, amount, server_intraday_time, bids, and asks using the same field semantics as `sh` and `sz`

### Requirement: Infer Beijing realtime quote market
The system SHALL infer Beijing market for supported Beijing code prefixes in realtime quote requests.

#### Scenario: Infer 920 prefix
- **WHEN** the operator runs `marketd quote --symbol 920001`
- **THEN** the request is treated as `bj:920001`

#### Scenario: Infer 8 or 4 prefix
- **WHEN** the operator runs `marketd quote --symbol 830001` or `marketd quote --symbol 430001`
- **THEN** the request is treated as a Beijing realtime quote request

### Requirement: Discover Beijing online symbols
The system SHALL support Beijing online security-list discovery when the verified TDX standard行情 path supports it.

#### Scenario: Beijing security list
- **WHEN** the operator requests online symbols for `bj`
- **THEN** the system returns discovered six-digit Beijing symbols

#### Scenario: Beijing quote sweep
- **WHEN** the operator runs `marketd quote-sweep --market bj`
- **THEN** the system discovers Beijing symbols and fetches realtime quotes through the batch workflow

### Requirement: Preserve unsupported behavior until verified
The system SHALL keep unsupported Beijing behavior explicit until the verified implementation is complete.

#### Scenario: Unsupported before verification
- **WHEN** `bj` support has not been implemented
- **THEN** `marketd quote --symbol bj:920001` exits with validation error rather than silently querying another market

### Requirement: Document Beijing realtime quote support
The system SHALL document the verified Beijing quote behavior and operator commands.

#### Scenario: Documentation updated
- **WHEN** Beijing realtime quote support is implemented
- **THEN** `docs/design/tdx-realtime-quotes.md` describes the TDX market mapping, sample commands, and any limitations
