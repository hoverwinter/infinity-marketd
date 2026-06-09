## MODIFIED Requirements

### Requirement: Standard HQ server operations
The system SHALL provide read-only TDX standard行情 server operations for probing, session setup, retries, and security discovery, including verified Beijing Stock Exchange security discovery.

#### Scenario: Probe standard HQ servers
- **WHEN** the operator runs a standard HQ server probe command with one or more servers
- **THEN** the system attempts standard HQ connection setup for each server
- **AND** it returns success, latency, errors, and the preferred reachable server as JSON

#### Scenario: Fetch security count and list
- **WHEN** the operator requests standard HQ security count or security list data for a supported market
- **THEN** the system requests the data from the provided standard HQ server
- **AND** it returns market, symbol, decoded name when available, volume unit, decimal point, and previous close fields as JSON

#### Scenario: Fetch Beijing security count and list
- **WHEN** the operator requests standard HQ security count or security list data for `bj`
- **THEN** the system requests the verified Beijing count/list data from the provided standard HQ server
- **AND** it returns discovered securities with `market` equal to `bj`

#### Scenario: Reject unsupported security discovery market
- **WHEN** the operator requests security discovery for a market whose standard HQ count/list behavior is not verified
- **THEN** the system rejects the request before returning discovered securities for that market
