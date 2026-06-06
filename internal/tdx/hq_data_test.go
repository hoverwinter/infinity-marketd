package tdx

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestParseHQBarsRequestValidation(t *testing.T) {
	req, err := ParseHQBarsRequest(HQKLineDayAlt, "sh", "600519", 0, MaxHQKLineCount)
	if err != nil {
		t.Fatal(err)
	}
	if req.Market != "sh" || req.Symbol != "600519" || req.Category != HQKLineDayAlt {
		t.Fatalf("req = %#v", req)
	}
	if _, err := ParseHQBarsRequest(HQKLineDayAlt, "sh", "600519", 0, MaxHQKLineCount+1); err == nil {
		t.Fatal("expected max count error")
	}
	if _, err := ParseHQBarsRequest(99, "sh", "600519", 0, 1); err == nil {
		t.Fatal("expected category error")
	}
}

func TestBuildHQBarsPacket(t *testing.T) {
	req := HQBarsRequest{Category: HQKLineDayAlt, Market: "sh", Symbol: "600519", Start: 800, Count: 100}
	packet := BuildHQSecurityBarsPacket(req)
	if len(packet) != 38 {
		t.Fatalf("len = %d", len(packet))
	}
	if got := binary.LittleEndian.Uint16(packet[0:2]); got != 0x010c {
		t.Fatalf("command = %#x", got)
	}
	if got := binary.LittleEndian.Uint32(packet[2:6]); got != 0x01016408 {
		t.Fatalf("magic = %#x", got)
	}
	if got := binary.LittleEndian.Uint16(packet[12:14]); got != uint16(tdxMarketSH) {
		t.Fatalf("market = %d", got)
	}
	if string(packet[14:20]) != "600519" {
		t.Fatalf("symbol = %q", packet[14:20])
	}
	if got := binary.LittleEndian.Uint16(packet[20:22]); got != HQKLineDayAlt {
		t.Fatalf("category = %d", got)
	}
	if got := binary.LittleEndian.Uint16(packet[24:26]); got != 800 {
		t.Fatalf("start = %d", got)
	}
	if got := binary.LittleEndian.Uint16(packet[26:28]); got != 100 {
		t.Fatalf("count = %d", got)
	}
}

func TestDecodeHQBarsResponse(t *testing.T) {
	req := HQBarsRequest{Category: HQKLineDayAlt, Market: "sh", Symbol: "600519"}
	body := []byte{1, 0}
	body = binary.LittleEndian.AppendUint32(body, 20260605)
	body = append(body, encodeTDXVarInt(1272860)...)
	body = append(body, encodeTDXVarInt(10)...)
	body = append(body, encodeTDXVarInt(20)...)
	body = append(body, encodeTDXVarInt(-30)...)
	body = append(body, 0, 0, 0, 0)
	body = append(body, 0, 0, 0, 0)

	bars, err := DecodeHQBarsResponse(req, false, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 1 {
		t.Fatalf("bars = %#v", bars)
	}
	bar := bars[0]
	if bar.DateTime != "2026-06-05 15:00" || bar.Open != 1272.86 || bar.Close != 1272.87 || bar.High != 1272.88 || bar.Low != 1272.83 {
		t.Fatalf("bar = %#v", bar)
	}
}

func TestDecodeHQMinuteTimeResponses(t *testing.T) {
	req := HQMinuteRequest{Market: "sh", Symbol: "600519"}
	body := []byte{2, 0, 0, 0}
	body = append(body, encodeTDXVarInt(10000)...)
	body = append(body, encodeTDXVarInt(0)...)
	body = append(body, encodeTDXVarInt(5)...)
	body = append(body, encodeTDXVarInt(10)...)
	body = append(body, encodeTDXVarInt(0)...)
	body = append(body, encodeTDXVarInt(6)...)
	points, err := DecodeHQMinuteTimeResponse(req, 0, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 2 || points[0].Time != "09:30" || points[0].Price != 100 || points[1].Price != 100.1 || points[1].Volume != 6 {
		t.Fatalf("points = %#v", points)
	}

	history, err := DecodeHQHistoryMinuteTimeResponse(req, 20260605, []byte{})
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("history = %#v", history)
	}
}

func TestDecodeHQTransactionResponses(t *testing.T) {
	req := HQMinuteRequest{Market: "sz", Symbol: "000001"}
	body := []byte{1, 0}
	body = binary.LittleEndian.AppendUint16(body, 9*60+31)
	body = append(body, encodeTDXVarInt(1234)...)
	body = append(body, encodeTDXVarInt(20)...)
	body = append(body, encodeTDXVarInt(2)...)
	body = append(body, encodeTDXVarInt(1)...)
	body = append(body, encodeTDXVarInt(0)...)
	ticks, err := DecodeHQTransactionResponse(req, 0, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(ticks) != 1 || ticks[0].Time != "09:31" || ticks[0].Price != 12.34 || ticks[0].Num != 2 || ticks[0].BuyOrSell != 1 {
		t.Fatalf("ticks = %#v", ticks)
	}
}

func TestDecodeHQCompanyInfo(t *testing.T) {
	req := HQMinuteRequest{Market: "sh", Symbol: "600519"}
	body := []byte{1, 0}
	record := make([]byte, 152)
	copy(record[0:64], []byte("notice"))
	copy(record[64:144], []byte("600519.txt"))
	binary.LittleEndian.PutUint32(record[144:148], 123)
	binary.LittleEndian.PutUint32(record[148:152], 456)
	body = append(body, record...)
	categories, err := DecodeHQCompanyInfoCategoryResponse(req, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) != 1 || categories[0].Name != "notice" || categories[0].Filename != "600519.txt" || categories[0].Start != 123 {
		t.Fatalf("categories = %#v", categories)
	}

	content := []byte("hello")
	contentBody := make([]byte, 12)
	binary.LittleEndian.PutUint16(contentBody[10:12], uint16(len(content)))
	contentBody = append(contentBody, content...)
	got, err := DecodeHQCompanyInfoContentResponse(contentBody)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatalf("content = %q", got)
	}
}

func TestDecodeHQXDXRAndFinance(t *testing.T) {
	req := HQMinuteRequest{Market: "sh", Symbol: "600519"}
	body := make([]byte, 11)
	binary.LittleEndian.PutUint16(body[9:11], 1)
	body = append(body, byte(tdxMarketSH))
	body = appendFixedASCII(body, "600519", 6)
	body = append(body, 0)
	body = binary.LittleEndian.AppendUint32(body, 20260605)
	body = append(body, 1)
	body = appendFloat32(body, 1.2)
	body = appendFloat32(body, 2.3)
	body = appendFloat32(body, 3.4)
	body = appendFloat32(body, 4.5)
	rows, err := DecodeHQXDXRResponse(req, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Name != "除权除息" || rows[0].FenHong == nil || math.Abs(*rows[0].FenHong-1.2) > 0.0001 {
		t.Fatalf("rows = %#v", rows)
	}

	finance := make([]byte, 2)
	finance = append(finance, byte(tdxMarketSH))
	finance = appendFixedASCII(finance, "600519", 6)
	fields := make([]byte, 140)
	writeF32(fields[0:], 10)
	binary.LittleEndian.PutUint16(fields[4:6], 18)
	binary.LittleEndian.PutUint16(fields[6:8], 1)
	binary.LittleEndian.PutUint32(fields[8:12], 20260605)
	binary.LittleEndian.PutUint32(fields[12:16], 20010827)
	writeF32(fields[16:], 20)
	finance = append(finance, fields...)
	info, err := DecodeHQFinanceInfoResponse(req, finance)
	if err != nil {
		t.Fatal(err)
	}
	if info.Market != "sh" || info.Symbol != "600519" || info.LiuTongGuBen != 100000 || info.ZongGuBen != 200000 {
		t.Fatalf("finance = %#v", info)
	}
}

func TestDecodeHQBlockMembers(t *testing.T) {
	data := make([]byte, 384)
	data = binary.LittleEndian.AppendUint16(data, 1)
	data = appendFixedASCII(data, "测试", 9)
	data = binary.LittleEndian.AppendUint16(data, 2)
	data = binary.LittleEndian.AppendUint16(data, 7)
	data = appendFixedASCII(data, "1600519", 7)
	data = appendFixedASCII(data, "0000001", 7)
	data = append(data, make([]byte, 2800-14)...)
	members, err := DecodeHQBlockMembers("block.dat", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 || members[0].Market != "sh" || members[0].Symbol != "600519" || members[1].Market != "sz" {
		t.Fatalf("members = %#v", members)
	}
	if strings.TrimSpace(members[0].BlockName) == "" {
		t.Fatalf("block name empty: %#v", members)
	}
}

func appendFloat32(out []byte, value float32) []byte {
	raw := make([]byte, 4)
	writeF32(raw, value)
	return append(out, raw...)
}

func writeF32(out []byte, value float32) {
	binary.LittleEndian.PutUint32(out[:4], math.Float32bits(value))
}
