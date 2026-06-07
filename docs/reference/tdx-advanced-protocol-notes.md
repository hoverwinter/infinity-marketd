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
| 0x122C | SP/MAC board members | ⏳ pending |
| F10 0x02CF/0x02D0 | LHB (parsed from company content) | ⏳ pending |
| 7727 fund | fund kline / fund detail | ⏳ pending |

## 0x054B quotes list

Request body (18 bytes, LE uint16 each): category, sortType, start, count,
sortReverse, `5`, excludeMask, `1`, `0`. sortReverse: code-sort→0, else
desc→1 / asc→2. excludeMask bits exclude stock types (FilterBJ returns empty
from server).

Response: header 4 bytes (count at `[2:4]`). Per row: market(1) + code(6) +
active(2), then 9 signed varints (price, pre_close, open, high, low,
server_time, neg_price, vol, cur_vol), amount(float32), 8 skipped varints, then
a 56-byte fixed tail (rise_speed = int16 at tail offset +2, /100). Prices are 厘
(/100); OHLC/pre_close are deltas added to price before scaling.

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
