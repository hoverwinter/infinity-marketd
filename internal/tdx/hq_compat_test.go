package tdx

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestBuildHQCompactBatchQuotePacket(t *testing.T) {
	pkt, err := BuildHQCompactBatchQuotePacket([]QuoteRequest{{Market: "sh", Symbol: "600519"}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if pkt[0] != 0x0c {
		t.Fatalf("magic = %#x", pkt[0])
	}
	if cmd := binary.LittleEndian.Uint16(pkt[10:12]); cmd != cmdCompactBatchQuote {
		t.Fatalf("cmd = %#x", cmd)
	}
	if count := binary.LittleEndian.Uint16(pkt[20:22]); count != 1 {
		t.Fatalf("count = %d", count)
	}
}

func TestDecodeHQCompactBatchQuoteResponse(t *testing.T) {
	var body []byte
	body = append(body, 0x00, 0x00)
	body = binary.LittleEndian.AppendUint16(body, 1)
	body = append(body, 1)
	body = append(body, []byte("600519")...)
	body = binary.LittleEndian.AppendUint16(body, 7)
	for _, v := range []int{127286, -286, 714, 1014, -512, 145222494, 0, 31303, 560} {
		body = append(body, encTDXVarInt(v)...)
	}
	body = binary.LittleEndian.AppendUint32(body, math.Float32bits(3.98e9))
	for _, v := range []int{100, 200, 0, 0, -186, 0, 9, 7} {
		body = append(body, encTDXVarInt(v)...)
	}
	tail := make([]byte, 56)
	binary.LittleEndian.PutUint16(tail[2:4], uint16(int16(123)))
	binary.LittleEndian.PutUint16(tail[4:6], uint16(int16(45)))
	binary.LittleEndian.PutUint32(tail[6:10], math.Float32bits(1234))
	binary.LittleEndian.PutUint32(tail[22:26], math.Float32bits(1.5))
	binary.LittleEndian.PutUint32(tail[26:30], math.Float32bits(2.5))
	body = append(body, tail...)

	items, err := DecodeHQCompactBatchQuoteResponse(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len = %d", len(items))
	}
	it := items[0]
	if it.Market != "sh" || it.Symbol != "600519" || it.Price != 1272.86 || it.PreClose != 1270 {
		t.Fatalf("identity/prices wrong: %+v", it)
	}
	if it.BidPrice != 1271 || it.AskPrice != 1272.86 || it.BidVolume != 9 || it.AskVolume != 7 {
		t.Fatalf("bid/ask wrong: %+v", it)
	}
	if it.InsideVolume != 100 || it.OutsideVolume != 200 || it.RiseSpeed != 1.23 || it.ShortTurnover != 0.45 {
		t.Fatalf("extended fields wrong: %+v", it)
	}
	if it.VolRatio != 1.5 || it.Depth != 2.5 || it.Min2Amount != 1234 {
		t.Fatalf("tail fields wrong: %+v", it)
	}
}

func TestBuildHQCompactBatchQuoteValidation(t *testing.T) {
	if _, err := BuildHQCompactBatchQuotePacket(nil); err == nil {
		t.Fatal("expected empty request error")
	}
	if _, err := BuildHQCompactBatchQuotePacket([]QuoteRequest{{Market: "xx", Symbol: "600519"}}); err == nil {
		t.Fatal("expected invalid market error")
	}
}

func TestBuildHQTickChartPacket(t *testing.T) {
	req, err := ParseHQTickChartRequest("sh", "600519", 0, 2)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	pkt, err := BuildHQTickChartPacket(req)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if cmd := binary.LittleEndian.Uint16(pkt[10:12]); cmd != cmdTickChart {
		t.Fatalf("cmd = %#x", cmd)
	}
	if market := binary.LittleEndian.Uint16(pkt[12:14]); market != 1 {
		t.Fatalf("market = %d", market)
	}
	if count := binary.LittleEndian.Uint16(pkt[22:24]); count != 2 {
		t.Fatalf("count = %d", count)
	}
}

func TestDecodeHQTickChartResponse(t *testing.T) {
	body := binary.LittleEndian.AppendUint16(nil, 2)
	body = binary.LittleEndian.AppendUint16(body, 0)
	for _, v := range []int{1000, 100000, 10, 5, 50, 20} {
		body = append(body, encTDXVarInt(v)...)
	}
	req := HQTickChartRequest{Market: "sh", Symbol: "600519", Start: 0, Count: 2}
	points, err := DecodeHQTickChartResponse(req, body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("len = %d", len(points))
	}
	if points[0].Time != "09:30" || points[0].Price != 10 || points[0].Avg != 10 || points[0].Volume != 10 {
		t.Fatalf("point 0 wrong: %+v", points[0])
	}
	if points[1].Time != "09:31" || points[1].Price != 10.05 || points[1].Avg != 10.005 || points[1].Volume != 20 {
		t.Fatalf("point 1 wrong: %+v", points[1])
	}
}

func TestParseHQTickChartRequestValidation(t *testing.T) {
	for _, tc := range []HQTickChartRequest{
		{Market: "sh", Symbol: "600519", Start: -1, Count: 1},
		{Market: "sh", Symbol: "600519", Start: 0, Count: 0},
		{Market: "sh", Symbol: "600519", Start: 0, Count: MaxHQTickChartCount + 1},
		{Market: "sh", Symbol: "600519", Start: 1, Count: MaxHQTickChartCount},
	} {
		if _, err := ParseHQTickChartRequest(tc.Market, tc.Symbol, tc.Start, tc.Count); err == nil {
			t.Fatalf("expected validation error for %+v", tc)
		}
	}
}
