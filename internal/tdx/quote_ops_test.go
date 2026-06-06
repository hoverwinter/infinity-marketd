package tdx

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestProbeHQServersSelectsBestReachableServer(t *testing.T) {
	fast := startScriptedHQServer(t, setupSteps())
	bad := startClosingHQServer(t)

	results := ProbeHQServers(context.Background(), []string{bad, fast}, QuoteClientOptions{Timeout: time.Second})
	if len(results) != 2 {
		t.Fatalf("results = %#v", results)
	}
	if best := BestHQServer(results); best != fast {
		t.Fatalf("best = %q results=%#v", best, results)
	}
	foundFailure := false
	foundPreferred := false
	for _, result := range results {
		if !result.Success && result.Error != "" {
			foundFailure = true
		}
		if result.Server == fast && result.Preferred {
			foundPreferred = true
		}
	}
	if !foundFailure || !foundPreferred {
		t.Fatalf("results = %#v", results)
	}
}

func TestFetchRealtimeQuotesRetriesAcrossServers(t *testing.T) {
	bad := startClosingHQServer(t)
	request := QuoteRequest{Market: "sh", Symbol: "600519"}
	good := startScriptedHQServer(t, append(setupSteps(), scriptStep{
		ReadLen: len(BuildQuoteRequestPacket([]QuoteRequest{request})),
		Body:    quoteResponseBody(),
	}))

	quotes, err := FetchRealtimeQuotes(context.Background(), []QuoteRequest{request}, QuoteClientOptions{
		Servers: []string{bad, good},
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) != 1 || quotes[0].Symbol != "600519" {
		t.Fatalf("quotes = %#v", quotes)
	}
}

func TestFetchRealtimeQuotesAddsExplicitTradeDate(t *testing.T) {
	request := QuoteRequest{Market: "sh", Symbol: "600519"}
	server := startScriptedHQServer(t, append(setupSteps(), scriptStep{
		ReadLen: len(BuildQuoteRequestPacket([]QuoteRequest{request})),
		Body:    quoteResponseBody(),
	}))
	loc := mustShanghai(t)
	tradeDate := time.Date(2026, 6, 5, 0, 0, 0, 0, loc)
	quotes, err := FetchRealtimeQuotes(context.Background(), []QuoteRequest{request}, QuoteClientOptions{
		Server:    server,
		Timeout:   time.Second,
		TradeDate: tradeDate,
		Location:  loc,
	})
	if err != nil {
		t.Fatal(err)
	}
	if quotes[0].TradeDate != "2026-06-05" || quotes[0].QuoteTime != "2026-06-05 9:30:00.000" {
		t.Fatalf("quote time fields = %#v", quotes[0])
	}
}

func TestFetchRealtimeQuotesSupportsBeijingMarket(t *testing.T) {
	request := QuoteRequest{Market: "bj", Symbol: "920001"}
	server := startScriptedHQServer(t, append(setupSteps(), scriptStep{
		ReadLen: len(BuildQuoteRequestPacket([]QuoteRequest{request})),
		Body:    quoteResponseBodyFor(tdxMarketBJ, "920001"),
	}))
	quotes, err := FetchRealtimeQuotes(context.Background(), []QuoteRequest{request}, QuoteClientOptions{
		Server:  server,
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) != 1 || quotes[0].Market != "bj" || quotes[0].Symbol != "920001" {
		t.Fatalf("quotes = %#v", quotes)
	}
}

func TestFetchRealtimeQuotesSplitsBeijingRequestsSingly(t *testing.T) {
	requests := []QuoteRequest{
		{Market: "bj", Symbol: "920001"},
		{Market: "bj", Symbol: "920799"},
	}
	server := startScriptedHQServer(t, append(setupSteps(),
		scriptStep{
			ReadLen: len(BuildQuoteRequestPacket(requests[:1])),
			Body:    quoteResponseBodyFor(tdxMarketBJ, "920001"),
		},
		scriptStep{
			ReadLen: len(BuildQuoteRequestPacket(requests[1:])),
			Body:    quoteResponseBodyFor(tdxMarketBJ, "920799"),
		},
	))
	quotes, err := FetchRealtimeQuotes(context.Background(), requests, QuoteClientOptions{
		Server:    server,
		Timeout:   time.Second,
		BatchSize: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) != 2 || quotes[0].Symbol != "920001" || quotes[1].Symbol != "920799" {
		t.Fatalf("quotes = %#v", quotes)
	}
}

func TestFetchRealtimeQuotesRejectsMismatchedResponse(t *testing.T) {
	request := QuoteRequest{Market: "bj", Symbol: "920001"}
	server := startScriptedHQServer(t, append(setupSteps(), scriptStep{
		ReadLen: len(BuildQuoteRequestPacket([]QuoteRequest{request})),
		Body:    quoteResponseBody(),
	}))
	_, err := FetchRealtimeQuotes(context.Background(), []QuoteRequest{request}, QuoteClientOptions{
		Server:  server,
		Timeout: time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("err = %v", err)
	}
}

func TestSplitQuoteRequests(t *testing.T) {
	requests := []QuoteRequest{
		{Market: "sh", Symbol: "600001"},
		{Market: "sh", Symbol: "600002"},
		{Market: "sz", Symbol: "000001"},
	}
	batches := SplitQuoteRequests(requests, 2)
	if len(batches) != 2 || len(batches[0]) != 2 || len(batches[1]) != 1 {
		t.Fatalf("batches = %#v", batches)
	}
	requests[0].Symbol = "changed"
	if batches[0][0].Symbol != "600001" {
		t.Fatalf("batch was not copied: %#v", batches)
	}
}

func TestDecodeSecurityListResponse(t *testing.T) {
	body := securityListBody([]securityRecord{
		{Code: "600519", Name: "MOUTAI", VolUnit: 100, DecimalPoint: 2},
		{Code: "bad", Name: "BAD", VolUnit: 100, DecimalPoint: 2},
	})
	securities, err := DecodeSecurityListResponse("sh", body)
	if err != nil {
		t.Fatal(err)
	}
	if len(securities) != 1 {
		t.Fatalf("securities = %#v", securities)
	}
	if securities[0].Market != "sh" || securities[0].Symbol != "600519" || securities[0].Name != "MOUTAI" {
		t.Fatalf("security = %#v", securities[0])
	}
}

func TestDecodeSecurityListResponseDecodesGB18030Name(t *testing.T) {
	body := securityListBody([]securityRecord{
		{
			Code:         "600519",
			NameBytes:    []byte{0xb9, 0xf3, 0xd6, 0xdd, 0xc3, 0xa9, 0xcc, 0xa8},
			VolUnit:      100,
			DecimalPoint: 2,
		},
	})
	securities, err := DecodeSecurityListResponse("sh", body)
	if err != nil {
		t.Fatal(err)
	}
	if len(securities) != 1 {
		t.Fatalf("securities = %#v", securities)
	}
	if securities[0].Name != "贵州茅台" {
		t.Fatalf("name = %q", securities[0].Name)
	}
}

func TestDecodeSecurityListResponseTrimsNamePadding(t *testing.T) {
	body := securityListBody([]securityRecord{
		{Code: "600519", Name: "ABC", VolUnit: 100, DecimalPoint: 2},
		{Code: "600520", NameBytes: []byte{'A', 'B', 'C', ' ', ' ', ' ', ' ', ' '}, VolUnit: 100, DecimalPoint: 2},
	})
	securities, err := DecodeSecurityListResponse("sh", body)
	if err != nil {
		t.Fatal(err)
	}
	if len(securities) != 2 {
		t.Fatalf("securities = %#v", securities)
	}
	if securities[0].Name != "ABC" || securities[1].Name != "ABC" {
		t.Fatalf("names = %#v", securities)
	}
}

func TestDecodeSecurityListResponseToleratesMalformedName(t *testing.T) {
	body := securityListBody([]securityRecord{
		{Code: "600519", NameBytes: []byte{0xff, 0xff, 0xff, 0xff}, VolUnit: 100, DecimalPoint: 2},
	})
	securities, err := DecodeSecurityListResponse("sh", body)
	if err != nil {
		t.Fatal(err)
	}
	if len(securities) != 1 {
		t.Fatalf("securities = %#v", securities)
	}
	if securities[0].Symbol != "600519" || securities[0].Name != "" {
		t.Fatalf("security = %#v", securities[0])
	}
}

func TestFetchSecurityListRejectsBeijingDiscovery(t *testing.T) {
	_, err := FetchSecurityList(context.Background(), "bj", QuoteClientOptions{})
	if err == nil || !strings.Contains(err.Error(), "unsupported security-list market") {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchSecurityListUsesCountAndPages(t *testing.T) {
	server := startScriptedHQServer(t, append(setupSteps(),
		scriptStep{ReadLen: len(BuildSecurityCountPacket("sh")), Body: countBody(1)},
		scriptStep{ReadLen: len(BuildSecurityListPacket("sh", 0)), Body: securityListBody([]securityRecord{{Code: "600519", Name: "MOUTAI", VolUnit: 100, DecimalPoint: 2}})},
	))
	securities, err := FetchSecurityList(context.Background(), "sh", QuoteClientOptions{Server: server, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if len(securities) != 1 || securities[0].Symbol != "600519" {
		t.Fatalf("securities = %#v", securities)
	}
}

func TestQuoteSweepWithExplicitBeijingSymbolUsesBatchWorkflow(t *testing.T) {
	request := QuoteRequest{Market: "bj", Symbol: "920001"}
	server := startScriptedHQServer(t, append(setupSteps(), scriptStep{
		ReadLen: len(BuildQuoteRequestPacket([]QuoteRequest{request})),
		Body:    quoteResponseBodyFor(tdxMarketBJ, "920001"),
	}))
	quotes, err := QuoteSweep(context.Background(), QuoteSweepOptions{
		Requests: []QuoteRequest{request},
		Client: QuoteClientOptions{
			Server:  server,
			Timeout: time.Second,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) != 1 || quotes[0].Market != "bj" || quotes[0].Symbol != "920001" {
		t.Fatalf("quotes = %#v", quotes)
	}
}

func TestQuoteSweepRejectsBeijingDiscovery(t *testing.T) {
	_, err := QuoteSweep(context.Background(), QuoteSweepOptions{Markets: []string{"bj"}})
	if err == nil || !strings.Contains(err.Error(), "unsupported security-list market") {
		t.Fatalf("err = %v", err)
	}
}

func TestQuoteSweepWithExplicitSymbolsUsesBatchWorkflow(t *testing.T) {
	requests := []QuoteRequest{
		{Market: "sh", Symbol: "600519"},
		{Market: "sh", Symbol: "600000"},
	}
	server := startScriptedHQServer(t, append(setupSteps(),
		scriptStep{ReadLen: len(BuildQuoteRequestPacket(requests[:1])), Body: quoteResponseBody()},
		scriptStep{ReadLen: len(BuildQuoteRequestPacket(requests[1:])), Body: quoteResponseBodyFor(tdxMarketSH, "600000")},
	))
	quotes, err := QuoteSweep(context.Background(), QuoteSweepOptions{
		Requests: requests,
		Client: QuoteClientOptions{
			Server:    server,
			Timeout:   time.Second,
			BatchSize: 1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) != 2 {
		t.Fatalf("quotes = %#v", quotes)
	}
}

func TestExtendedQuoteValidationIsSeparate(t *testing.T) {
	if _, err := ParseQuoteRequest("47:IF1709"); err == nil {
		t.Fatal("standard quote parser accepted extended code")
	}
	req, err := ParseExQuoteRequest(47, "IF1709")
	if err != nil {
		t.Fatal(err)
	}
	if req.Market != 47 || req.Code != "IF1709" {
		t.Fatalf("request = %#v", req)
	}
}

type scriptStep struct {
	ReadLen int
	Body    []byte
}

func setupSteps() []scriptStep {
	return []scriptStep{
		{ReadLen: len(hqSetupPackets[0]), Body: []byte{0}},
		{ReadLen: len(hqSetupPackets[1]), Body: []byte{0}},
		{ReadLen: len(hqSetupPackets[2]), Body: []byte{0}},
	}
}

func startScriptedHQServer(t *testing.T, steps []scriptStep) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				for _, step := range steps {
					packet := make([]byte, step.ReadLen)
					if _, err := io.ReadFull(conn, packet); err != nil {
						return
					}
					if _, err := conn.Write(tdxResponse(step.Body)); err != nil {
						return
					}
				}
			}()
		}
	}()
	return ln.Addr().String()
}

func startClosingHQServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	return ln.Addr().String()
}

func tdxResponse(body []byte) []byte {
	header := make([]byte, 16)
	binary.LittleEndian.PutUint16(header[12:14], uint16(len(body)))
	binary.LittleEndian.PutUint16(header[14:16], uint16(len(body)))
	return append(header, body...)
}

func countBody(n int) []byte {
	body := make([]byte, 2)
	binary.LittleEndian.PutUint16(body, uint16(n))
	return body
}

type securityRecord struct {
	Code         string
	Name         string
	NameBytes    []byte
	VolUnit      uint16
	DecimalPoint byte
}

func securityListBody(records []securityRecord) []byte {
	body := make([]byte, 2)
	binary.LittleEndian.PutUint16(body[:2], uint16(len(records)))
	for _, record := range records {
		raw := make([]byte, 29)
		copy(raw[0:6], record.Code)
		binary.LittleEndian.PutUint16(raw[6:8], record.VolUnit)
		name := record.NameBytes
		if name == nil {
			name = []byte(strings.ToUpper(record.Name))
		}
		copy(raw[8:16], name)
		raw[20] = record.DecimalPoint
		body = append(body, raw...)
	}
	return body
}
