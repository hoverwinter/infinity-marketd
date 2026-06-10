package tdx

import (
	"encoding/binary"
	"testing"
)

func TestBuildMACSymbolQuotesPacket(t *testing.T) {
	pkt := BuildMACSymbolQuotesPacket([]MACSymbolQuoteRequest{{Market: "bj", Symbol: "920001"}})
	if pkt[0] != 0x01 || pkt[5] != 0x01 {
		t.Fatalf("header = %x", pkt[:10])
	}
	if cmd := binary.LittleEndian.Uint16(pkt[10:12]); cmd != cmdMACSymbolQuotes {
		t.Fatalf("cmd = %#x", cmd)
	}
	if got := binary.LittleEndian.Uint16(pkt[32:34]); got != 1 {
		t.Fatalf("request count = %d", got)
	}
	if market := binary.LittleEndian.Uint16(pkt[34:36]); market != tdxMarketBJ {
		t.Fatalf("market = %d", market)
	}
	if string(pkt[36:42]) != "920001" {
		t.Fatalf("symbol = %q", string(pkt[36:42]))
	}
}

func TestDecodeMACSymbolQuotesResponse(t *testing.T) {
	var bitmap [20]byte
	bitmap[0] = 0x03
	body := append([]byte{}, bitmap[:]...)
	body = binary.LittleEndian.AppendUint32(body, 1)
	body = binary.LittleEndian.AppendUint16(body, 1)

	row := make([]byte, 68+8)
	binary.LittleEndian.PutUint16(row[0:2], tdxMarketBJ)
	copy(row[2:24], "920001")
	copy(row[24:68], "BJTEST")
	body = append(body, row...)

	items, err := DecodeMACSymbolQuotesResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %#v", items)
	}
	if items[0].Market != "bj" || items[0].Symbol != "920001" || items[0].Name != "BJTEST" {
		t.Fatalf("item = %+v", items[0])
	}
}
