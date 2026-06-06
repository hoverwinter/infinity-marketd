package tdx

import (
	"context"
	"encoding/binary"
	"math"
	"testing"
)

func TestParseExQuoteRequest(t *testing.T) {
	req, err := ParseExQuoteRequest(47, " IF1709 ")
	if err != nil {
		t.Fatal(err)
	}
	if req.Market != 47 || req.Code != "IF1709" {
		t.Fatalf("request = %#v", req)
	}

	tests := []struct {
		name   string
		market int
		code   string
	}{
		{name: "zero market", market: 0, code: "IF1709"},
		{name: "large market", market: 256, code: "IF1709"},
		{name: "empty code", market: 47, code: ""},
		{name: "long code", market: 47, code: "1234567890"},
		{name: "space code", market: 47, code: "IF 1709"},
		{name: "colon code", market: 47, code: "47:IF1709"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseExQuoteRequest(tt.market, tt.code); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestBuildExMarketListPacket(t *testing.T) {
	packet := BuildExMarketListPacket()
	want := []byte{0x01, 0x02, 0x48, 0x69, 0x00, 0x01, 0x02, 0x00, 0x02, 0x00, 0xf4, 0x23}
	if string(packet) != string(want) {
		t.Fatalf("packet = %v", packet)
	}
}

func TestDecodeExMarketListResponse(t *testing.T) {
	body := exMarketListBody([]exMarketRecord{
		{Market: 47, Category: 3, Name: "Futures", ShortName: "CZ"},
		{Market: 0, Category: 0, Name: "skip", ShortName: "NO"},
	})
	markets, err := DecodeExMarketListResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(markets) != 1 {
		t.Fatalf("markets = %#v", markets)
	}
	if markets[0] != (ExMarket{Market: 47, Category: 3, Name: "Futures", ShortName: "CZ"}) {
		t.Fatalf("market = %#v", markets[0])
	}
}

func TestBuildExQuoteRequestPacket(t *testing.T) {
	req := ExQuoteRequest{Market: 47, Code: "IF1709"}
	packet := BuildExQuoteRequestPacket(req)
	if len(packet) != 22 {
		t.Fatalf("packet len = %d", len(packet))
	}
	wantPrefix := []byte{0x01, 0x01, 0x08, 0x02, 0x02, 0x01, 0x0c, 0x00, 0x0c, 0x00, 0xfa, 0x23}
	if string(packet[:12]) != string(wantPrefix) {
		t.Fatalf("prefix = %v", packet[:12])
	}
	if packet[12] != 47 {
		t.Fatalf("market = %d", packet[12])
	}
	if got := string(packet[13:19]); got != "IF1709" {
		t.Fatalf("code = %q", got)
	}
	for i, b := range packet[19:22] {
		if b != 0 {
			t.Fatalf("padding byte %d = %d", i, b)
		}
	}
}

func TestDecodeExQuoteResponse(t *testing.T) {
	quote, err := DecodeExQuoteResponse(exQuoteResponseBody())
	if err != nil {
		t.Fatal(err)
	}
	if quote.Market != 47 || quote.Code != "IF1709" {
		t.Fatalf("identity = %#v", quote)
	}
	if !floatNear(quote.PreClose, 3718.2) || !floatNear(quote.Open, 3717.2) || !floatNear(quote.High, 3724) || !floatNear(quote.Low, 3696.6) || !floatNear(quote.Price, 3703) {
		t.Fatalf("prices = %#v", quote)
	}
	if quote.KaiCang != 2043 || quote.ZongLiang != 1728 || quote.XianLiang != 3 || quote.NeiPan != 869 || quote.WaiPan != 859 || quote.ChiCang != 13340 {
		t.Fatalf("volumes = %#v", quote)
	}
	if len(quote.Bids) != 5 || len(quote.Asks) != 5 {
		t.Fatalf("depth = %#v %#v", quote.Bids, quote.Asks)
	}
	if !floatNear(quote.Bids[0].Price, 3702.8) || quote.Bids[0].Volume != 1 || !floatNear(quote.Asks[0].Price, 3704.4) || quote.Asks[0].Volume != 1 {
		t.Fatalf("level1 bids=%#v asks=%#v", quote.Bids[0], quote.Asks[0])
	}
}

func TestFetchExQuoteUsesExHQSetupAndRequest(t *testing.T) {
	req := ExQuoteRequest{Market: 47, Code: "IF1709"}
	server := startScriptedHQServer(t, []scriptStep{
		{ReadLen: len(exHQSetupPacket), Body: []byte{0}},
		{ReadLen: len(BuildExQuoteRequestPacket(req)), Body: exQuoteResponseBody()},
	})
	quote, err := FetchExQuote(context.Background(), req, ExQuoteClientOptions{Server: server})
	if err != nil {
		t.Fatal(err)
	}
	if quote.Market != 47 || quote.Code != "IF1709" {
		t.Fatalf("quote = %#v", quote)
	}
}

func TestFetchExMarketsUsesExHQSetupAndRequest(t *testing.T) {
	server := startScriptedHQServer(t, []scriptStep{
		{ReadLen: len(exHQSetupPacket), Body: []byte{0}},
		{ReadLen: len(BuildExMarketListPacket()), Body: exMarketListBody([]exMarketRecord{{Market: 47, Category: 3, Name: "Futures", ShortName: "CZ"}})},
	})
	markets, err := FetchExMarkets(context.Background(), ExQuoteClientOptions{Server: server})
	if err != nil {
		t.Fatal(err)
	}
	if len(markets) != 1 || markets[0].Market != 47 {
		t.Fatalf("markets = %#v", markets)
	}
}

type exMarketRecord struct {
	Market    byte
	Category  byte
	Name      string
	ShortName string
}

func exMarketListBody(records []exMarketRecord) []byte {
	body := make([]byte, 2)
	binary.LittleEndian.PutUint16(body, uint16(len(records)))
	for _, item := range records {
		record := make([]byte, 64)
		record[0] = item.Category
		copy(record[1:33], item.Name)
		record[33] = item.Market
		copy(record[34:36], item.ShortName)
		body = append(body, record...)
	}
	return body
}

func exQuoteResponseBody() []byte {
	body := make([]byte, 0, 150)
	body = append(body, 47)
	code := make([]byte, 9)
	copy(code, "IF1709")
	body = append(body, code...)
	body = append(body, 0, 0, 0, 0)

	for _, value := range []float32{3718.2, 3717.2, 3724, 3696.6, 3703} {
		body = appendExFloat32(body, value)
	}
	for _, value := range []uint32{2043, 0, 1728, 3, 0, 869, 859, 0, 13340} {
		body = binary.LittleEndian.AppendUint32(body, value)
	}
	for _, value := range []float32{3702.8, 0, 0, 0, 0} {
		body = appendExFloat32(body, value)
	}
	for _, value := range []uint32{1, 0, 0, 0, 0} {
		body = binary.LittleEndian.AppendUint32(body, value)
	}
	for _, value := range []float32{3704.4, 0, 0, 0, 0} {
		body = appendExFloat32(body, value)
	}
	for _, value := range []uint32{1, 0, 0, 0, 0} {
		body = binary.LittleEndian.AppendUint32(body, value)
	}
	return body
}

func appendExFloat32(body []byte, value float32) []byte {
	return binary.LittleEndian.AppendUint32(body, math.Float32bits(value))
}

func floatNear(got, want float64) bool {
	return math.Abs(got-want) < 0.001
}
