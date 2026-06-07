## ADDED Requirements

### Requirement: Standard HQ server operations
The system SHALL provide read-only TDX standard行情 server operations for probing, session setup, retries, and security discovery.

#### Scenario: Probe standard HQ servers
- **WHEN** the operator runs a standard HQ server probe command with one or more servers
- **THEN** the system attempts standard HQ connection setup for each server
- **AND** it returns success, latency, errors, and the preferred reachable server as JSON

#### Scenario: Fetch security count and list
- **WHEN** the operator requests standard HQ security count or security list data for a supported market
- **THEN** the system requests the data from the provided standard HQ server
- **AND** it returns market, symbol, decoded name when available, volume unit, decimal point, and previous close fields as JSON

#### Scenario: Reject unsupported security discovery market
- **WHEN** the operator requests security discovery for a market whose standard HQ count/list behavior is not verified
- **THEN** the system rejects the request before returning discovered securities for that market

### Requirement: Realtime quote snapshots
The system SHALL fetch standard HQ realtime quote snapshots for supported A-share markets.

#### Scenario: Fetch realtime quote snapshots
- **WHEN** the operator requests one or more supported `sh`, `sz`, or verified `bj` symbols
- **THEN** the system requests standard HQ realtime quotes
- **AND** it returns market, symbol, price, last close, open, high, low, volume, current volume, amount, buy volume, sell volume, server intraday time, and bid/ask levels as JSON

#### Scenario: Validate realtime quote identity
- **WHEN** a standard HQ quote response returns a market or symbol that does not match the request
- **THEN** the system rejects the response as a protocol mismatch

### Requirement: Standard HQ K-line data
The system SHALL fetch standard HQ stock and index K-line data by market, symbol, category, start offset, and count.

#### Scenario: Fetch security K-line data
- **WHEN** the operator requests standard HQ security K-line data with category, market, symbol, start, and count
- **THEN** the system requests `get_security_bars` equivalent data
- **AND** it returns timestamp, open, high, low, close, volume, amount, market, symbol, and category fields as JSON

#### Scenario: Fetch index K-line data
- **WHEN** the operator requests standard HQ index K-line data with category, market, symbol, start, and count
- **THEN** the system requests `get_index_bars` equivalent data
- **AND** it returns timestamp, open, high, low, close, volume, amount, market, symbol, and category fields as JSON

#### Scenario: Reject invalid K-line page
- **WHEN** the operator provides a negative start, unsupported category, or count outside the supported K-line range
- **THEN** the system rejects the request before connecting to a standard HQ server

#### Scenario: Respect K-line page limit
- **WHEN** the operator requests more than 800 K-line rows in a single standard HQ K-line request
- **THEN** the system rejects the request and reports that K-line count must be at most 800

### Requirement: Standard HQ minute-time data
The system SHALL fetch current-day and historical standard HQ minute-time data as price and volume points.

#### Scenario: Fetch current-day minute-time data
- **WHEN** the operator requests current-day minute-time data for a supported market and symbol
- **THEN** the system requests `get_minute_time_data` equivalent data
- **AND** it returns market, symbol, time, price, and volume fields as JSON

#### Scenario: Fetch historical minute-time data
- **WHEN** the operator requests historical minute-time data for a supported market, symbol, and `YYYYMMDD` date
- **THEN** the system requests `get_history_minute_time_data` equivalent data for that date
- **AND** it returns date, market, symbol, time, price, and volume fields as JSON

#### Scenario: Preserve empty historical minute-time result
- **WHEN** the requested historical minute-time date has no data on the selected server
- **THEN** the system returns an empty JSON array without treating the response as a decode failure

### Requirement: Standard HQ transaction data
The system SHALL fetch current-day and historical standard HQ transaction data by market, symbol, start offset, and count.

#### Scenario: Fetch current-day transactions
- **WHEN** the operator requests current-day transaction data for a supported market and symbol
- **THEN** the system requests `get_transaction_data` equivalent data
- **AND** it returns transaction time, price, volume, buy/sell direction when decodable, market, and symbol fields as JSON

#### Scenario: Fetch historical transactions
- **WHEN** the operator requests historical transaction data for a supported market, symbol, `YYYYMMDD` date, start, and count
- **THEN** the system requests `get_history_transaction_data` equivalent data
- **AND** it returns date, transaction time, price, volume, buy/sell direction when decodable, market, and symbol fields as JSON

#### Scenario: Reject invalid transaction page
- **WHEN** the operator provides a negative transaction start or unsupported transaction count
- **THEN** the system rejects the request before connecting to a standard HQ server

### Requirement: Standard HQ company information
The system SHALL fetch standard HQ company/F10 category metadata and content.

#### Scenario: Fetch company info categories
- **WHEN** the operator requests company info categories for a supported market and symbol
- **THEN** the system requests `get_company_info_category` equivalent data
- **AND** it returns category name, filename, start, length, market, and symbol fields as JSON

#### Scenario: Fetch company info content
- **WHEN** the operator requests company info content by market, symbol, filename, start, and length
- **THEN** the system requests `get_company_info_content` equivalent data
- **AND** it returns decoded text content as JSON

#### Scenario: Decode company info text
- **WHEN** company info category names or content are GBK/GB18030 encoded
- **THEN** the system decodes them to UTF-8 strings for JSON output

### Requirement: Standard HQ xdxr and finance data
The system SHALL fetch standard HQ xdxr corporate-action data and finance info for supported markets and symbols.

#### Scenario: Fetch xdxr data
- **WHEN** the operator requests xdxr data for a supported market and symbol
- **THEN** the system requests `get_xdxr_info` equivalent data
- **AND** it returns decoded corporate-action rows as JSON

#### Scenario: Fetch finance info
- **WHEN** the operator requests finance info for a supported market and symbol
- **THEN** the system requests `get_finance_info` equivalent data
- **AND** it returns decoded finance fields as JSON

### Requirement: Standard HQ block data
The system SHALL fetch standard HQ block metadata and block membership data.

#### Scenario: Fetch block metadata
- **WHEN** the operator requests standard HQ block metadata
- **THEN** the system requests `get_block_info_meta` equivalent data
- **AND** it returns decoded block names, identifiers, and metadata fields as JSON

#### Scenario: Fetch block membership
- **WHEN** the operator requests standard HQ block membership data by block identifier or metadata entry
- **THEN** the system requests `get_block_info` equivalent data
- **AND** it returns decoded block name, market, symbol, and membership fields as JSON

#### Scenario: Decode block text
- **WHEN** block names or membership text fields are GBK/GB18030 encoded
- **THEN** the system decodes them to UTF-8 strings for JSON output

### Requirement: Keep standard HQ reads non-persistent
The system SHALL NOT write standard HQ online read results to ClickHouse in this capability.

#### Scenario: Standard HQ commands do not open ClickHouse
- **WHEN** the operator runs any standard HQ online read command from this capability
- **THEN** the command connects only to standard HQ servers
- **AND** it does not require ClickHouse config
- **AND** it does not write to any market fact table, derived table, or ops table
