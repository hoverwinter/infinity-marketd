## ADDED Requirements

### Requirement: Keep TDX extended行情 on a separate quote path
The system SHALL implement TDX extended行情 quote retrieval separately from standard A-share realtime quote retrieval.

#### Scenario: Extended identity uses numeric market ID
- **WHEN** an extended quote request is validated
- **THEN** the request identity uses a positive numeric extended market ID
- **AND** it uses an extended instrument code
- **AND** it does not use `sh`, `sz`, or `bj` as the market identity

#### Scenario: Standard quotes do not accept extended instruments
- **WHEN** an operator or caller provides an extended-market request to the standard realtime quote path
- **THEN** the system rejects the request with a validation error
- **AND** it does not translate the request into an A-share market

### Requirement: Fetch TDX extended market metadata
The system SHALL fetch extended行情 market metadata from a TDX `exhq` server.

#### Scenario: Decode market list
- **WHEN** the extended market-list response contains market records
- **THEN** the system returns market ID, category, name when decodable, and short name when decodable

#### Scenario: Market-list CLI
- **WHEN** the operator runs `marketd exquote-markets --server 127.0.0.1:7727`
- **THEN** the command uses the provided `exhq` server
- **AND** it writes a JSON array of extended market records

### Requirement: Fetch a single TDX extended instrument quote
The system SHALL fetch a single extended instrument quote from a TDX `exhq` server.

#### Scenario: Decode extended quote fields
- **WHEN** a valid extended quote response contains pre-close, open, high, low, current price, volume fields, open-interest style fields, and five bid/ask levels
- **THEN** the returned quote includes those typed fields
- **AND** it preserves the numeric extended market ID and instrument code

#### Scenario: Extended quote CLI
- **WHEN** the operator runs `marketd exquote --market 47 --code IF1709 --server 127.0.0.1:7727`
- **THEN** the command uses the provided `exhq` server
- **AND** it writes one extended quote object as JSON

#### Scenario: Invalid extended quote identity
- **WHEN** the operator runs `marketd exquote` with a non-positive market ID or empty code
- **THEN** the system rejects the request with a validation error
- **AND** it does not connect to any TDX server

### Requirement: Keep extended quote retrieval read-only
The system SHALL NOT write TDX extended quote results to ClickHouse as part of this capability.

#### Scenario: Extended quote commands do not open ClickHouse
- **WHEN** the operator runs `marketd exquote` or `marketd exquote-markets`
- **THEN** the command connects only to TDX `exhq` servers
- **AND** it does not require ClickHouse config
- **AND** it does not write to any canonical market fact table

### Requirement: Document current extended行情 limits
The system SHALL document the implemented `exhq` scope and current exclusions.

#### Scenario: Documentation updated
- **WHEN** extended quote support is implemented
- **THEN** the documentation states that market list and single quote are implemented
- **AND** it states that extended K-line, minute-time, transaction, history, instrument-list, and persistence are not implemented
