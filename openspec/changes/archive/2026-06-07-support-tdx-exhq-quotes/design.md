## Research Summary

Primary references:

- pytdx extended行情 docs describe `TdxExHq_API`, port `7727`, `get_markets()`, and `get_instrument_quote(market, code)`.
- pytdx `ex_setup_commands.py` defines a single extended行情 setup packet.
- pytdx `ex_get_markets.py` defines the market-list request and 64-byte market records.
- pytdx `ex_get_instrument_quote.py` defines the single quote request and 136-byte quote field layout after the response identity header.

The important first-principles distinction is that `exhq` is not an A-share market extension. It is a different TDX protocol family.

## Goals

- Fetch extended market metadata from an `exhq` server.
- Fetch one extended instrument quote by numeric extended market ID and instrument code.
- Keep the implementation independent from standard `hq` quote validation and packet building.
- Return typed JSON from CLI commands.
- Keep all behavior read-only and outside ClickHouse.

## Non-Goals

- No `exhq` server probing workflow beyond explicit `--server` fallback behavior.
- No extended instrument discovery list in this change.
- No extended K-line, minute-time, transaction, or historical transaction support.
- No persistence or table schema.

## Protocol Shape

### Setup

`exhq` uses one setup packet before requests. It is separate from the three standard `hq` setup packets.

### Market List

Request packet:

```text
01 02 48 69 00 01 02 00 02 00 f4 23
```

Response body:

```text
uint16 count
repeated 64-byte records:
  uint8 category
  char[32] name, GBK in pytdx
  uint8 market
  char[2] short_name, GBK in pytdx
  byte[26] reserved
  byte[2] unknown
```

The Go MVP decodes names as UTF-8 only when valid and otherwise leaves them empty. GBK decoding can be added later without changing market IDs.

### Instrument Quote

Request prefix:

```text
01 01 08 02 02 01 0c 00 0c 00 fa 23
```

Request suffix:

```text
uint8 market
char[9] code
```

Response body:

```text
uint8 market
char[9] code
byte[4] ignored
136-byte quote fields:
  float32 pre_close, open, high, low, price
  uint32 kaicang, ignored, zongliang, xianliang, ignored, neipan, waipan, ignored, chicang
  float32 bid1..bid5
  uint32 bid_vol1..bid_vol5
  float32 ask1..ask5
  uint32 ask_vol1..ask_vol5
```

## CLI Contract

```bash
go run ./cmd/marketd exquote-markets --server 61.152.107.141:7727
go run ./cmd/marketd exquote --market 47 --code IF1709 --server 61.152.107.141:7727
```

Both commands emit JSON and do not require config or ClickHouse.

## Risks / Trade-offs

- Public `exhq` servers are not guaranteed reachable. Operators can repeat `--server`; the client should try candidates in order.
- Some market names are GBK. Leaving non-UTF-8 names empty is acceptable for this MVP because market ID and category are the operational keys.
- Extended instrument taxonomy is broad. This change intentionally does not normalize instruments into canonical storage.
