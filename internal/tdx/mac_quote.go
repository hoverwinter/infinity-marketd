package tdx

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	DefaultMACHQServer = "121.36.248.138:7709"

	cmdMACSymbolQuotes uint16 = 0x122B
)

var DefaultMACHQServers = []string{
	DefaultMACHQServer,
	"123.60.47.136:7709",
	"121.37.207.165:7709",
}

type MACClientOptions struct {
	Server  string
	Servers []string
	Timeout time.Duration
}

type MACSymbolQuoteRequest struct {
	Market string
	Symbol string
}

type MACSymbolQuote struct {
	Market     string `json:"market"`
	Symbol     string `json:"symbol"`
	Name       string `json:"name"`
	MarketCode uint16 `json:"market_code"`
}

type MACSession struct {
	server  string
	timeout time.Duration
	conn    net.Conn
	client  quoteConn
}

func NormalizeMACHQServers(opts MACClientOptions) []string {
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
		servers = append(servers, DefaultMACHQServers...)
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

func OpenMACSession(ctx context.Context, server string, timeout time.Duration) (*MACSession, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return nil, fmt.Errorf("TDX MAC HQ server is required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", server)
	if err != nil {
		return nil, fmt.Errorf("connect TDX MAC HQ server %s: %w", server, err)
	}
	return &MACSession{
		server:  server,
		timeout: timeout,
		conn:    conn,
		client:  quoteConn{rw: conn},
	}, nil
}

func (s *MACSession) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func (s *MACSession) call(packet []byte) ([]byte, error) {
	if s.conn != nil && s.timeout > 0 {
		if err := s.conn.SetDeadline(time.Now().Add(s.timeout)); err != nil {
			return nil, fmt.Errorf("set TDX MAC connection deadline: %w", err)
		}
	}
	return s.client.call(packet)
}

func (s *MACSession) SymbolQuotes(requests []MACSymbolQuoteRequest) ([]MACSymbolQuote, error) {
	body, err := s.call(BuildMACSymbolQuotesPacket(requests))
	if err != nil {
		return nil, fmt.Errorf("TDX MAC symbol quotes %s: %w", s.server, err)
	}
	return DecodeMACSymbolQuotesResponse(body)
}

func FetchMACSymbolQuotes(ctx context.Context, requests []MACSymbolQuoteRequest, opts MACClientOptions) ([]MACSymbolQuote, error) {
	if len(requests) == 0 {
		return nil, fmt.Errorf("at least one MAC symbol quote is required")
	}
	const batchSize = 80
	var attempts []string
	for _, server := range NormalizeMACHQServers(opts) {
		session, err := OpenMACSession(ctx, server, opts.Timeout)
		if err != nil {
			attempts = append(attempts, err.Error())
			continue
		}
		var quotes []MACSymbolQuote
		var fetchErr error
		for start := 0; start < len(requests); start += batchSize {
			end := start + batchSize
			if end > len(requests) {
				end = len(requests)
			}
			var page []MACSymbolQuote
			page, fetchErr = session.SymbolQuotes(requests[start:end])
			if fetchErr != nil {
				break
			}
			quotes = append(quotes, page...)
		}
		_ = session.Close()
		if fetchErr != nil {
			attempts = append(attempts, fetchErr.Error())
			continue
		}
		return quotes, nil
	}
	return nil, fmt.Errorf("TDX MAC symbol quotes failed on %d server(s): %s", len(attempts), strings.Join(attempts, "; "))
}

func BuildMACSymbolQuotesPacket(requests []MACSymbolQuoteRequest) []byte {
	body := make([]byte, 0, 22+len(requests)*24)
	body = append(body, defaultMACSymbolQuoteBitmap()...)
	body = binary.LittleEndian.AppendUint16(body, uint16(len(requests)))
	for _, req := range requests {
		body = binary.LittleEndian.AppendUint16(body, uint16(marketCodeForStandardHQ(req.Market)))
		code := make([]byte, 22)
		copy(code, strings.TrimSpace(req.Symbol))
		body = append(body, code...)
	}
	return buildMACDirectFrame(cmdMACSymbolQuotes, body)
}

func DecodeMACSymbolQuotesResponse(body []byte) ([]MACSymbolQuote, error) {
	if len(body) < 26 {
		return nil, fmt.Errorf("decode TDX MAC symbol quotes: body too short: %d", len(body))
	}
	activeBits := spActiveBits(body[:20])
	count := int(binary.LittleEndian.Uint16(body[24:26]))
	rowLen := 68 + 4*len(activeBits)
	pos := 26
	items := make([]MACSymbolQuote, 0, count)
	for i := 0; i < count; i++ {
		if pos+rowLen > len(body) {
			return nil, fmt.Errorf("decode TDX MAC symbol quotes record %d: unexpected EOF", i)
		}
		row := body[pos : pos+rowLen]
		marketCode := binary.LittleEndian.Uint16(row[0:2])
		items = append(items, MACSymbolQuote{
			Market:     quotesListMarketName(marketCode),
			MarketCode: marketCode,
			Symbol:     decodeSecurityName(row[2:24]),
			Name:       decodeSecurityName(row[24:68]),
		})
		pos += rowLen
	}
	return items, nil
}

func buildMACDirectFrame(method uint16, body []byte) []byte {
	innerLen := len(body) + 2
	packet := make([]byte, 10+innerLen)
	packet[0] = 0x01
	packet[5] = 0x01
	binary.LittleEndian.PutUint16(packet[6:8], uint16(innerLen))
	binary.LittleEndian.PutUint16(packet[8:10], uint16(innerLen))
	binary.LittleEndian.PutUint16(packet[10:12], method)
	copy(packet[12:], body)
	return packet
}

func defaultMACSymbolQuoteBitmap() []byte {
	return []byte{
		0xff, 0xfc, 0xe1, 0xcc, 0x3f, 0x08, 0x03, 0x01, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
}
