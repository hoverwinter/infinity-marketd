## ADDED Requirements

### Requirement: Compact batch quote rows
`marketd` SHALL support standard HQ compact batch quote reads equivalent to the TDX `0x054C` request.

#### Scenario: Fetch compact batch quotes
- **WHEN** an operator requests compact batch quotes for one or more supported A-share symbols
- **THEN** `marketd` SHALL send the compact batch quote request to the selected HQ server
- **AND** it SHALL return structured rows with market, symbol, price, open, high, low, pre-close, volume, current volume, amount, inside/outside volume, rise speed, volume ratio, turnover-related fields when decoded, and raw protocol metadata
- **AND** it SHALL NOT write ClickHouse or ops tables as a hidden side effect

#### Scenario: Validate compact batch quote request
- **WHEN** the request contains no symbols, unsupported markets, invalid symbols, or more than the configured maximum symbol count
- **THEN** `marketd` SHALL reject the request before contacting a TDX server

### Requirement: Tick-chart reads
`marketd` SHALL support standard HQ tick-chart reads equivalent to `millken/tdx.GetTickChart`.

#### Scenario: Fetch tick chart
- **WHEN** an operator requests a tick chart by market, symbol, start, and count
- **THEN** `marketd` SHALL request the corresponding live HQ tick-chart packet
- **AND** it SHALL return ordered points with market, symbol, point index, time when available, price, volume, amount or average-price fields when decoded, and raw protocol metadata

#### Scenario: Keep tick chart separate from minute-time
- **WHEN** callers use existing `/api/tdx/hq/minute` or `hq-minute`
- **THEN** those APIs SHALL continue to return TDX minute-time price/volume points
- **AND** they SHALL NOT silently switch to the tick-chart packet

#### Scenario: Validate tick-chart page
- **WHEN** the caller provides a negative start, non-positive count, count above the supported packet limit, or a start/count window beyond the supported trading-minute range
- **THEN** `marketd` SHALL reject the request before contacting a TDX server

### Requirement: SP and fund server discovery
`marketd` SHALL support address catalogs, probe results, and best-server selection for SP and fund 7727 online reads.

#### Scenario: List SP and fund candidate servers
- **WHEN** an operator requests SP or fund server candidates
- **THEN** `marketd` SHALL return the configured or built-in candidate addresses with protocol labels and no connection side effect

#### Scenario: Probe SP servers
- **WHEN** an operator probes SP servers
- **THEN** `marketd` SHALL perform the SP handshake used by board-member reads
- **AND** it SHALL return per-server success, latency, errors, and the preferred reachable server

#### Scenario: Probe fund servers
- **WHEN** an operator probes fund servers
- **THEN** `marketd` SHALL perform the fund 7727 login/bootstrap used by fund K-line and fund detail reads
- **AND** it SHALL return per-server success, latency, errors, and the preferred reachable server

#### Scenario: Use explicit server override
- **WHEN** an SP or fund read includes an explicit server
- **THEN** `marketd` SHALL try the explicit server according to the request order
- **AND** it SHALL NOT silently replace it with a best-server candidate

#### Scenario: Use best server when requested
- **WHEN** an SP or fund read enables best-server selection without explicit servers
- **THEN** `marketd` SHALL probe or load the relevant cache and use the preferred reachable server for that protocol

### Requirement: Labeled fund detail
`marketd` SHALL decode known fund detail item ids into labeled fields while preserving raw fund detail rows.

#### Scenario: Decode known fund detail items
- **WHEN** a fund detail response contains item ids with confirmed labels
- **THEN** `marketd` SHALL include decoded label, unit, typed value when derivable, and raw value array for each known item

#### Scenario: Preserve unknown fund detail items
- **WHEN** a fund detail response contains an item id without a confirmed label
- **THEN** `marketd` SHALL preserve the raw id and raw value array
- **AND** it SHALL NOT fabricate a semantic label

#### Scenario: Keep fund detail raw compatibility
- **WHEN** callers consume the existing raw fund detail shape
- **THEN** `marketd` SHALL keep raw item ids and value arrays available in the response

### Requirement: One-shot online adjusted bars
`marketd` SHALL provide a live provider convenience API for one-shot adjusted standard HQ bars.

#### Scenario: Fetch online adjusted HQ bars
- **WHEN** an operator requests online HQ bars with `adjust=none`, `adjust=qfq`, or `adjust=hfq`
- **THEN** `marketd` SHALL fetch raw HQ bars from the selected live HQ server
- **AND** for adjusted modes it SHALL fetch live HQ XDXR rows for the same symbol
- **AND** it SHALL return bars with adjusted OHLC values for `qfq` or `hfq` and raw volume/amount
- **AND** it SHALL include response metadata indicating that the data came from live provider reads

#### Scenario: Reject unsupported online adjustment
- **WHEN** the caller provides an unsupported adjustment mode
- **THEN** `marketd` SHALL reject the request before contacting a TDX server

#### Scenario: Missing online adjustment inputs
- **WHEN** live raw bars or live XDXR data are insufficient to compute a requested adjusted response
- **THEN** `marketd` SHALL return an explicit missing-factor or insufficient-history error
- **AND** it SHALL NOT silently mix raw and adjusted OHLC values in one successful adjusted response

#### Scenario: Online adjusted bars are not persisted
- **WHEN** one-shot online adjusted bars succeed
- **THEN** `marketd` SHALL NOT write raw bars, XDXR events, adjustment factors, market facts, derived rows, task runs, or watermarks

### Requirement: CLI and provider exposure for compatibility gaps
`marketd` SHALL expose compact batch quotes, tick chart, SP/fund discovery, labeled fund detail, and one-shot online adjusted bars through CLI and TDX provider endpoints.

#### Scenario: CLI commands are discoverable
- **WHEN** an operator runs `marketd help`
- **THEN** commands for compact batch quotes, tick chart, SP/fund server discovery, labeled fund detail, and online adjusted bars SHALL be listed

#### Scenario: Provider endpoints are exposed
- **WHEN** `infinity querier serve` is running
- **THEN** `/api/tdx/*` endpoints for compact batch quotes, tick chart, SP/fund server discovery, labeled fund detail, and online adjusted bars SHALL be available

#### Scenario: Provider endpoints do not affect product queries
- **WHEN** the compatibility provider endpoints are called
- **THEN** `/api/v1` product query behavior SHALL remain unchanged
