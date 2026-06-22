package tdx

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

const (
	cmdCompactBatchQuote uint16 = 0x054C
	cmdTickChart         uint16 = 0x0537

	MaxCompactBatchQuoteCount = 200
	MaxHQTickChartCount       = 240
)

type HQCompactBatchQuote struct {
	Market        string  `json:"market"`
	Symbol        string  `json:"symbol"`
	MarketCode    uint16  `json:"market_code"`
	Active        uint16  `json:"active"`
	Price         float64 `json:"price"`
	PreClose      float64 `json:"pre_close"`
	Open          float64 `json:"open"`
	High          float64 `json:"high"`
	Low           float64 `json:"low"`
	ServerTime    string  `json:"server_time"`
	Volume        int64   `json:"volume"`
	CurrentVolume int64   `json:"current_volume"`
	Amount        float64 `json:"amount"`
	InsideVolume  int64   `json:"inside_volume"`
	OutsideVolume int64   `json:"outside_volume"`
	BidPrice      float64 `json:"bid_price"`
	BidVolume     int64   `json:"bid_volume"`
	AskPrice      float64 `json:"ask_price"`
	AskVolume     int64   `json:"ask_volume"`
	RiseSpeed     float64 `json:"rise_speed"`
	ShortTurnover float64 `json:"short_turnover"`
	Min2Amount    float64 `json:"min2_amount"`
	VolRatio      float64 `json:"vol_ratio"`
	Depth         float64 `json:"depth"`
}

type HQTickChartRequest struct {
	Market string `json:"market"`
	Symbol string `json:"symbol"`
	Start  int    `json:"start"`
	Count  int    `json:"count"`
}

type HQTickChartPoint struct {
	Market string  `json:"market"`
	Symbol string  `json:"symbol"`
	Index  int     `json:"index"`
	Time   string  `json:"time"`
	Price  float64 `json:"price"`
	Avg    float64 `json:"avg_price"`
	Volume int64   `json:"volume"`
}

func ParseHQTickChartRequest(market, symbol string, start, count int) (HQTickChartRequest, error) {
	market, symbol, err := parseHQMarketSymbol(market, symbol, "tick chart")
	if err != nil {
		return HQTickChartRequest{}, err
	}
	if start < 0 {
		return HQTickChartRequest{}, fmt.Errorf("tick chart start must be non-negative")
	}
	if count <= 0 || count > MaxHQTickChartCount {
		return HQTickChartRequest{}, fmt.Errorf("tick chart count must be between 1 and %d", MaxHQTickChartCount)
	}
	if start+count > MaxHQTickChartCount {
		return HQTickChartRequest{}, fmt.Errorf("tick chart start + count must be <= %d", MaxHQTickChartCount)
	}
	return HQTickChartRequest{Market: market, Symbol: symbol, Start: start, Count: count}, nil
}

func validateCompactBatchQuoteRequests(requests []QuoteRequest) error {
	if len(requests) == 0 {
		return fmt.Errorf("at least one compact batch quote symbol is required")
	}
	if len(requests) > MaxCompactBatchQuoteCount {
		return fmt.Errorf("compact batch quote symbol count must be <= %d", MaxCompactBatchQuoteCount)
	}
	for _, req := range requests {
		value := req.Symbol
		if strings.TrimSpace(req.Market) != "" {
			value = req.Market + ":" + req.Symbol
		}
		if _, err := ParseQuoteRequest(value); err != nil {
			return err
		}
	}
	return nil
}

func BuildHQCompactBatchQuotePacket(requests []QuoteRequest) ([]byte, error) {
	if err := validateCompactBatchQuoteRequests(requests); err != nil {
		return nil, err
	}
	body := make([]byte, 10+len(requests)*7)
	binary.LittleEndian.PutUint16(body[0:2], 5)
	binary.LittleEndian.PutUint16(body[8:10], uint16(len(requests)))
	pos := 10
	for _, req := range requests {
		value := req.Symbol
		if strings.TrimSpace(req.Market) != "" {
			value = req.Market + ":" + req.Symbol
		}
		normalized, _ := ParseQuoteRequest(value)
		body[pos] = byte(quoteMarketCode(normalized.Market))
		copy(body[pos+1:pos+7], normalized.Symbol)
		pos += 7
	}
	return buildHQDirectFrame(0x000A0401, cmdCompactBatchQuote, body), nil
}

func DecodeHQCompactBatchQuoteResponse(body []byte) ([]HQCompactBatchQuote, error) {
	if len(body) < 4 {
		return nil, fmt.Errorf("decode TDX HQ compact batch quotes: body too short: %d", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[2:4]))
	pos := 4
	items := make([]HQCompactBatchQuote, 0, count)
	for i := 0; i < count; i++ {
		if pos+9 > len(body) {
			return nil, fmt.Errorf("decode TDX HQ compact batch quote record %d header: unexpected EOF", i)
		}
		marketCode := uint16(body[pos])
		code := strings.TrimRight(string(body[pos+1:pos+7]), "\x00")
		active := binary.LittleEndian.Uint16(body[pos+7 : pos+9])
		pos += 9

		var err error
		read := func(field string) int {
			if err != nil {
				return 0
			}
			var v int
			v, pos, err = readTDXVarInt(body, pos)
			if err != nil {
				err = fmt.Errorf("decode TDX HQ compact batch quote record %d %s: %w", i, field, err)
			}
			return v
		}
		price := read("price")
		preClose := read("pre_close")
		open := read("open")
		high := read("high")
		low := read("low")
		serverTime := read("server_time")
		read("neg_price")
		volume := read("volume")
		curVol := read("current_volume")
		if err != nil {
			return nil, err
		}
		if pos+4 > len(body) {
			return nil, fmt.Errorf("decode TDX HQ compact batch quote record %d amount: unexpected EOF", i)
		}
		amount := math.Float32frombits(binary.LittleEndian.Uint32(body[pos : pos+4]))
		pos += 4
		insideVolume := read("inside_volume")
		outsideVolume := read("outside_volume")
		read("s_amount")
		read("open_amount")
		bidDelta := read("bid_price")
		askDelta := read("ask_price")
		bidVol := read("bid_volume")
		askVol := read("ask_volume")
		if err != nil {
			return nil, err
		}
		if pos+56 > len(body) {
			return nil, fmt.Errorf("decode TDX HQ compact batch quote record %d tail: unexpected EOF", i)
		}
		tail := body[pos : pos+56]
		pos += 56

		items = append(items, HQCompactBatchQuote{
			Market:        quotesListMarketName(marketCode),
			Symbol:        code,
			MarketCode:    marketCode,
			Active:        active,
			Price:         float64(price) / 100,
			PreClose:      float64(price+preClose) / 100,
			Open:          float64(price+open) / 100,
			High:          float64(price+high) / 100,
			Low:           float64(price+low) / 100,
			ServerTime:    formatTDXQuoteTime(serverTime),
			Volume:        int64(volume),
			CurrentVolume: int64(curVol),
			Amount:        float64(amount),
			InsideVolume:  int64(insideVolume),
			OutsideVolume: int64(outsideVolume),
			BidPrice:      float64(price+bidDelta) / 100,
			BidVolume:     int64(bidVol),
			AskPrice:      float64(price+askDelta) / 100,
			AskVolume:     int64(askVol),
			RiseSpeed:     float64(int16(binary.LittleEndian.Uint16(tail[2:4]))) / 100,
			ShortTurnover: float64(int16(binary.LittleEndian.Uint16(tail[4:6]))) / 100,
			Min2Amount:    float64(math.Float32frombits(binary.LittleEndian.Uint32(tail[6:10]))),
			VolRatio:      float64(math.Float32frombits(binary.LittleEndian.Uint32(tail[22:26]))),
			Depth:         float64(math.Float32frombits(binary.LittleEndian.Uint32(tail[26:30]))),
		})
	}
	return items, nil
}

func (s *QuoteSession) CompactBatchQuotes(requests []QuoteRequest) ([]HQCompactBatchQuote, error) {
	packet, err := BuildHQCompactBatchQuotePacket(requests)
	if err != nil {
		return nil, err
	}
	body, err := s.call(packet)
	if err != nil {
		return nil, fmt.Errorf("TDX HQ compact batch quote %s: %w", s.server, err)
	}
	return DecodeHQCompactBatchQuoteResponse(body)
}

func FetchHQCompactBatchQuotes(ctx context.Context, requests []QuoteRequest, opts QuoteClientOptions) ([]HQCompactBatchQuote, error) {
	if err := validateCompactBatchQuoteRequests(requests); err != nil {
		return nil, err
	}
	return fetchHQRead(ctx, opts, "compact batch quotes", func(s *QuoteSession) ([]HQCompactBatchQuote, error) {
		return s.CompactBatchQuotes(requests)
	})
}

func BuildHQTickChartPacket(req HQTickChartRequest) ([]byte, error) {
	normalized, err := ParseHQTickChartRequest(req.Market, req.Symbol, req.Start, req.Count)
	if err != nil {
		return nil, err
	}
	body := make([]byte, 12)
	binary.LittleEndian.PutUint16(body[0:2], uint16(marketCodeForStandardHQ(normalized.Market)))
	copy(body[2:8], normalized.Symbol)
	binary.LittleEndian.PutUint16(body[8:10], uint16(normalized.Start))
	binary.LittleEndian.PutUint16(body[10:12], uint16(normalized.Count))
	return buildHQDirectFrame(0x01000802, cmdTickChart, body), nil
}

func DecodeHQTickChartResponse(req HQTickChartRequest, body []byte) ([]HQTickChartPoint, error) {
	if len(body) < 4 {
		return nil, fmt.Errorf("decode TDX HQ tick chart: body too short: %d", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[:2]))
	pos := 4
	points := make([]HQTickChartPoint, 0, count)
	var startPrice int
	var startAvg int
	for i := 0; i < count; i++ {
		price, next, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, fmt.Errorf("decode TDX HQ tick chart record %d price: %w", i, err)
		}
		pos = next
		avg, next, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, fmt.Errorf("decode TDX HQ tick chart record %d avg: %w", i, err)
		}
		pos = next
		volume, next, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, fmt.Errorf("decode TDX HQ tick chart record %d volume: %w", i, err)
		}
		pos = next
		points = append(points, HQTickChartPoint{
			Market: req.Market,
			Symbol: req.Symbol,
			Index:  req.Start + i,
			Time:   tickChartLabel(req.Start + i),
			Price:  float64(startPrice+price) / 100,
			Avg:    float64(startAvg+avg) / 10000,
			Volume: int64(volume),
		})
		if startPrice == 0 {
			startPrice = price
		}
		if startAvg == 0 {
			startAvg = avg
		}
	}
	return points, nil
}

func (s *QuoteSession) TickChart(req HQTickChartRequest) ([]HQTickChartPoint, error) {
	normalized, err := ParseHQTickChartRequest(req.Market, req.Symbol, req.Start, req.Count)
	if err != nil {
		return nil, err
	}
	packet, err := BuildHQTickChartPacket(normalized)
	if err != nil {
		return nil, err
	}
	body, err := s.call(packet)
	if err != nil {
		return nil, fmt.Errorf("TDX HQ tick chart %s: %w", s.server, err)
	}
	return DecodeHQTickChartResponse(normalized, body)
}

func FetchHQTickChart(ctx context.Context, req HQTickChartRequest, opts QuoteClientOptions) ([]HQTickChartPoint, error) {
	if _, err := ParseHQTickChartRequest(req.Market, req.Symbol, req.Start, req.Count); err != nil {
		return nil, err
	}
	return fetchHQRead(ctx, opts, "tick chart", func(s *QuoteSession) ([]HQTickChartPoint, error) {
		return s.TickChart(req)
	})
}

func tickChartLabel(index int) string {
	baseMorning := 9*60 + 30
	baseAfternoon := 13 * 60
	minuteOfDay := baseMorning + index
	if index >= 121 {
		minuteOfDay = baseAfternoon + (index - 121)
	}
	return fmt.Sprintf("%02d:%02d", minuteOfDay/60, minuteOfDay%60)
}
