package tdx

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultHQServer = "180.153.18.170:7709"

	tdxMarketSZ = 0
	tdxMarketSH = 1
	tdxMarketBJ = 2
)

var DefaultHQServers = []string{
	DefaultHQServer,
	"60.191.117.167:7709",
	"119.147.212.81:7709",
	"101.227.73.20:7709",
	"14.17.75.71:7709",
}

var hqSetupPackets = [][]byte{
	{0x0c, 0x02, 0x18, 0x93, 0x00, 0x01, 0x03, 0x00, 0x03, 0x00, 0x0d, 0x00, 0x01},
	{0x0c, 0x02, 0x18, 0x94, 0x00, 0x01, 0x03, 0x00, 0x03, 0x00, 0x0d, 0x00, 0x02},
	{
		0x0c, 0x03, 0x18, 0x99, 0x00, 0x01, 0x20, 0x00, 0x20, 0x00, 0xdb, 0x0f, 0xd5,
		0xd0, 0xc9, 0xcc, 0xd6, 0xa4, 0xa8, 0xaf, 0x00, 0x00, 0x00, 0x8f, 0xc2, 0x25,
		0x40, 0x13, 0x00, 0x00, 0xd5, 0x00, 0xc9, 0xcc, 0xbd, 0xf0, 0xd7, 0xea, 0x00,
		0x00, 0x00, 0x02,
	},
}

type QuoteRequest struct {
	Market string `json:"market"`
	Symbol string `json:"symbol"`
}

type Quote struct {
	Market             string       `json:"market"`
	Symbol             string       `json:"symbol"`
	Price              float64      `json:"price"`
	LastClose          float64      `json:"last_close"`
	Open               float64      `json:"open"`
	High               float64      `json:"high"`
	Low                float64      `json:"low"`
	ServerTime         string       `json:"server_time"`
	ServerIntradayTime string       `json:"server_intraday_time"`
	TradeDate          string       `json:"trade_date,omitempty"`
	QuoteTime          string       `json:"quote_time,omitempty"`
	Volume             int64        `json:"volume"`
	CurrentVol         int64        `json:"current_volume"`
	Amount             float64      `json:"amount"`
	SellVolume         int64        `json:"sell_volume"`
	BuyVolume          int64        `json:"buy_volume"`
	Bids               []QuoteLevel `json:"bids"`
	Asks               []QuoteLevel `json:"asks"`
}

type QuoteLevel struct {
	Price  float64 `json:"price"`
	Volume int64   `json:"volume"`
}

type QuoteClientOptions struct {
	Server          string
	Servers         []string
	MACServers      []string
	Timeout         time.Duration
	BatchSize       int
	TradeDate       time.Time
	Location        *time.Location
	BestIP          bool
	BestIPCachePath string
	BestIPMaxAge    time.Duration
	BestIPRefresh   bool
}

func ParseQuoteRequest(input string) (QuoteRequest, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return QuoteRequest{}, fmt.Errorf("symbol is required")
	}

	market := ""
	symbol := raw
	if left, right, ok := strings.Cut(raw, ":"); ok {
		market = strings.ToLower(strings.TrimSpace(left))
		symbol = strings.TrimSpace(right)
	}
	if market == "" {
		market = InferMarketFromCode(symbol)
	}
	if market != "sh" && market != "sz" && market != "bj" {
		return QuoteRequest{}, fmt.Errorf("unsupported realtime quote market %q", market)
	}
	if len(symbol) != 6 || !allDigits(symbol) {
		return QuoteRequest{}, fmt.Errorf("unsupported symbol %q", symbol)
	}
	return QuoteRequest{Market: market, Symbol: symbol}, nil
}

func FetchRealtimeQuotes(ctx context.Context, requests []QuoteRequest, opts QuoteClientOptions) ([]Quote, error) {
	if len(requests) == 0 {
		return nil, fmt.Errorf("at least one symbol is required")
	}
	return FetchRealtimeQuoteBatches(ctx, requests, opts)
}

func BuildQuoteRequestPacket(requests []QuoteRequest) []byte {
	dataLen := uint16(len(requests)*7 + 12)
	packet := make([]byte, 22, 22+len(requests)*7)
	binary.LittleEndian.PutUint16(packet[0:2], 0x010c)
	binary.LittleEndian.PutUint32(packet[2:6], 0x02006320)
	binary.LittleEndian.PutUint16(packet[6:8], dataLen)
	binary.LittleEndian.PutUint16(packet[8:10], dataLen)
	binary.LittleEndian.PutUint32(packet[10:14], 0x0005053e)
	binary.LittleEndian.PutUint32(packet[14:18], 0)
	binary.LittleEndian.PutUint16(packet[18:20], 0)
	binary.LittleEndian.PutUint16(packet[20:22], uint16(len(requests)))

	for _, req := range requests {
		marketCode := quoteMarketCode(req.Market)
		packet = append(packet, byte(marketCode))
		code := make([]byte, 6)
		copy(code, req.Symbol)
		packet = append(packet, code...)
	}
	return packet
}

func quoteMarketCode(market string) int {
	switch strings.ToLower(strings.TrimSpace(market)) {
	case "sh":
		return tdxMarketSH
	case "bj":
		return tdxMarketBJ
	default:
		return tdxMarketSZ
	}
}

func DecodeQuoteResponse(body []byte) ([]Quote, error) {
	if len(body) < 4 {
		return nil, fmt.Errorf("TDX quote response too short: %d bytes", len(body))
	}
	pos := 2
	num := int(binary.LittleEndian.Uint16(body[pos : pos+2]))
	pos += 2
	quotes := make([]Quote, 0, num)
	for i := 0; i < num; i++ {
		if pos+9 > len(body) {
			return nil, fmt.Errorf("TDX quote response truncated before quote %d", i)
		}
		marketCode := int(body[pos])
		pos++
		code := strings.TrimRight(string(body[pos:pos+6]), "\x00")
		pos += 6
		pos += 2 // active1

		price, next, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		pos = next
		lastCloseDiff, pos, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		openDiff, pos, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		highDiff, pos, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		lowDiff, pos, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		serverRaw, pos, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		_, pos, err = readTDXVarInt(body, pos) // reversed_bytes1
		if err != nil {
			return nil, err
		}
		vol, pos, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		curVol, pos, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		if pos+4 > len(body) {
			return nil, fmt.Errorf("TDX quote response truncated before amount")
		}
		amount := decodeTDXFloat(binary.LittleEndian.Uint32(body[pos : pos+4]))
		pos += 4
		sellVol, pos, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		buyVol, pos, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		for j := 0; j < 2; j++ {
			if _, pos, err = readTDXVarInt(body, pos); err != nil {
				return nil, err
			}
		}

		bids := make([]QuoteLevel, 5)
		asks := make([]QuoteLevel, 5)
		for level := 0; level < 5; level++ {
			bidDiff, p, err := readTDXVarInt(body, pos)
			if err != nil {
				return nil, err
			}
			pos = p
			askDiff, p, err := readTDXVarInt(body, pos)
			if err != nil {
				return nil, err
			}
			pos = p
			bidVol, p, err := readTDXVarInt(body, pos)
			if err != nil {
				return nil, err
			}
			pos = p
			askVol, p, err := readTDXVarInt(body, pos)
			if err != nil {
				return nil, err
			}
			pos = p
			bids[level] = QuoteLevel{Price: quotePrice(price, bidDiff), Volume: int64(bidVol)}
			asks[level] = QuoteLevel{Price: quotePrice(price, askDiff), Volume: int64(askVol)}
		}

		if pos+2 > len(body) {
			return nil, fmt.Errorf("TDX quote response truncated before tail")
		}
		pos += 2 // reversed_bytes4
		for j := 0; j < 4; j++ {
			if _, pos, err = readTDXVarInt(body, pos); err != nil {
				return nil, err
			}
		}
		if pos+4 > len(body) {
			return nil, fmt.Errorf("TDX quote response truncated before active2")
		}
		pos += 4 // reversed_bytes9, active2

		market, err := quoteMarketName(marketCode)
		if err != nil {
			return nil, err
		}
		serverTime := formatTDXQuoteTime(serverRaw)
		quotes = append(quotes, Quote{
			Market:             market,
			Symbol:             code,
			Price:              quotePrice(price, 0),
			LastClose:          quotePrice(price, lastCloseDiff),
			Open:               quotePrice(price, openDiff),
			High:               quotePrice(price, highDiff),
			Low:                quotePrice(price, lowDiff),
			ServerTime:         serverTime,
			ServerIntradayTime: serverTime,
			Volume:             int64(vol),
			CurrentVol:         int64(curVol),
			Amount:             amount,
			SellVolume:         int64(sellVol),
			BuyVolume:          int64(buyVol),
			Bids:               bids,
			Asks:               asks,
		})
	}
	return quotes, nil
}

type quoteConn struct {
	rw io.ReadWriter
}

func (c quoteConn) call(packet []byte) ([]byte, error) {
	if _, err := c.rw.Write(packet); err != nil {
		return nil, fmt.Errorf("send packet: %w", err)
	}
	header := make([]byte, 16)
	if _, err := io.ReadFull(c.rw, header); err != nil {
		return nil, fmt.Errorf("read response header: %w", err)
	}
	zipSize := int(binary.LittleEndian.Uint16(header[12:14]))
	unzipSize := int(binary.LittleEndian.Uint16(header[14:16]))
	body := make([]byte, zipSize)
	if _, err := io.ReadFull(c.rw, body); err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if zipSize == unzipSize {
		return body, nil
	}
	zr, err := zlib.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("open compressed response: %w", err)
	}
	defer zr.Close()
	unzipped, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("decompress response: %w", err)
	}
	return unzipped, nil
}

func readTDXVarInt(data []byte, pos int) (int, int, error) {
	if pos >= len(data) {
		return 0, pos, fmt.Errorf("TDX varint truncated at %d", pos)
	}
	posByte := 6
	b := data[pos]
	value := int(b & 0x3f)
	negative := b&0x40 != 0
	if b&0x80 != 0 {
		for {
			pos++
			if pos >= len(data) {
				return 0, pos, fmt.Errorf("TDX varint truncated at %d", pos)
			}
			b = data[pos]
			value += int(b&0x7f) << posByte
			posByte += 7
			if b&0x80 == 0 {
				break
			}
		}
	}
	pos++
	if negative {
		value = -value
	}
	return value, pos, nil
}

func decodeTDXFloat(raw uint32) float64 {
	logpoint := int(raw >> 24)
	hleax := int((raw >> 16) & 0xff)
	lheax := int((raw >> 8) & 0xff)
	lleax := int(raw & 0xff)

	ecx := logpoint*2 - 0x7f
	edx := logpoint*2 - 0x86
	esi := logpoint*2 - 0x8e
	eax := logpoint*2 - 0x96

	xmm6 := math.Pow(2.0, math.Abs(float64(ecx)))
	if ecx < 0 {
		xmm6 = 1.0 / xmm6
	}

	var xmm4 float64
	if hleax > 0x80 {
		xmm4 = math.Pow(2.0, float64(edx))*128.0 + float64(hleax&0x7f)*math.Pow(2.0, float64(edx+1))
	} else if edx >= 0 {
		xmm4 = math.Pow(2.0, float64(edx)) * float64(hleax)
	} else {
		xmm4 = (1 / math.Pow(2.0, float64(edx))) * float64(hleax)
	}

	xmm3 := math.Pow(2.0, float64(esi)) * float64(lheax)
	xmm1 := math.Pow(2.0, float64(eax)) * float64(lleax)
	if hleax&0x80 != 0 {
		xmm3 *= 2.0
		xmm1 *= 2.0
	}
	return xmm6 + xmm4 + xmm3 + xmm1
}

func quotePrice(base int, diff int) float64 {
	return float64(base+diff) / 100.0
}

func quoteMarketName(market int) (string, error) {
	switch market {
	case tdxMarketSZ:
		return "sz", nil
	case tdxMarketSH:
		return "sh", nil
	case tdxMarketBJ:
		return "bj", nil
	default:
		return "", fmt.Errorf("unsupported TDX quote market code %d", market)
	}
}

func formatTDXQuoteTime(raw int) string {
	s := strconv.Itoa(raw)
	if len(s) < 7 {
		s = strings.Repeat("0", 7-len(s)) + s
	}
	prefix := s[:len(s)-6]
	last6, _ := strconv.Atoi(s[len(s)-6:])
	mid2, _ := strconv.Atoi(s[len(s)-6 : len(s)-4])
	if mid2 < 60 {
		last4, _ := strconv.Atoi(s[len(s)-4:])
		return fmt.Sprintf("%s:%02d:%06.3f", prefix, mid2, float64(last4)*60/10000.0)
	}
	minute := last6 * 60 / 1000000
	second := float64((last6*60)%1000000) * 60 / 1000000.0
	return fmt.Sprintf("%s:%02d:%06.3f", prefix, minute, second)
}
