package tdx

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"strings"
	"time"
)

// Fund-specialized TDX 7727 reads ported from millken/tdx: fund K-line (0x2489)
// and raw fund detail (0x2488). These use SP-mode framing but a distinct open
// sequence — SP login (0x2454) then fund bootstrap (0x23F0), without the
// ping/auth/stage2 handshake. Live reads; never write ClickHouse. No public
// fund server defaults exist, so callers must pass an explicit server.

const (
	cmdFundBootstrap uint16 = 0x23F0
	cmdFundDetail    uint16 = 0x2488
	cmdFundKline     uint16 = 0x2489

	defaultFundDetailMode uint16 = 50
)

// fund kline wire period values (subset; used for time decoding).
const (
	fundPeriod5Min  uint16 = 0
	fundPeriod15Min uint16 = 1
	fundPeriod30Min uint16 = 2
	fundPeriod60Min uint16 = 3
	fundPeriodDay   uint16 = 4
	fundPeriodWeek  uint16 = 5
	fundPeriodMonth uint16 = 6
	fundPeriod1Min  uint16 = 7
	fundPeriodMulti uint16 = 8
)

var fundPeriodMap = map[string]uint16{
	"1m": fundPeriod1Min, "5m": fundPeriod5Min, "15m": fundPeriod15Min,
	"30m": fundPeriod30Min, "60m": fundPeriod60Min, "day": fundPeriodDay,
	"week": fundPeriodWeek, "month": fundPeriodMonth,
}

// HQFundDetailItem is one 16-byte fund detail row (semantics not fully mapped).
type HQFundDetailItem struct {
	ID     uint32    `json:"id"`
	Values [6]uint16 `json:"values"`
}

// HQFundDetail is a 0x2488 fund detail response.
type HQFundDetail struct {
	Category byte               `json:"category"`
	Code     string             `json:"code"`
	Items    []HQFundDetailItem `json:"items"`
}

// HQFundBar is one fund K-line bar.
type HQFundBar struct {
	Time   string  `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Amount float64 `json:"amount"`
	Volume int64   `json:"volume"`
}

// OpenFundSession dials a 7727 server and performs SP login + fund bootstrap.
func OpenFundSession(ctx context.Context, server string, timeout time.Duration) (*SPSession, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return nil, fmt.Errorf("fund server is required (no public fund defaults)")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", server)
	if err != nil {
		return nil, fmt.Errorf("connect fund server %s: %w", server, err)
	}
	s := &SPSession{server: server, conn: conn, r: bufio.NewReader(conn), timeout: timeout}
	if _, err := s.exchange(buildSPFrame(0x01, cmdSPLogin, spLoginBody), cmdSPLogin); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("fund sp login %s: %w", server, err)
	}
	if _, err := s.exchange(buildSPFrame(0x01, cmdFundBootstrap, nil), cmdFundBootstrap); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("fund bootstrap %s: %w", server, err)
	}
	return s, nil
}

// FundDetail fetches raw fund detail over an open fund session.
func (s *SPSession) FundDetail(code string, mode uint16) (HQFundDetail, error) {
	frame, err := buildFundDetailFrame(code, mode)
	if err != nil {
		return HQFundDetail{}, err
	}
	body, err := s.exchange(frame, cmdFundDetail)
	if err != nil {
		return HQFundDetail{}, err
	}
	return DecodeFundDetail(body)
}

// FundKline fetches fund K-line bars over an open fund session.
func (s *SPSession) FundKline(code string, period string, count int) ([]HQFundBar, error) {
	if count <= 0 || count > 800 {
		return nil, fmt.Errorf("fund kline count must be 1..800, got %d", count)
	}
	pv, ok := fundPeriodMap[strings.ToLower(period)]
	if !ok {
		return nil, fmt.Errorf("unsupported fund period %q (use 1m,5m,15m,30m,60m,day,week,month)", period)
	}
	frame, err := buildFundKlineFrame(code, pv, 1, 0, uint32(count))
	if err != nil {
		return nil, err
	}
	body, err := s.exchange(frame, cmdFundKline)
	if err != nil {
		return nil, err
	}
	return DecodeFundKlines(body, pv)
}

// FetchFundDetail opens a fund session to an explicit server and fetches detail.
func FetchFundDetail(ctx context.Context, server string, code string, mode uint16, timeout time.Duration) (HQFundDetail, error) {
	session, err := OpenFundSession(ctx, server, timeout)
	if err != nil {
		return HQFundDetail{}, err
	}
	defer session.Close()
	return session.FundDetail(code, mode)
}

// FetchFundKline opens a fund session to an explicit server and fetches K-line.
func FetchFundKline(ctx context.Context, server string, code string, period string, count int, timeout time.Duration) ([]HQFundBar, error) {
	session, err := OpenFundSession(ctx, server, timeout)
	if err != nil {
		return nil, err
	}
	defer session.Close()
	return session.FundKline(code, period, count)
}

func buildFundDetailFrame(code string, mode uint16) ([]byte, error) {
	code, err := fundNormalizeCode(code)
	if err != nil {
		return nil, err
	}
	if mode == 0 {
		mode = defaultFundDetailMode
	}
	body := make([]byte, 38)
	body[0] = fundInferCategory(code)
	copy(body[1:24], code)
	binary.LittleEndian.PutUint16(body[28:30], mode)
	return buildSPFrame(0x01, cmdFundDetail, body), nil
}

func buildFundKlineFrame(code string, period uint16, times uint16, start uint32, count uint32) ([]byte, error) {
	code, err := fundNormalizeCode(code)
	if err != nil {
		return nil, err
	}
	body := make([]byte, 52)
	body[0] = fundInferCategory(code)
	copy(body[1:24], code)
	binary.LittleEndian.PutUint16(body[24:26], period)
	binary.LittleEndian.PutUint16(body[26:28], times)
	binary.LittleEndian.PutUint32(body[28:32], start)
	binary.LittleEndian.PutUint32(body[32:36], count)
	return buildSPFrame(0x01, cmdFundKline, body), nil
}

// DecodeFundDetail decodes a 0x2488 response: count at [36:38], then 16-byte rows.
func DecodeFundDetail(body []byte) (HQFundDetail, error) {
	if len(body) < 38 {
		return HQFundDetail{}, fmt.Errorf("decode TDX fund detail: body too short: %d", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[36:38]))
	if len(body) < 38+count*16 {
		return HQFundDetail{}, fmt.Errorf("decode TDX fund detail: truncated len=%d want>=%d", len(body), 38+count*16)
	}
	detail := HQFundDetail{
		Category: body[0],
		Code:     strings.TrimSpace(strings.TrimRight(string(body[1:36]), "\x00")),
		Items:    make([]HQFundDetailItem, 0, count),
	}
	pos := 38
	for i := 0; i < count; i++ {
		row := body[pos : pos+16]
		item := HQFundDetailItem{ID: binary.LittleEndian.Uint32(row[0:4])}
		for j := range item.Values {
			item.Values[j] = binary.LittleEndian.Uint16(row[4+j*2 : 6+j*2])
		}
		detail.Items = append(detail.Items, item)
		pos += 16
	}
	return detail, nil
}

// DecodeFundKlines decodes a 0x2489 response: period at [24:26], count at [40:42],
// then 32-byte records (time + float32 OHLC/amount + uint32 volume).
func DecodeFundKlines(body []byte, period uint16) ([]HQFundBar, error) {
	if len(body) < 42 {
		return nil, fmt.Errorf("decode TDX fund kline: body too short: %d", len(body))
	}
	if rp := binary.LittleEndian.Uint16(body[24:26]); rp != 0 {
		period = rp
	}
	count := int(binary.LittleEndian.Uint16(body[40:42]))
	pos := 42
	bars := make([]HQFundBar, 0, count)
	for i := 0; i < count; i++ {
		if pos+32 > len(body) {
			return nil, fmt.Errorf("decode TDX fund kline: record %d truncated", i)
		}
		rec := body[pos : pos+32]
		bars = append(bars, HQFundBar{
			Time:   fundKlineTime(rec[0:4], period),
			Open:   float64(math.Float32frombits(binary.LittleEndian.Uint32(rec[4:8]))),
			High:   float64(math.Float32frombits(binary.LittleEndian.Uint32(rec[8:12]))),
			Low:    float64(math.Float32frombits(binary.LittleEndian.Uint32(rec[12:16]))),
			Close:  float64(math.Float32frombits(binary.LittleEndian.Uint32(rec[16:20]))),
			Amount: float64(math.Float32frombits(binary.LittleEndian.Uint32(rec[20:24]))),
			Volume: int64(binary.LittleEndian.Uint32(rec[24:28])),
		})
		pos += 32
	}
	return bars, nil
}

// fundNormalizeCode strips an sh/sz/bj prefix and requires a 6-digit code.
func fundNormalizeCode(code string) (string, error) {
	c := strings.ToLower(strings.TrimSpace(code))
	if len(c) == 8 && (strings.HasPrefix(c, "sh") || strings.HasPrefix(c, "sz") || strings.HasPrefix(c, "bj")) {
		c = c[2:]
	}
	if len(c) != 6 || !allDigits(c) {
		return "", fmt.Errorf("invalid fund code %q", code)
	}
	if strings.HasPrefix(c, "92") || strings.HasPrefix(c, "899") {
		return "", fmt.Errorf("fund reads do not support Beijing market code %q", code)
	}
	return c, nil
}

// fundInferCategory mirrors millken inferFundCategory (SH 73x -> 0x22 else 0x21).
func fundInferCategory(code string) byte {
	if len(code) == 6 && strings.HasPrefix(code, "73") {
		return 0x22
	}
	return 0x21
}

func fundKlineTime(data []byte, period uint16) string {
	loc := shanghaiLoc()
	if len(data) < 4 {
		return ""
	}
	switch period {
	case fundPeriod1Min, fundPeriod5Min, fundPeriod15Min, fundPeriod30Min, fundPeriod60Min, fundPeriodMulti:
		ymd := binary.LittleEndian.Uint16(data[:2])
		hm := binary.LittleEndian.Uint16(data[2:4])
		dayBits := ymd % 2048
		year := int(ymd>>11) + 2004
		t := time.Date(year, time.Month(dayBits/100), int(dayBits%100), int(hm/60), int(hm%60), 0, 0, loc)
		return t.Format("2006-01-02 15:04")
	default:
		ymd := binary.LittleEndian.Uint32(data[:4])
		t := time.Date(int(ymd/10000), time.Month((ymd%10000)/100), int(ymd%100), 0, 0, 0, 0, loc)
		return t.Format("2006-01-02")
	}
}

func shanghaiLoc() *time.Location {
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return loc
	}
	return time.FixedZone("CST", 8*3600)
}
