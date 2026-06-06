package tdx

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestParseQuoteRequest(t *testing.T) {
	tests := []struct {
		input  string
		market string
		symbol string
	}{
		{input: "600519", market: "sh", symbol: "600519"},
		{input: "000001", market: "sz", symbol: "000001"},
		{input: "920001", market: "bj", symbol: "920001"},
		{input: "830001", market: "bj", symbol: "830001"},
		{input: "430001", market: "bj", symbol: "430001"},
		{input: "sh:600000", market: "sh", symbol: "600000"},
		{input: "sz:300750", market: "sz", symbol: "300750"},
		{input: "bj:920001", market: "bj", symbol: "920001"},
	}
	for _, tt := range tests {
		got, err := ParseQuoteRequest(tt.input)
		if err != nil {
			t.Fatalf("ParseQuoteRequest(%q): %v", tt.input, err)
		}
		if got.Market != tt.market || got.Symbol != tt.symbol {
			t.Fatalf("ParseQuoteRequest(%q) = %#v", tt.input, got)
		}
	}
}

func TestParseQuoteRequestRejectsUnsupportedMarket(t *testing.T) {
	if _, err := ParseQuoteRequest("hk:00700"); err == nil {
		t.Fatal("expected unsupported market error")
	}
	if _, err := ParseQuoteRequest("60051"); err == nil {
		t.Fatal("expected bad symbol error")
	}
}

func TestBuildQuoteRequestPacket(t *testing.T) {
	packet := BuildQuoteRequestPacket([]QuoteRequest{
		{Market: "sz", Symbol: "000001"},
		{Market: "sh", Symbol: "600519"},
		{Market: "bj", Symbol: "920001"},
	})
	if len(packet) != 43 {
		t.Fatalf("packet len = %d", len(packet))
	}
	if got := binary.LittleEndian.Uint16(packet[0:2]); got != 0x010c {
		t.Fatalf("command = %#x", got)
	}
	if got := binary.LittleEndian.Uint32(packet[2:6]); got != 0x02006320 {
		t.Fatalf("magic = %#x", got)
	}
	if got := binary.LittleEndian.Uint16(packet[6:8]); got != 33 {
		t.Fatalf("data len = %d", got)
	}
	if got := binary.LittleEndian.Uint16(packet[20:22]); got != 3 {
		t.Fatalf("stock count = %d", got)
	}
	wantTail := []byte{
		0, '0', '0', '0', '0', '0', '1',
		1, '6', '0', '0', '5', '1', '9',
		2, '9', '2', '0', '0', '0', '1',
	}
	if string(packet[22:]) != string(wantTail) {
		t.Fatalf("tail = %v", packet[22:])
	}
}

func TestTDXVarInt(t *testing.T) {
	values := []int{0, 1, -1, 63, 64, -145, 12345, -12345}
	for _, value := range values {
		encoded := encodeTDXVarInt(value)
		got, pos, err := readTDXVarInt(encoded, 0)
		if err != nil {
			t.Fatalf("readTDXVarInt(%d): %v", value, err)
		}
		if got != value || pos != len(encoded) {
			t.Fatalf("readTDXVarInt(%d) = %d pos %d encoded %v", value, got, pos, encoded)
		}
	}
}

func TestDecodeQuoteResponse(t *testing.T) {
	body := quoteResponseBody()
	quotes, err := DecodeQuoteResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) != 1 {
		t.Fatalf("quotes = %d", len(quotes))
	}
	q := quotes[0]
	if q.Market != "sh" || q.Symbol != "600519" {
		t.Fatalf("identity = %#v", q)
	}
	if q.Price != 123.45 || q.LastClose != 123.00 || q.Open != 123.40 || q.High != 124.00 || q.Low != 122.00 {
		t.Fatalf("prices = %#v", q)
	}
	if q.ServerTime != "9:30:00.000" {
		t.Fatalf("server time = %q", q.ServerTime)
	}
	if q.Volume != 10000 || q.CurrentVol != 100 || q.SellVolume != 4000 || q.BuyVolume != 6000 {
		t.Fatalf("volumes = %#v", q)
	}
	if len(q.Bids) != 5 || len(q.Asks) != 5 {
		t.Fatalf("depth sizes bids=%d asks=%d", len(q.Bids), len(q.Asks))
	}
	if q.Bids[0].Price != 123.44 || q.Bids[0].Volume != 100 || q.Asks[0].Price != 123.46 || q.Asks[0].Volume != 120 {
		t.Fatalf("level1 bids=%#v asks=%#v", q.Bids[0], q.Asks[0])
	}
	if math.IsNaN(q.Amount) {
		t.Fatal("amount is NaN")
	}
}

func TestDecodeQuoteResponseBeijing(t *testing.T) {
	body := quoteResponseBodyFor(tdxMarketBJ, "920001")
	quotes, err := DecodeQuoteResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) != 1 {
		t.Fatalf("quotes = %d", len(quotes))
	}
	if quotes[0].Market != "bj" || quotes[0].Symbol != "920001" || quotes[0].Price != 123.45 {
		t.Fatalf("quote = %#v", quotes[0])
	}
}

func TestFormatTDXQuoteTime(t *testing.T) {
	if got := formatTDXQuoteTime(9300000); got != "9:30:00.000" {
		t.Fatalf("time = %q", got)
	}
}

func quoteResponseBody() []byte {
	return quoteResponseBodyFor(tdxMarketSH, "600519")
}

func quoteResponseBodyFor(market int, code string) []byte {
	body := []byte{0xb1, 0xcb, 1, 0}
	body = append(body, byte(market))
	rawCode := make([]byte, 6)
	copy(rawCode, code)
	body = append(body, rawCode...)
	body = append(body, 0, 0)
	body = append(body, encodeTDXVarInt(12345)...)
	body = append(body, encodeTDXVarInt(-45)...)
	body = append(body, encodeTDXVarInt(-5)...)
	body = append(body, encodeTDXVarInt(55)...)
	body = append(body, encodeTDXVarInt(-145)...)
	body = append(body, encodeTDXVarInt(9300000)...)
	body = append(body, encodeTDXVarInt(-12345)...)
	body = append(body, encodeTDXVarInt(10000)...)
	body = append(body, encodeTDXVarInt(100)...)
	body = append(body, 0, 0, 0, 0)
	body = append(body, encodeTDXVarInt(4000)...)
	body = append(body, encodeTDXVarInt(6000)...)
	body = append(body, encodeTDXVarInt(0)...)
	body = append(body, encodeTDXVarInt(0)...)
	for i := 0; i < 5; i++ {
		body = append(body, encodeTDXVarInt(-1-i)...)
		body = append(body, encodeTDXVarInt(1+i)...)
		body = append(body, encodeTDXVarInt(100+i)...)
		body = append(body, encodeTDXVarInt(120+i)...)
	}
	body = append(body, 0, 0)
	for i := 0; i < 4; i++ {
		body = append(body, encodeTDXVarInt(0)...)
	}
	body = append(body, 0, 0, 0, 0)
	return body
}

func encodeTDXVarInt(value int) []byte {
	negative := value < 0
	if negative {
		value = -value
	}
	first := byte(value & 0x3f)
	if negative {
		first |= 0x40
	}
	value >>= 6
	if value == 0 {
		return []byte{first}
	}
	out := []byte{first | 0x80}
	for value > 0 {
		b := byte(value & 0x7f)
		value >>= 7
		if value > 0 {
			b |= 0x80
		}
		out = append(out, b)
	}
	return out
}
