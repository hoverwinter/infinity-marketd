## ADDED Requirements

### Requirement: Fetch TDX extended instrument catalog
The system SHALL fetch ExHQ instrument count and instrument list data without using ClickHouse.

#### Scenario: Fetch instrument count
- **WHEN** the operator runs `marketd exquote-count --server 127.0.0.1:7727`
- **THEN** the command uses the provided ExHQ server
- **AND** it writes the instrument count as JSON

#### Scenario: Fetch instrument list
- **WHEN** the operator runs `marketd exquote-instruments --start 0 --count 100 --server 127.0.0.1:7727`
- **THEN** the command uses the provided ExHQ server
- **AND** it writes a JSON array containing category, numeric market ID, code, name when decodable, and description when decodable

### Requirement: Fetch TDX extended K-line data
The system SHALL fetch ExHQ K-line data by TDX category, numeric market ID, code, start offset, and count.

#### Scenario: Fetch K-line data
- **WHEN** the operator runs `marketd exquote-bars --market 47 --code ICL0 --category 4 --start 0 --count 100`
- **THEN** the command requests ExHQ K-line data
- **AND** it writes typed OHLC, position, trade, price, timestamp, market, code, and category fields as JSON

#### Scenario: Reject invalid K-line window
- **WHEN** the operator provides a negative start or a count greater than the supported K-line maximum
- **THEN** the command rejects the request before connecting to an ExHQ server

### Requirement: Fetch TDX extended minute-time data
The system SHALL fetch ExHQ current-day and historical minute-time data.

#### Scenario: Fetch current minute-time data
- **WHEN** the operator runs `marketd exquote-minute --market 47 --code ICL0`
- **THEN** the command requests current ExHQ minute-time data
- **AND** it writes price, average price, volume, open interest, time, market, and code fields as JSON

#### Scenario: Fetch historical minute-time data
- **WHEN** the operator runs `marketd exquote-history-minute --market 47 --code ICL0 --date 20260605`
- **THEN** the command requests ExHQ historical minute-time data for that `YYYYMMDD` date
- **AND** it includes date and datetime fields in JSON results

### Requirement: Fetch TDX extended transaction data
The system SHALL fetch ExHQ current-day and historical transaction data.

#### Scenario: Fetch current transactions
- **WHEN** the operator runs `marketd exquote-transactions --market 47 --code ICL0 --start 0 --count 1800`
- **THEN** the command requests ExHQ transaction data
- **AND** it writes price, volume, zengcang, nature, nature name, direction, time, market, and code fields as JSON

#### Scenario: Fetch historical transactions
- **WHEN** the operator runs `marketd exquote-history-transactions --market 47 --code ICL0 --date 20260605 --start 0 --count 1800`
- **THEN** the command requests ExHQ historical transaction data for that `YYYYMMDD` date
- **AND** it includes date and datetime fields in JSON results

### Requirement: Fetch TDX extended historical K-line ranges
The system SHALL fetch ExHQ historical K-line range data by numeric market ID, code, start date, and end date.

#### Scenario: Fetch historical K-line range
- **WHEN** the operator runs `marketd exquote-history-bars --market 74 --code BABA --start-date 20260601 --end-date 20260605`
- **THEN** the command requests the ExHQ historical K-line date range
- **AND** it writes typed K-line rows as JSON

#### Scenario: Reject invalid date range
- **WHEN** the operator provides a start date after the end date
- **THEN** the command rejects the request before connecting to an ExHQ server

### Requirement: Keep expanded ExHQ reads non-persistent
The system SHALL NOT write ExHQ catalog, K-line, minute-time, transaction, or historical results to ClickHouse in this capability.

#### Scenario: Expanded ExHQ commands do not open ClickHouse
- **WHEN** the operator runs any `exquote-*` command
- **THEN** the command connects only to ExHQ servers
- **AND** it does not require ClickHouse config
- **AND** it does not write to any market fact table
