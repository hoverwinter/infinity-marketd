# TDX Advanced Online Protocol Notes

Implementation reference for `add-tdx-advanced-online-apis`, captured from
[`millken/tdx`](https://github.com/millken/tdx) (reverse-engineered) and adapted
to marketd's existing `QuoteSession` framing. These are **live upstream reads**;
none of them write ClickHouse.

## Framing equivalence (important)

millken's `BuildDirectFrame` is byte-for-byte the same as marketd's existing
per-command HQ packets:

```
[0]    0x0c                      magic
[1:5]  msgID   (uint32 LE)       per-command id (server-tolerant)
[5]    control (0x01)
[6:8]  len(body)+2 (uint16 LE)
[8:10] len(body)+2 (uint16 LE)
[10:12] frameType / cmd (uint16 LE)
[12:]  body
```

marketd's `QuoteSession.call` already handles the 16-byte response header
(zip/raw length at `[12:14]`/`[14:16]`) + zlib, so millken decoders port directly
onto the body it returns. marketd's `readTDXVarInt` == millken's `cutPrice`
(signed get_price varint: low 6 bits + 0x40 sign + 0x80 continuation, 7 bits each).
Helper: `buildHQDirectFrame(msgID, frameType, body)` in `internal/tdx/hq_advanced.go`.

## Command ids (millken DirectFrameType)

| cmd | feature | status in marketd |
|-----|---------|-------------------|
| 0x054B | quotes list (sorted scanner) | ✅ `hq_advanced.go` |
| 0x053F | top board (9 ranking groups) | ✅ `hq_advanced.go` |
| 0x122B | MAC batch symbol quotes | ✅ `mac_quote.go` |
| 0x122C | SP/MAC board members | ✅ `sp_client.go` |
| F10 0x02CF/0x02D0 | LHB (parsed from company content) | ✅ `hq_lhb.go` |
| 7727 fund | fund kline / fund detail | ✅ `fund_client.go` |

## 0x054B quotes list

Request body (18 bytes, LE uint16 each): category, sortType, start, count,
sortReverse, `5`, excludeMask, `1`, `0`. sortReverse: code-sort→0, else
desc→1 / asc→2. excludeMask bits exclude stock types (FilterBJ returns empty
from server).

BJ securities master/provider uses `category=12` (`北证A`) with `sortType=0`
code sort and `count=80`. Stop paging when the returned page has fewer than 80
rows and still filter decoded rows to `market=2`; public servers can return
non-BJ rows if paging continues past the BJ range.

Response: header 4 bytes (count at `[2:4]`). Per row: market(1) + code(6) +
active(2), then 9 signed varints (price, pre_close, open, high, low,
server_time, neg_price, vol, cur_vol), amount(float32), 8 skipped varints, then
a 56-byte fixed tail (rise_speed = int16 at tail offset +2, /100). Prices are 厘
(/100); OHLC/pre_close are deltas added to price before scaling.

## 0x122B MAC batch symbol quotes (`mac_quote.go`)

MAC HQ uses a `0x01` direct frame with a 10-byte header:

```text
[0]    0x01
[1:5]  customize = 0
[5]    packet type = 0x01
[6:8]  len(method+body)
[8:10] len(method+body)
[10:12] method = 0x122B
[12:]  bitmap(20) + count(2) + count * (market(2) + code(22))
```

Unlike `sp_client.go`, this path does not send SP login. It connects to MAC HQ
servers such as `121.36.248.138:7709` and reuses the normal 16-byte response
header + optional zlib body decoder.

Response layout starts with bitmap echo(20), total(4), count(2), then rows of
market(2)+code(22 GBK/GB18030)+name(44 GBK/GB18030)+4 bytes per active bitmap
field. `marketd` decodes only identity fields for securities master: market,
symbol, and name. BJ import batches these requests by 80 symbols.

## 0x053F top board

Request body (10 bytes): category(1), `5`, marker bytes, size(1) at `[9]`.
Response: size(1) byte, then 9 lists × size entries; each entry market(1) +
code(6) + price(float32) + value(float32) = 15 bytes. Groups in order: gainers,
losers, amplitude, rise_speed, fall_speed, volume_ratio, commission_ratio_pos,
commission_ratio_neg, turnover.

## 0x122C SP board members (`sp_client.go`)

Distinct connection model. Open sequence (`OpenSPSession`): ping `0x0015` →
connect-auth `0x1894/0x000D` → stage2 `0x1899/0x0FDB` (these reply as 16-byte
`0xB1CB7400` frames) → SP login `0x2454` (80-byte encrypted blob, head=0x01
frame). Request: `BuildSPFrame(head=0x01, msgID=0x122C, body)` where body =
boardCode(4)+pad to 13, sortType(2)+start(4)+pageSize(1)+0+order(1)+0, then the
20-byte field bitmap. Response is a `0x01` SP frame: 20-byte bitmap echo +
total(4) + row_count(2), then rows of market(2)+code(22 GBK)+name(44 GBK) + 4
bytes per active bitmap field (kind per `boardMembersFieldFmt`). Auto-paginate by
80. No public SP server defaults → explicit `--server` required.

## LHB (`hq_lhb.go`)

Not a binary packet. Locate the F10 `资金动向` category (via existing
`CompanyInfoCategories`), fetch its content (`CompanyInfoContent`), trim to
`【1.交易龙虎榜】`, and regex-parse records (date/info-type, summary line,
买入前五/卖出前五 seat tables). Missing section returns an empty result with a
message, not an error.

## Fund 7727 (`fund_client.go`)

SP framing but a different open: SP login `0x2454` then fund bootstrap `0x23F0`
(no ping/auth/stage2). Fund detail `0x2488`: body category(1)+code(23)+mode@28(2);
response count@36(2) then 16-byte rows (id u32 + 6×u16, semantics unmapped).
Fund kline `0x2489`: body category(1)+code(23)+period(2)+times(2)+start(4)+count(4);
response period@24(2)+count@40(2) then 32-byte records (time(4) + float32
O/H/L/C/amount + u32 volume). Beijing codes rejected. No public fund defaults.

## Validation note

All decoders are covered by round-trip / `net.Pipe` tests against the documented
layouts. There is **no live public-server validation** for these advanced reads;
correctness rests on faithful porting from millken/tdx plus fixture tests.

## 0x054C compact batch quotes (`hq_compat.go`)

Request body: fixed 10-byte header with mode `5`, count at `[8:10]`, followed by
`count * (market byte + 6-byte code)`. It uses standard HQ direct-frame command
`0x054C`.

Response layout matches the quote-list row prefix: market, code, active, varint
price fields, server time, volume/current volume, float32 amount, inside/outside
volume, best bid/ask, and a 56-byte tail. The tail currently exposes rise speed,
short turnover, 2-minute amount, volume ratio, and depth. This endpoint is useful
when callers need scanner-style compact rows rather than full five-level quote
snapshots.

## 0x0537 tick chart (`hq_compat.go`)

Request body: market(2), code(6), start(2), count(2). Response body starts with
count and then varint triples: price, average price, volume. Price is scaled by
`/100`, average price by `/10000`. Point labels use the A-share trading minute
sequence from `09:30`, switching to `13:00` after the morning session. Requests
must keep `start + count <= 240`.

This is deliberately separate from `/api/tdx/hq/minute`, which remains the
existing minute-time point API.

## SP/fund server discovery (`server_discovery.go`)

SP and fund candidates use separate built-in lists because their handshakes are
not standard HQ probes:

- SP probe opens an SP session: ping -> connect-auth -> stage2 -> SP login.
- Fund probe opens a fund 7727 session: SP login -> fund bootstrap.

Explicit `server` overrides still win for reads. Probe/best-server helpers are
operator conveniences, not a guarantee that every later business request will be
accepted by that upstream.

## Fund detail labels

`fund-detail` keeps the raw `id + [6]uint16` rows and additionally emits decoded
items for a small dictionary of confirmed ids. Unknown ids are preserved raw and
are not assigned fabricated semantics.

## Online adjusted bars

`hq-adjusted-bars-online` and `/api/tdx/hq/adjusted-bars` fetch live HQ bars and
live XDXR rows, compute qfq/hfq factors in memory, and return adjusted OHLC for
that response only. For adjusted modes, the helper pages daily HQ bars from the
latest page toward history until it covers the earliest requested raw bar date,
so older windows either get a matching factor or an explicit insufficient-history
error. They do not write ClickHouse and do not replace
`/api/v1/bars?adjust=...`, which stays backed by persisted raw bars and persisted
adjustment factors.
