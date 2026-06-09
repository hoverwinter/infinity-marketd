## MODIFIED Requirements

### Requirement: Keep Beijing online discovery unsupported until verified
The system SHALL support Beijing online security-list discovery after the TDX standard行情 count/list path is verified, and SHALL keep discovery rejected when that verification is absent.

#### Scenario: Beijing security list supported after verification
- **WHEN** the operator runs `marketd quote-sweep --market bj` against a standard HQ server with the verified Beijing security-list path
- **THEN** the command discovers Beijing securities online
- **AND** it fetches quote snapshots for the discovered `bj` securities through the quote sweep workflow

#### Scenario: Beijing HQ securities API supported after verification
- **WHEN** a client requests `GET /api/tdx/hq/securities?market=bj` against a configured TDX provider
- **THEN** the API returns Beijing security-list entries with `market` equal to `bj`

#### Scenario: Beijing security list remains explicit when unverified
- **WHEN** the Beijing standard HQ count/list path is not verified for the selected server or implementation
- **THEN** Beijing security-list discovery fails with a clear unsupported or source-failure error
- **AND** the system does not silently query another market or source
