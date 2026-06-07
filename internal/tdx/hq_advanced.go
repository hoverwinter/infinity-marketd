package tdx

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// Advanced TDX standard 行情 (hq) online reads ported from millken/tdx:
// sorted quote lists (0x054B) and top-board rankings (0x053F). These are live
// upstream reads and never write ClickHouse. Protocol framing matches the
// existing QuoteSession (12-byte 0x0c direct frame + 16-byte response header).

// Quotes-list direct frame command id.
const cmdQuotesList uint16 = 0x054B

// Quotes-list named sort keys (raw protocol ids preserved for advanced callers).
const (
	QuotesSortCode       uint16 = 0x00
	QuotesSortPrice      uint16 = 0x06
	QuotesSortVolume     uint16 = 0x09
	QuotesSortAmount     uint16 = 0x0A
	QuotesSortChangePct  uint16 = 0x0E
	QuotesSortAmplitude  uint16 = 0x0F
	QuotesSortVolRatio   uint16 = 0x23
	QuotesSortTurnover   uint16 = 0x24
	QuotesSortSpeed      uint16 = 0x2E
	QuotesSortMainNetAmt uint16 = 0xD4
)

// Quotes-list exclude bitmask flags (set to EXCLUDE that stock type).
const (
	QuotesFilterNew uint16 = 1 << 0
	QuotesFilterKCB uint16 = 1 << 1
	QuotesFilterST  uint16 = 1 << 2
	QuotesFilterCYB uint16 = 1 << 3
	QuotesFilterBJ  uint16 = 1 << 4
)

const maxQuotesListCount = 1000

// HQQuotesListRequest is a sorted market quote-list request.
type HQQuotesListRequest struct {
	Category uint16
	SortType uint16
	Start    int
	Count    int
	Reverse  bool
	Exclude  uint16
}

// HQQuotesListItem is one row of a sorted quote list. Raw protocol ids
// (MarketCode, Active, EffectiveExclude) are preserved alongside decoded values.
type HQQuotesListItem struct {
	Market        string  `json:"market"`
	Symbol        string  `json:"symbol"`
	Price         float64 `json:"price"`
	Open          float64 `json:"open"`
	High          float64 `json:"high"`
	Low           float64 `json:"low"`
	PreClose      float64 `json:"pre_close"`
	Volume        int64   `json:"volume"`
	CurrentVolume int64   `json:"current_volume"`
	Amount        float64 `json:"amount"`
	RiseSpeed     float64 `json:"rise_speed"`
	Active        uint16  `json:"active"`
	MarketCode    uint16  `json:"market_code"`
}

func (r HQQuotesListRequest) validate() error {
	if r.Start < 0 {
		return fmt.Errorf("quotes list start must be >= 0")
	}
	if r.Count <= 0 {
		return fmt.Errorf("quotes list count must be > 0")
	}
	if r.Count > maxQuotesListCount {
		return fmt.Errorf("quotes list count %d exceeds max %d", r.Count, maxQuotesListCount)
	}
	return nil
}

// sortReverse derives the protocol order field: code sort uses none, otherwise
// descending unless the caller asks for ascending.
func (r HQQuotesListRequest) sortReverse() uint16 {
	if r.SortType == QuotesSortCode {
		return 0
	}
	if r.Reverse {
		return 2
	}
	return 1
}

// BuildHQQuotesListPacket builds the 0x054B quotes-list request frame.
func BuildHQQuotesListPacket(req HQQuotesListRequest) []byte {
	body := make([]byte, 18)
	binary.LittleEndian.PutUint16(body[0:2], req.Category)
	binary.LittleEndian.PutUint16(body[2:4], req.SortType)
	binary.LittleEndian.PutUint16(body[4:6], uint16(req.Start))
	binary.LittleEndian.PutUint16(body[6:8], uint16(req.Count))
	binary.LittleEndian.PutUint16(body[8:10], req.sortReverse())
	binary.LittleEndian.PutUint16(body[10:12], 5)
	binary.LittleEndian.PutUint16(body[12:14], req.Exclude)
	binary.LittleEndian.PutUint16(body[14:16], 1)
	binary.LittleEndian.PutUint16(body[16:18], 0)
	return buildHQDirectFrame(0x000A0401, cmdQuotesList, body)
}

// DecodeHQQuotesListResponse decodes a 0x054B quotes-list response body.
// Layout per row: market(1) + code(6) + active(2), then 9 signed varints
// (price, pre_close, open, high, low, server_time, neg_price, vol, cur_vol),
// amount(float32), 8 skipped varints, and a 56-byte fixed tail whose int16 at
// offset +2 is rise_speed (%). Prices are 厘; OHLC fields are deltas from price.
func DecodeHQQuotesListResponse(body []byte) ([]HQQuotesListItem, error) {
	if len(body) < 4 {
		return nil, fmt.Errorf("decode TDX HQ quotes list: body too short: %d", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[2:4]))
	pos := 4
	items := make([]HQQuotesListItem, 0, count)
	for i := 0; i < count; i++ {
		if pos+9 > len(body) {
			break
		}
		marketCode := uint16(body[pos])
		code := strings.TrimRight(string(body[pos+1:pos+7]), "\x00")
		active := binary.LittleEndian.Uint16(body[pos+7 : pos+9])
		pos += 9

		var price, preClose, open, high, low, vol, curVol int
		var err error
		read := func(field string) int {
			if err != nil {
				return 0
			}
			var v int
			v, pos, err = readTDXVarInt(body, pos)
			if err != nil {
				err = fmt.Errorf("decode TDX HQ quotes list record %d %s: %w", i, field, err)
			}
			return v
		}
		price = read("price")
		preClose = read("pre_close")
		open = read("open")
		high = read("high")
		low = read("low")
		read("server_time")
		read("neg_price")
		vol = read("vol")
		curVol = read("cur_vol")
		if err != nil {
			return nil, err
		}
		if pos+4 > len(body) {
			return nil, fmt.Errorf("decode TDX HQ quotes list record %d amount: unexpected EOF", i)
		}
		amount := math.Float32frombits(binary.LittleEndian.Uint32(body[pos : pos+4]))
		pos += 4
		for j := 0; j < 8; j++ { // in/out/s_amount/open_amount + bid/ask/bid_vol/ask_vol
			read("skip")
		}
		if err != nil {
			return nil, err
		}
		if pos+56 > len(body) {
			return nil, fmt.Errorf("decode TDX HQ quotes list record %d tail: unexpected EOF", i)
		}
		riseSpeed := float64(int16(binary.LittleEndian.Uint16(body[pos+2:pos+4]))) / 100
		pos += 56

		items = append(items, HQQuotesListItem{
			Market:        quotesListMarketName(marketCode),
			Symbol:        code,
			MarketCode:    marketCode,
			Active:        active,
			Price:         float64(price) / 100,
			Open:          float64(open+price) / 100,
			High:          float64(high+price) / 100,
			Low:           float64(low+price) / 100,
			PreClose:      float64(preClose+price) / 100,
			Volume:        int64(vol),
			CurrentVolume: int64(curVol),
			Amount:        float64(amount),
			RiseSpeed:     riseSpeed,
		})
	}
	return items, nil
}

// QuotesList fetches a sorted quote list over an open session.
func (s *QuoteSession) QuotesList(req HQQuotesListRequest) ([]HQQuotesListItem, error) {
	body, err := s.call(BuildHQQuotesListPacket(req))
	if err != nil {
		return nil, fmt.Errorf("TDX HQ quotes list %s: %w", s.server, err)
	}
	return DecodeHQQuotesListResponse(body)
}

// FetchHQQuotesList fetches a sorted quote list with server fallback.
func FetchHQQuotesList(ctx context.Context, req HQQuotesListRequest, opts QuoteClientOptions) ([]HQQuotesListItem, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}
	return fetchHQRead(ctx, opts, "quotes list", func(s *QuoteSession) ([]HQQuotesListItem, error) {
		return s.QuotesList(req)
	})
}

// Top-board direct frame command id.
const cmdTopBoard uint16 = 0x053F

const maxTopBoardSize = 100

// HQTopBoardItem is one ranking-list entry.
type HQTopBoardItem struct {
	Market     string  `json:"market"`
	Symbol     string  `json:"symbol"`
	MarketCode uint16  `json:"market_code"`
	Price      float64 `json:"price"`
	Value      float64 `json:"value"`
}

// HQTopBoardGroup pairs a named ranking group with its raw protocol order id.
type HQTopBoardGroup struct {
	Name    string           `json:"name"`
	GroupID int              `json:"group_id"`
	Items   []HQTopBoardItem `json:"items"`
}

// topBoardGroupNames lists the 9 ranking groups in protocol order.
var topBoardGroupNames = []string{
	"gainers", "losers", "amplitude", "rise_speed", "fall_speed",
	"volume_ratio", "commission_ratio_pos", "commission_ratio_neg", "turnover",
}

// BuildHQTopBoardPacket builds the 0x053F top-board request frame.
func BuildHQTopBoardPacket(category uint16, size int) []byte {
	body := make([]byte, 10)
	body[0] = byte(category)
	body[1] = 5
	// body[2:9] fixed marker bytes; body[9] = size
	body[6] = 0x01
	body[9] = byte(size)
	return buildHQDirectFrame(0x000A0401, cmdTopBoard, body)
}

// DecodeHQTopBoardResponse decodes a 0x053F response: 1 size byte, then 9 lists
// of `size` entries; each entry is market(1)+code(6)+price(f32)+value(f32).
func DecodeHQTopBoardResponse(body []byte) ([]HQTopBoardGroup, error) {
	if len(body) < 1 {
		return nil, fmt.Errorf("decode TDX HQ top board: body too short: %d", len(body))
	}
	size := int(body[0])
	pos := 1
	const entrySize = 15
	groups := make([]HQTopBoardGroup, 0, len(topBoardGroupNames))
	for g, name := range topBoardGroupNames {
		items := make([]HQTopBoardItem, 0, size)
		for i := 0; i < size; i++ {
			if pos+entrySize > len(body) {
				break
			}
			marketCode := uint16(body[pos])
			code := strings.TrimRight(string(body[pos+1:pos+7]), "\x00")
			price := math.Float32frombits(binary.LittleEndian.Uint32(body[pos+7 : pos+11]))
			value := math.Float32frombits(binary.LittleEndian.Uint32(body[pos+11 : pos+15]))
			pos += entrySize
			items = append(items, HQTopBoardItem{
				Market:     quotesListMarketName(marketCode),
				Symbol:     code,
				MarketCode: marketCode,
				Price:      float64(price),
				Value:      float64(value),
			})
		}
		groups = append(groups, HQTopBoardGroup{Name: name, GroupID: g, Items: items})
	}
	return groups, nil
}

// TopBoard fetches the 9 ranking groups over an open session.
func (s *QuoteSession) TopBoard(category uint16, size int) ([]HQTopBoardGroup, error) {
	body, err := s.call(BuildHQTopBoardPacket(category, size))
	if err != nil {
		return nil, fmt.Errorf("TDX HQ top board %s: %w", s.server, err)
	}
	return DecodeHQTopBoardResponse(body)
}

// FetchHQTopBoard fetches the top-board rankings with server fallback.
func FetchHQTopBoard(ctx context.Context, category uint16, size int, opts QuoteClientOptions) ([]HQTopBoardGroup, error) {
	if size <= 0 || size > maxTopBoardSize {
		return nil, fmt.Errorf("top board size must be between 1 and %d", maxTopBoardSize)
	}
	return fetchHQRead(ctx, opts, "top board", func(s *QuoteSession) ([]HQTopBoardGroup, error) {
		return s.TopBoard(category, size)
	})
}

// buildHQDirectFrame builds a 12-byte 0x0c direct-frame header + body, matching
// the existing per-command packets and millken/tdx's BuildDirectFrame.
func buildHQDirectFrame(msgID uint32, frameType uint16, body []byte) []byte {
	packet := make([]byte, 12+len(body))
	packet[0] = 0x0c
	binary.LittleEndian.PutUint32(packet[1:5], msgID)
	packet[5] = 0x01
	binary.LittleEndian.PutUint16(packet[6:8], uint16(len(body)+2))
	binary.LittleEndian.PutUint16(packet[8:10], uint16(len(body)+2))
	binary.LittleEndian.PutUint16(packet[10:12], frameType)
	copy(packet[12:], body)
	return packet
}

// quotesListMarketName maps the raw market byte (0=sz,1=sh,2=bj) to a label.
func quotesListMarketName(code uint16) string {
	switch code {
	case 0:
		return "sz"
	case 1:
		return "sh"
	case 2:
		return "bj"
	default:
		return fmt.Sprintf("%d", code)
	}
}
