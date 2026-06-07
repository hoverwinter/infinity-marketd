package tdx

import (
	"encoding/binary"
	"math"
	"testing"
)

// encTDXVarInt is the inverse of readTDXVarInt, used to build decoder fixtures.
func encTDXVarInt(v int) []byte {
	neg := v < 0
	if neg {
		v = -v
	}
	first := byte(v & 0x3f)
	rem := v >> 6
	if neg {
		first |= 0x40
	}
	if rem > 0 {
		first |= 0x80
	}
	out := []byte{first}
	for rem > 0 {
		b := byte(rem & 0x7f)
		rem >>= 7
		if rem > 0 {
			b |= 0x80
		}
		out = append(out, b)
	}
	return out
}

func TestEncTDXVarIntRoundTrip(t *testing.T) {
	for _, v := range []int{0, 1, 54, 286, 127286, -512, -1, 65535, 1 << 20} {
		buf := encTDXVarInt(v)
		got, pos, err := readTDXVarInt(buf, 0)
		if err != nil || got != v || pos != len(buf) {
			t.Fatalf("varint %d: got %d pos %d err %v (buf %x)", v, got, pos, err, buf)
		}
	}
}

func TestBuildHQQuotesListPacketHeader(t *testing.T) {
	pkt := BuildHQQuotesListPacket(HQQuotesListRequest{Category: 0, SortType: QuotesSortChangePct, Start: 0, Count: 50})
	if pkt[0] != 0x0c {
		t.Fatalf("magic = %#x", pkt[0])
	}
	if cmd := binary.LittleEndian.Uint16(pkt[10:12]); cmd != cmdQuotesList {
		t.Fatalf("cmd = %#x", cmd)
	}
	if l := binary.LittleEndian.Uint16(pkt[6:8]); l != 18+2 {
		t.Fatalf("len = %d", l)
	}
	if len(pkt) != 12+18 {
		t.Fatalf("packet len = %d", len(pkt))
	}
}

func TestDecodeHQQuotesListResponse(t *testing.T) {
	var body []byte
	body = append(body, 0x00, 0x00)                  // block
	body = binary.LittleEndian.AppendUint16(body, 1) // count = 1

	body = append(body, 1)                           // market = sh
	body = append(body, []byte("600519")...)         // code
	body = binary.LittleEndian.AppendUint16(body, 7) // active

	for _, v := range []int{127286, -286, 714, 1014, -512, 0, 0, 31303, 560} {
		body = append(body, encTDXVarInt(v)...)
	}
	body = binary.LittleEndian.AppendUint32(body, math.Float32bits(3.98e9)) // amount
	for i := 0; i < 8; i++ {
		body = append(body, encTDXVarInt(0)...)
	}
	tail := make([]byte, 56)
	binary.LittleEndian.PutUint16(tail[2:4], uint16(int16(123))) // rise_speed 1.23
	body = append(body, tail...)

	items, err := DecodeHQQuotesListResponse(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	it := items[0]
	if it.Market != "sh" || it.Symbol != "600519" || it.MarketCode != 1 || it.Active != 7 {
		t.Fatalf("identity wrong: %+v", it)
	}
	if it.Price != 1272.86 || it.Open != 1280 || it.High != 1283 || it.Low != 1267.74 || it.PreClose != 1270 {
		t.Fatalf("prices wrong: %+v", it)
	}
	if it.Volume != 31303 || it.CurrentVolume != 560 {
		t.Fatalf("volumes wrong: %+v", it)
	}
	if it.RiseSpeed != 1.23 {
		t.Fatalf("rise speed = %v", it.RiseSpeed)
	}
	if it.Amount != float64(float32(3.98e9)) {
		t.Fatalf("amount = %v", it.Amount)
	}
}

func TestDecodeHQTopBoardResponse(t *testing.T) {
	size := 1
	body := []byte{byte(size)}
	for g := 0; g < 9; g++ {
		body = append(body, 1)                   // market sh
		body = append(body, []byte("600000")...) // code
		body = binary.LittleEndian.AppendUint32(body, math.Float32bits(float32(10+g)))
		body = binary.LittleEndian.AppendUint32(body, math.Float32bits(float32(g)))
	}
	groups, err := DecodeHQTopBoardResponse(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(groups) != 9 {
		t.Fatalf("expected 9 groups, got %d", len(groups))
	}
	if groups[0].Name != "gainers" || groups[8].Name != "turnover" {
		t.Fatalf("group names wrong: %s..%s", groups[0].Name, groups[8].Name)
	}
	for g, grp := range groups {
		if grp.GroupID != g || len(grp.Items) != 1 {
			t.Fatalf("group %d wrong: %+v", g, grp)
		}
		it := grp.Items[0]
		if it.Symbol != "600000" || it.Market != "sh" || it.Price != float64(float32(10+g)) || it.Value != float64(float32(g)) {
			t.Fatalf("group %d item wrong: %+v", g, it)
		}
	}
}

func TestFetchHQQuotesListValidation(t *testing.T) {
	for _, req := range []HQQuotesListRequest{
		{Count: 0},
		{Start: -1, Count: 10},
		{Count: maxQuotesListCount + 1},
	} {
		if _, err := FetchHQQuotesList(nil, req, QuoteClientOptions{}); err == nil {
			t.Fatalf("expected validation error for %+v", req)
		}
	}
}
