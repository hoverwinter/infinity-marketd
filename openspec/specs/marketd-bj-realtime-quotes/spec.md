# marketd-bj-realtime-quotes Specification

## Purpose
TBD - created by archiving change support-bj-realtime-quotes. Update Purpose after archive.
## Requirements
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

### Requirement: Sweep explicit Beijing symbols
The system SHALL support explicit Beijing symbols in the quote sweep workflow.

#### Scenario: Explicit Beijing quote sweep
- **WHEN** the operator runs `marketd quote-sweep --symbol 920001,bj:920799`
- **THEN** the system fetches Beijing realtime quotes through the batch workflow

#### Scenario: Beijing requests split conservatively
- **WHEN** a quote batch contains a Beijing request
- **THEN** the system sends Beijing quote requests as single-symbol TDX requests until live multi-record response parsing is hardened

### Requirement: Keep Beijing online discovery unsupported until verified
The system SHALL keep Beijing online security-list discovery disabled when the TDX standard行情 list path is not verified.

#### Scenario: Beijing security list unsupported
- **WHEN** the operator runs `marketd quote-sweep --market bj`
- **THEN** the command fails with a clear unsupported security-list market error

### Requirement: Reject mismatched quote responses
The system SHALL reject realtime quote responses whose decoded market or symbol does not match the request.

#### Scenario: Server fallback record
- **WHEN** a Beijing quote request receives an unrelated fallback record such as `sh:600839`
- **THEN** the system returns an identity mismatch error instead of emitting that quote

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

