package tdx

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"strings"
	"time"
	"unicode/utf8"
)

const DefaultExHQServer = "47.112.95.207:7720"

var DefaultExHQServers = []string{
	DefaultExHQServer,
	"47.102.108.214:7727",
	"112.74.214.43:7727",
	"120.25.218.6:7727",
	"116.205.143.214:7727",
	"124.71.223.19:7727",
	"61.152.107.141:7727",
	"121.14.110.210:7727",
}

var exHQSetupPacket = []byte{
	0x01, 0x01, 0x48, 0x65, 0x00, 0x01, 0x52, 0x00, 0x52, 0x00, 0x54, 0x24,
	0x1f, 0x32, 0xc6, 0xe5, 0xd5, 0x3d, 0xfb, 0x41, 0x1f, 0x32, 0xc6, 0xe5,
	0xd5, 0x3d, 0xfb, 0x41, 0x1f, 0x32, 0xc6, 0xe5, 0xd5, 0x3d, 0xfb, 0x41,
	0x1f, 0x32, 0xc6, 0xe5, 0xd5, 0x3d, 0xfb, 0x41, 0x1f, 0x32, 0xc6, 0xe5,
	0xd5, 0x3d, 0xfb, 0x41, 0x1f, 0x32, 0xc6, 0xe5, 0xd5, 0x3d, 0xfb, 0x41,
	0x1f, 0x32, 0xc6, 0xe5, 0xd5, 0x3d, 0xfb, 0x41, 0xcc, 0xe1, 0x6d, 0xff,
	0xd5, 0xba, 0x3f, 0xb8, 0xcb, 0xc5, 0x7a, 0x05, 0x4f, 0x77, 0x48, 0xea,
}

type ExQuoteRequest struct {
	Market int    `json:"market"`
	Code   string `json:"code"`
}

type ExMarket struct {
	Market    int    `json:"market"`
	Category  int    `json:"category"`
	Name      string `json:"name,omitempty"`
	ShortName string `json:"short_name,omitempty"`
}

type ExQuote struct {
	Market    int          `json:"market"`
	Code      string       `json:"code"`
	PreClose  float64      `json:"pre_close"`
	Open      float64      `json:"open"`
	High      float64      `json:"high"`
	Low       float64      `json:"low"`
	Price     float64      `json:"price"`
	KaiCang   int64        `json:"kaicang"`
	ZongLiang int64        `json:"zongliang"`
	XianLiang int64        `json:"xianliang"`
	NeiPan    int64        `json:"neipan"`
	WaiPan    int64        `json:"waipan"`
	ChiCang   int64        `json:"chicang"`
	Bids      []QuoteLevel `json:"bids"`
	Asks      []QuoteLevel `json:"asks"`
}

type ExQuoteClientOptions struct {
	Server  string
	Servers []string
	Timeout time.Duration
}

type ExQuoteSession struct {
	server  string
	timeout time.Duration
	conn    net.Conn
	client  quoteConn
}

func ParseExQuoteRequest(market int, code string) (ExQuoteRequest, error) {
	code = strings.TrimSpace(code)
	if market <= 0 {
		return ExQuoteRequest{}, fmt.Errorf("extended quote market must be positive")
	}
	if market > 255 {
		return ExQuoteRequest{}, fmt.Errorf("unsupported extended quote market %d", market)
	}
	if code == "" || len(code) > 9 || !isExQuoteCode(code) {
		return ExQuoteRequest{}, fmt.Errorf("unsupported extended quote code %q", code)
	}
	return ExQuoteRequest{Market: market, Code: code}, nil
}

func NormalizeExHQServers(opts ExQuoteClientOptions) []string {
	var servers []string
	add := func(value string) {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				servers = append(servers, part)
			}
		}
	}
	add(opts.Server)
	for _, server := range opts.Servers {
		add(server)
	}
	if len(servers) == 0 {
		servers = append(servers, DefaultExHQServers...)
	}
	seen := make(map[string]struct{}, len(servers))
	out := servers[:0]
	for _, server := range servers {
		if _, ok := seen[server]; ok {
			continue
		}
		seen[server] = struct{}{}
		out = append(out, server)
	}
	return append([]string(nil), out...)
}

func OpenExQuoteSession(ctx context.Context, server string, timeout time.Duration) (*ExQuoteSession, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return nil, fmt.Errorf("TDX ExHQ server is required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", server)
	if err != nil {
		return nil, fmt.Errorf("connect TDX ExHQ server %s: %w", server, err)
	}
	session := &ExQuoteSession{
		server:  server,
		timeout: timeout,
		conn:    conn,
		client:  quoteConn{rw: conn},
	}
	return session, nil
}

func (s *ExQuoteSession) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func (s *ExQuoteSession) Markets() ([]ExMarket, error) {
	body, err := s.call(BuildExMarketListPacket())
	if err != nil {
		return nil, fmt.Errorf("TDX ExHQ market list %s: %w", s.server, err)
	}
	markets, err := DecodeExMarketListResponse(body)
	if err != nil {
		return nil, fmt.Errorf("decode TDX ExHQ market list response from %s: %w", s.server, err)
	}
	return markets, nil
}

func (s *ExQuoteSession) Quote(req ExQuoteRequest) (ExQuote, error) {
	normalized, err := ParseExQuoteRequest(req.Market, req.Code)
	if err != nil {
		return ExQuote{}, err
	}
	req = normalized
	body, err := s.call(BuildExQuoteRequestPacket(req))
	if err != nil {
		return ExQuote{}, fmt.Errorf("TDX ExHQ quote request %s: %w", s.server, err)
	}
	quote, err := DecodeExQuoteResponse(body)
	if err != nil {
		return ExQuote{}, fmt.Errorf("decode TDX ExHQ quote response from %s: %w", s.server, err)
	}
	return quote, nil
}

func (s *ExQuoteSession) call(packet []byte) ([]byte, error) {
	if s.conn != nil && s.timeout > 0 {
		if err := s.conn.SetDeadline(time.Now().Add(s.timeout)); err != nil {
			return nil, fmt.Errorf("set TDX ExHQ connection deadline: %w", err)
		}
	}
	return s.client.call(packet)
}

func FetchExMarkets(ctx context.Context, opts ExQuoteClientOptions) ([]ExMarket, error) {
	var attempts []string
	for _, server := range NormalizeExHQServers(opts) {
		session, err := OpenExQuoteSession(ctx, server, opts.Timeout)
		if err != nil {
			attempts = append(attempts, err.Error())
			continue
		}
		markets, fetchErr := session.Markets()
		_ = session.Close()
		if fetchErr != nil {
			attempts = append(attempts, fetchErr.Error())
			continue
		}
		return markets, nil
	}
	return nil, fmt.Errorf("TDX ExHQ market list failed on %d server(s): %s", len(attempts), strings.Join(attempts, "; "))
}

func FetchExQuote(ctx context.Context, req ExQuoteRequest, opts ExQuoteClientOptions) (ExQuote, error) {
	normalized, err := ParseExQuoteRequest(req.Market, req.Code)
	if err != nil {
		return ExQuote{}, err
	}
	req = normalized
	var attempts []string
	for _, server := range NormalizeExHQServers(opts) {
		session, err := OpenExQuoteSession(ctx, server, opts.Timeout)
		if err != nil {
			attempts = append(attempts, err.Error())
			continue
		}
		quote, fetchErr := session.Quote(req)
		_ = session.Close()
		if fetchErr != nil {
			attempts = append(attempts, fetchErr.Error())
			if strings.Contains(fetchErr.Error(), "decode TDX ExHQ quote response") {
				return ExQuote{}, fetchErr
			}
			continue
		}
		return quote, nil
	}
	return ExQuote{}, fmt.Errorf("TDX ExHQ quote request failed on %d server(s): %s", len(attempts), strings.Join(attempts, "; "))
}

func BuildExMarketListPacket() []byte {
	return []byte{0x01, 0x02, 0x48, 0x69, 0x00, 0x01, 0x02, 0x00, 0x02, 0x00, 0xf4, 0x23}
}

func DecodeExMarketListResponse(body []byte) ([]ExMarket, error) {
	if len(body) < 2 {
		return nil, fmt.Errorf("TDX ExHQ market list response too short: %d bytes", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[:2]))
	pos := 2
	markets := make([]ExMarket, 0, count)
	for i := 0; i < count; i++ {
		if pos+64 > len(body) {
			return nil, fmt.Errorf("TDX ExHQ market list response truncated at item %d", i)
		}
		record := body[pos : pos+64]
		category := int(record[0])
		market := int(record[33])
		if category != 0 || market != 0 {
			markets = append(markets, ExMarket{
				Market:    market,
				Category:  category,
				Name:      decodeExCString(record[1:33]),
				ShortName: decodeExCString(record[34:36]),
			})
		}
		pos += 64
	}
	return markets, nil
}

func BuildExQuoteRequestPacket(req ExQuoteRequest) []byte {
	packet := []byte{0x01, 0x01, 0x08, 0x02, 0x02, 0x01, 0x0c, 0x00, 0x0c, 0x00, 0xfa, 0x23}
	packet = append(packet, byte(req.Market))
	code := make([]byte, 9)
	copy(code, req.Code)
	packet = append(packet, code...)
	return packet
}

func DecodeExQuoteResponse(body []byte) (ExQuote, error) {
	const minExQuoteBodyLen = 10 + 4 + 136
	if len(body) < minExQuoteBodyLen {
		return ExQuote{}, fmt.Errorf("TDX ExHQ quote response too short: %d bytes", len(body))
	}
	market := int(body[0])
	code := strings.TrimRight(string(body[1:10]), "\x00")
	pos := 14

	preClose := readFloat32(body[pos:])
	openPrice := readFloat32(body[pos+4:])
	high := readFloat32(body[pos+8:])
	low := readFloat32(body[pos+12:])
	price := readFloat32(body[pos+16:])
	pos += 20

	kaicang := readUint32AsInt64(body[pos:])
	pos += 8 // kaicang + ignored
	zongliang := readUint32AsInt64(body[pos:])
	xianliang := readUint32AsInt64(body[pos+4:])
	pos += 12 // zongliang + xianliang + ignored
	neipan := readUint32AsInt64(body[pos:])
	waipan := readUint32AsInt64(body[pos+4:])
	pos += 12 // neipan + waipan + ignored
	chicang := readUint32AsInt64(body[pos:])
	pos += 4

	bids := make([]QuoteLevel, 5)
	for i := 0; i < 5; i++ {
		bids[i].Price = readFloat32(body[pos+i*4:])
	}
	pos += 20
	for i := 0; i < 5; i++ {
		bids[i].Volume = readUint32AsInt64(body[pos+i*4:])
	}
	pos += 20

	asks := make([]QuoteLevel, 5)
	for i := 0; i < 5; i++ {
		asks[i].Price = readFloat32(body[pos+i*4:])
	}
	pos += 20
	for i := 0; i < 5; i++ {
		asks[i].Volume = readUint32AsInt64(body[pos+i*4:])
	}

	return ExQuote{
		Market:    market,
		Code:      code,
		PreClose:  preClose,
		Open:      openPrice,
		High:      high,
		Low:       low,
		Price:     price,
		KaiCang:   kaicang,
		ZongLiang: zongliang,
		XianLiang: xianliang,
		NeiPan:    neipan,
		WaiPan:    waipan,
		ChiCang:   chicang,
		Bids:      bids,
		Asks:      asks,
	}, nil
}

func cleanUTF8CString(raw []byte) string {
	end := len(raw)
	for end > 0 && raw[end-1] == 0 {
		end--
	}
	raw = raw[:end]
	if len(raw) == 0 || !utf8.Valid(raw) {
		return ""
	}
	return string(raw)
}

func readFloat32(raw []byte) float64 {
	return float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[:4])))
}

func readUint32AsInt64(raw []byte) int64 {
	return int64(binary.LittleEndian.Uint32(raw[:4]))
}

func isExQuoteCode(code string) bool {
	for _, b := range []byte(code) {
		if b <= 0x20 || b > 0x7e || b == ':' {
			return false
		}
	}
	return true
}

var _ io.Closer = (*ExQuoteSession)(nil)
