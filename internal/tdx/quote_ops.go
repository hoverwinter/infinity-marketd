package tdx

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

const defaultQuoteBatchSize = 80

type ServerProbeResult struct {
	Server    string `json:"server"`
	Success   bool   `json:"success"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
	Preferred bool   `json:"preferred,omitempty"`
}

type Security struct {
	Market       string  `json:"market"`
	Symbol       string  `json:"symbol"`
	Name         string  `json:"name,omitempty"`
	VolUnit      uint16  `json:"volunit"`
	DecimalPoint byte    `json:"decimal_point"`
	PreClose     float64 `json:"pre_close"`
}

type QuoteSession struct {
	server  string
	timeout time.Duration
	conn    net.Conn
	client  quoteConn
}

func NormalizeHQServers(opts QuoteClientOptions) []string {
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
		servers = append(servers, DefaultHQServers...)
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

func ProbeHQServers(ctx context.Context, servers []string, opts QuoteClientOptions) []ServerProbeResult {
	if len(servers) == 0 {
		servers = NormalizeHQServers(opts)
	}
	results := make([]ServerProbeResult, 0, len(servers))
	for _, server := range servers {
		started := time.Now()
		session, err := OpenQuoteSession(ctx, server, opts.Timeout)
		elapsed := time.Since(started)
		result := ServerProbeResult{
			Server:    server,
			Success:   err == nil,
			LatencyMS: elapsed.Milliseconds(),
		}
		if err != nil {
			result.Error = err.Error()
		} else {
			_ = session.Close()
		}
		results = append(results, result)
	}
	if best := BestHQServer(results); best != "" {
		for i := range results {
			if results[i].Server == best {
				results[i].Preferred = true
				break
			}
		}
	}
	return results
}

func BestHQServer(results []ServerProbeResult) string {
	bestIndex := -1
	for i, result := range results {
		if !result.Success {
			continue
		}
		if bestIndex < 0 || result.LatencyMS < results[bestIndex].LatencyMS {
			bestIndex = i
		}
	}
	if bestIndex < 0 {
		return ""
	}
	return results[bestIndex].Server
}

func OpenQuoteSession(ctx context.Context, server string, timeout time.Duration) (*QuoteSession, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return nil, fmt.Errorf("TDX HQ server is required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", server)
	if err != nil {
		return nil, fmt.Errorf("connect TDX HQ server %s: %w", server, err)
	}
	session := &QuoteSession{
		server:  server,
		timeout: timeout,
		conn:    conn,
		client:  quoteConn{rw: conn},
	}
	for _, packet := range hqSetupPackets {
		if _, err := session.call(packet); err != nil {
			_ = session.Close()
			return nil, fmt.Errorf("TDX HQ setup %s: %w", server, err)
		}
	}
	return session, nil
}

func (s *QuoteSession) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func (s *QuoteSession) Fetch(requests []QuoteRequest) ([]Quote, error) {
	if len(requests) == 0 {
		return nil, fmt.Errorf("at least one symbol is required")
	}
	body, err := s.call(BuildQuoteRequestPacket(requests))
	if err != nil {
		return nil, fmt.Errorf("TDX HQ quote request %s: %w", s.server, err)
	}
	quotes, err := DecodeQuoteResponse(body)
	if err != nil {
		return nil, fmt.Errorf("decode TDX HQ quote response from %s: %w", s.server, err)
	}
	if err := validateQuoteResponseMatchesRequest(requests, quotes); err != nil {
		return nil, fmt.Errorf("TDX HQ quote response from %s: %w", s.server, err)
	}
	return quotes, nil
}

func (s *QuoteSession) SecurityCount(market string) (int, error) {
	body, err := s.call(BuildSecurityCountPacket(market))
	if err != nil {
		return 0, fmt.Errorf("TDX HQ security count %s: %w", s.server, err)
	}
	return DecodeSecurityCountResponse(body)
}

func (s *QuoteSession) SecurityList(market string, start int) ([]Security, error) {
	body, err := s.call(BuildSecurityListPacket(market, start))
	if err != nil {
		return nil, fmt.Errorf("TDX HQ security list %s: %w", s.server, err)
	}
	return DecodeSecurityListResponse(market, body)
}

func (s *QuoteSession) call(packet []byte) ([]byte, error) {
	if s.conn != nil && s.timeout > 0 {
		if err := s.conn.SetDeadline(time.Now().Add(s.timeout)); err != nil {
			return nil, fmt.Errorf("set TDX connection deadline: %w", err)
		}
	}
	return s.client.call(packet)
}

func FetchRealtimeQuoteBatches(ctx context.Context, requests []QuoteRequest, opts QuoteClientOptions) ([]Quote, error) {
	if len(requests) == 0 {
		return nil, fmt.Errorf("at least one symbol is required")
	}
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = defaultQuoteBatchSize
	}
	if containsBeijingQuote(requests) {
		batchSize = 1
	}
	batches := SplitQuoteRequests(requests, batchSize)
	var attempts []string
	for _, server := range NormalizeHQServers(opts) {
		session, err := OpenQuoteSession(ctx, server, opts.Timeout)
		if err != nil {
			attempts = append(attempts, err.Error())
			continue
		}
		quotes, fetchErr := fetchBatchesWithSession(session, batches, opts)
		_ = session.Close()
		if fetchErr != nil {
			attempts = append(attempts, fetchErr.Error())
			if isDecodeError(fetchErr) {
				return nil, fetchErr
			}
			continue
		}
		return quotes, nil
	}
	return nil, fmt.Errorf("TDX HQ quote request failed on %d server(s): %s", len(attempts), strings.Join(attempts, "; "))
}

func fetchBatchesWithSession(session *QuoteSession, batches [][]QuoteRequest, opts QuoteClientOptions) ([]Quote, error) {
	var quotes []Quote
	for _, batch := range batches {
		batchQuotes, err := session.Fetch(batch)
		if err != nil {
			return nil, err
		}
		applyQuoteDate(batchQuotes, opts.TradeDate, opts.Location)
		quotes = append(quotes, batchQuotes...)
	}
	return quotes, nil
}

func SplitQuoteRequests(requests []QuoteRequest, batchSize int) [][]QuoteRequest {
	if batchSize <= 0 {
		batchSize = defaultQuoteBatchSize
	}
	var batches [][]QuoteRequest
	for len(requests) > 0 {
		n := batchSize
		if len(requests) < n {
			n = len(requests)
		}
		batch := append([]QuoteRequest(nil), requests[:n]...)
		batches = append(batches, batch)
		requests = requests[n:]
	}
	return batches
}

func containsBeijingQuote(requests []QuoteRequest) bool {
	for _, request := range requests {
		if request.Market == "bj" {
			return true
		}
	}
	return false
}

func validateQuoteResponseMatchesRequest(requests []QuoteRequest, quotes []Quote) error {
	if len(quotes) != len(requests) {
		return fmt.Errorf("expected %d quote(s), got %d", len(requests), len(quotes))
	}
	for i, req := range requests {
		if quotes[i].Market == req.Market && quotes[i].Symbol == req.Symbol {
			continue
		}
		return fmt.Errorf(
			"quote %d identity mismatch: requested %s:%s, got %s:%s",
			i,
			req.Market,
			req.Symbol,
			quotes[i].Market,
			quotes[i].Symbol,
		)
	}
	return nil
}

func FetchSecurityList(ctx context.Context, market string, opts QuoteClientOptions) ([]Security, error) {
	market = strings.ToLower(strings.TrimSpace(market))
	if market != "sh" && market != "sz" {
		return nil, fmt.Errorf("unsupported security-list market %q", market)
	}
	var attempts []string
	for _, server := range NormalizeHQServers(opts) {
		session, err := OpenQuoteSession(ctx, server, opts.Timeout)
		if err != nil {
			attempts = append(attempts, err.Error())
			continue
		}
		securities, fetchErr := fetchSecurityListWithSession(session, market)
		_ = session.Close()
		if fetchErr != nil {
			attempts = append(attempts, fetchErr.Error())
			continue
		}
		return securities, nil
	}
	return nil, fmt.Errorf("TDX HQ security list failed on %d server(s): %s", len(attempts), strings.Join(attempts, "; "))
}

func fetchSecurityListWithSession(session *QuoteSession, market string) ([]Security, error) {
	count, err := session.SecurityCount(market)
	if err != nil {
		return nil, err
	}
	var securities []Security
	for start := 0; start < count; {
		page, err := session.SecurityList(market, start)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		securities = append(securities, page...)
		start += len(page)
	}
	if len(securities) > count {
		securities = securities[:count]
	}
	return securities, nil
}

func BuildSecurityCountPacket(market string) []byte {
	packet := []byte{0x0c, 0x0c, 0x18, 0x6c, 0x00, 0x01, 0x08, 0x00, 0x08, 0x00, 0x4e, 0x04}
	packet = binary.LittleEndian.AppendUint16(packet, uint16(marketCodeForStandardHQ(market)))
	packet = append(packet, 0x75, 0xc7, 0x33, 0x01)
	return packet
}

func BuildSecurityListPacket(market string, start int) []byte {
	packet := []byte{0x0c, 0x01, 0x18, 0x64, 0x01, 0x01, 0x06, 0x00, 0x06, 0x00, 0x50, 0x04}
	packet = binary.LittleEndian.AppendUint16(packet, uint16(marketCodeForStandardHQ(market)))
	packet = binary.LittleEndian.AppendUint16(packet, uint16(start))
	return packet
}

func DecodeSecurityCountResponse(body []byte) (int, error) {
	if len(body) < 2 {
		return 0, fmt.Errorf("TDX security count response too short: %d bytes", len(body))
	}
	return int(binary.LittleEndian.Uint16(body[:2])), nil
}

func DecodeSecurityListResponse(market string, body []byte) ([]Security, error) {
	if len(body) < 2 {
		return nil, fmt.Errorf("TDX security list response too short: %d bytes", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[:2]))
	pos := 2
	securities := make([]Security, 0, count)
	for i := 0; i < count; i++ {
		if pos+29 > len(body) {
			return nil, fmt.Errorf("TDX security list response truncated at item %d", i)
		}
		record := body[pos : pos+29]
		code := strings.TrimRight(string(record[0:6]), "\x00")
		name := decodeSecurityName(record[8:16])
		preClose := decodeTDXFloat(binary.LittleEndian.Uint32(record[21:25]))
		if len(code) == 6 && allDigits(code) {
			securities = append(securities, Security{
				Market:       market,
				Symbol:       code,
				Name:         name,
				VolUnit:      binary.LittleEndian.Uint16(record[6:8]),
				DecimalPoint: record[20],
				PreClose:     preClose,
			})
		}
		pos += 29
	}
	return securities, nil
}

func decodeSecurityName(raw []byte) string {
	raw = bytesTrimRight(raw, 0x00, ' ')
	if len(raw) == 0 {
		return ""
	}
	name, err := simplifiedchinese.GB18030.NewDecoder().String(string(raw))
	if err != nil {
		return ""
	}
	if strings.ContainsRune(name, '\uFFFD') {
		return ""
	}
	return strings.TrimRight(name, " ")
}

func bytesTrimRight(raw []byte, cutset ...byte) []byte {
	end := len(raw)
	for end > 0 {
		match := false
		for _, b := range cutset {
			if raw[end-1] == b {
				match = true
				break
			}
		}
		if !match {
			break
		}
		end--
	}
	return raw[:end]
}

func QuoteSweep(ctx context.Context, opts QuoteSweepOptions) ([]Quote, error) {
	requests := append([]QuoteRequest(nil), opts.Requests...)
	if len(requests) == 0 {
		markets := opts.Markets
		if len(markets) == 0 {
			markets = []string{"sh", "sz"}
		}
		for _, market := range markets {
			securities, err := FetchSecurityList(ctx, market, opts.Client)
			if err != nil {
				return nil, err
			}
			for _, security := range securities {
				requests = append(requests, QuoteRequest{Market: security.Market, Symbol: security.Symbol})
				if opts.Limit > 0 && len(requests) >= opts.Limit {
					break
				}
			}
			if opts.Limit > 0 && len(requests) >= opts.Limit {
				break
			}
		}
	}
	if opts.Limit > 0 && len(requests) > opts.Limit {
		requests = requests[:opts.Limit]
	}
	return FetchRealtimeQuoteBatches(ctx, requests, opts.Client)
}

type QuoteSweepOptions struct {
	Requests []QuoteRequest
	Markets  []string
	Limit    int
	Client   QuoteClientOptions
}

func applyQuoteDate(quotes []Quote, tradeDate time.Time, loc *time.Location) {
	if tradeDate.IsZero() {
		return
	}
	if loc == nil {
		loc = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	date := tradeDate.In(loc)
	dateText := date.Format("2006-01-02")
	for i := range quotes {
		quotes[i].TradeDate = dateText
		if quotes[i].ServerIntradayTime != "" {
			quotes[i].QuoteTime = dateText + " " + quotes[i].ServerIntradayTime
		}
	}
}

func marketCodeForStandardHQ(market string) int {
	switch strings.ToLower(strings.TrimSpace(market)) {
	case "sh":
		return tdxMarketSH
	case "bj":
		return tdxMarketBJ
	default:
		return tdxMarketSZ
	}
}

func isDecodeError(err error) bool {
	return strings.Contains(err.Error(), "decode TDX HQ quote response")
}

func SortProbeResults(results []ServerProbeResult) {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Success != results[j].Success {
			return results[i].Success
		}
		return results[i].LatencyMS < results[j].LatencyMS
	})
}

var _ io.Closer = (*QuoteSession)(nil)
