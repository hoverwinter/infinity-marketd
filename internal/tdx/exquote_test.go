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

func TestBuildExInstrumentCountPacket(t *testing.T) {
	packet := BuildExInstrumentCountPacket()
	want := []byte{0x01, 0x03, 0x48, 0x66, 0x00, 0x01, 0x02, 0x00, 0x02, 0x00, 0xf0, 0x23}
	if string(packet) != string(want) {
		t.Fatalf("packet = %v", packet)
	}
}

func TestDecodeExInstrumentCountResponse(t *testing.T) {
	body := make([]byte, 23)
	binary.LittleEndian.PutUint32(body[19:23], 12345)
	count, err := DecodeExInstrumentCountResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if count != 12345 {
		t.Fatalf("count = %d", count)
	}
}

func TestBuildAndDecodeExInstrumentInfo(t *testing.T) {
	packet, err := BuildExInstrumentInfoPacket(100, 2)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := []byte{0x01, 0x04, 0x48, 0x67, 0x00, 0x01, 0x08, 0x00, 0x08, 0x00, 0xf5, 0x23}
	if string(packet[:12]) != string(wantPrefix) {
		t.Fatalf("prefix = %v", packet[:12])
	}
	if got := binary.LittleEndian.Uint32(packet[12:16]); got != 100 {
		t.Fatalf("start = %d", got)
	}
	if got := binary.LittleEndian.Uint16(packet[16:18]); got != 2 {
		t.Fatalf("count = %d", got)
	}

	instruments, err := DecodeExInstrumentInfoResponse(exInstrumentInfoBody([]exInstrumentRecord{
		{Category: 3, Market: 47, Code: "IF1709", Name: "IF main", Desc: "index"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(instruments) != 1 {
		t.Fatalf("instruments = %#v", instruments)
	}
	if instruments[0] != (ExInstrument{Category: 3, Market: 47, Code: "IF1709", Name: "IF main", Desc: "index"}) {
		t.Fatalf("instrument = %#v", instruments[0])
	}
}

func TestBuildAndDecodeExBars(t *testing.T) {
	req, err := ParseExBarsRequest(ExKLineExHQ1Min, 47, "IF1709", 10, 2)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := BuildExBarsRequestPacket(req)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := []byte{0x01, 0x01, 0x08, 0x6a, 0x01, 0x01, 0x16, 0x00, 0x16, 0x00, 0xff, 0x23}
	if string(packet[:12]) != string(wantPrefix) {
		t.Fatalf("prefix = %v", packet[:12])
	}
	if packet[12] != 47 || string(packet[13:19]) != "IF1709" {
		t.Fatalf("identity = %v", packet[12:22])
	}
	if got := binary.LittleEndian.Uint16(packet[22:24]); got != ExKLineExHQ1Min {
		t.Fatalf("category = %d", got)
	}
	if got := binary.LittleEndian.Uint32(packet[26:30]); got != 10 {
		t.Fatalf("start = %d", got)
	}
	if got := binary.LittleEndian.Uint16(packet[30:32]); got != 2 {
		t.Fatalf("count = %d", got)
	}

	bars, err := DecodeExBarsResponse(req, exBarsBody(ExKLineExHQ1Min))
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 1 {
		t.Fatalf("bars = %#v", bars)
	}
	bar := bars[0]
	if bar.DateTime != "2026-06-05 09:30" || !floatNear(bar.Open, 3717.2) || !floatNear(bar.Close, 3703) || bar.Position != 2043 || bar.Trade != 1728 {
		t.Fatalf("bar = %#v", bar)
	}
}

func TestBuildAndDecodeExMinuteTime(t *testing.T) {
	req := ExQuoteRequest{Market: 47, Code: "IF1709"}
	packet := BuildExMinuteTimePacket(req)
	wantPrefix := []byte{0x01, 0x07, 0x08, 0x00, 0x01, 0x01, 0x0c, 0x00, 0x0c, 0x00, 0x0b, 0x24}
	if string(packet[:12]) != string(wantPrefix) {
		t.Fatalf("prefix = %v", packet[:12])
	}
	points, err := DecodeExMinuteTimeResponse(exMinuteTimeBody(false, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].Time != "09:30" || !floatNear(points[0].Price, 3706.2) || points[0].Volume != 27 || points[0].OpenInterest != 13336 {
		t.Fatalf("points = %#v", points)
	}

	historyPacket := BuildExHistoryMinuteTimePacket(req, 20260605)
	if got := binary.LittleEndian.Uint32(historyPacket[12:16]); got != 20260605 {
		t.Fatalf("history date = %d", got)
	}
	historyPoints, err := DecodeExHistoryMinuteTimeResponse(20260605, exMinuteTimeBody(true, 20260605))
	if err != nil {
		t.Fatal(err)
	}
	if len(historyPoints) != 1 || historyPoints[0].DateTime != "2026-06-05 09:30" {
		t.Fatalf("history points = %#v", historyPoints)
	}
}

func TestBuildAndDecodeExTransactions(t *testing.T) {
	req := ExQuoteRequest{Market: 31, Code: "00020"}
	packet, err := BuildExTransactionPacket(req, 1800, 100)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := []byte{0x01, 0x01, 0x08, 0x00, 0x03, 0x01, 0x12, 0x00, 0x12, 0x00, 0xfc, 0x23}
	if string(packet[:12]) != string(wantPrefix) {
		t.Fatalf("prefix = %v", packet[:12])
	}
	if got := binary.LittleEndian.Uint32(packet[22:26]); got != 1800 {
		t.Fatalf("start = %d", got)
	}
	if got := binary.LittleEndian.Uint16(packet[26:28]); got != 100 {
		t.Fatalf("count = %d", got)
	}

	transactions, err := DecodeExTransactionResponse(exTransactionBody(0))
	if err != nil {
		t.Fatal(err)
	}
	if len(transactions) != 1 {
		t.Fatalf("transactions = %#v", transactions)
	}
	tx := transactions[0]
	if tx.Time != "09:30:00" || tx.Price != 123456 || tx.Volume != 100 || tx.ZengCang != 10 || tx.NatureName != "B" || tx.Direction != 1 {
		t.Fatalf("transaction = %#v", tx)
	}

	historyPacket, err := BuildExHistoryTransactionPacket(req, 20260605, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint32(historyPacket[12:16]); got != 20260605 {
		t.Fatalf("history date = %d", got)
	}
	historyTransactions, err := DecodeExHistoryTransactionResponse(20260605, exTransactionBody(20260605))
	if err != nil {
		t.Fatal(err)
	}
	if len(historyTransactions) != 1 || historyTransactions[0].DateTime != "2026-06-05 09:30:00" {
		t.Fatalf("history transactions = %#v", historyTransactions)
	}
}

func TestBuildAndDecodeExHistoryBarsRange(t *testing.T) {
	req := ExQuoteRequest{Market: 74, Code: "BABA"}
	packet, err := BuildExHistoryBarsRangePacket(req, 20260601, 20260605)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := []byte{0x01, 0x01, 0x38, 0x92, 0x00, 0x01, 0x16, 0x00, 0x16, 0x00, 0x0d, 0x24}
	if string(packet[:12]) != string(wantPrefix) {
		t.Fatalf("prefix = %v", packet[:12])
	}
	if got := binary.LittleEndian.Uint16(packet[22:24]); got != ExKLineExHQ1Min {
		t.Fatalf("category = %d", got)
	}
	bars, err := DecodeExHistoryBarsRangeResponse(req, exHistoryBarsRangeBody())
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 1 {
		t.Fatalf("bars = %#v", bars)
	}
	bar := bars[0]
	if bar.DateTime != "2026-06-05 09:30" || !floatNear(bar.SettlementPrice, 3704.4) {
		t.Fatalf("bar = %#v", bar)
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

func TestFetchExQuoteUsesExHQRequest(t *testing.T) {
	req := ExQuoteRequest{Market: 47, Code: "IF1709"}
	server := startScriptedHQServer(t, []scriptStep{
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

func TestFetchExMarketsUsesExHQRequest(t *testing.T) {
	server := startScriptedHQServer(t, []scriptStep{
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

func TestFetchExInstrumentCountUsesExHQRequest(t *testing.T) {
	body := make([]byte, 23)
	binary.LittleEndian.PutUint32(body[19:23], 12345)
	server := startScriptedHQServer(t, []scriptStep{
		{ReadLen: len(BuildExInstrumentCountPacket()), Body: body},
	})
	count, err := FetchExInstrumentCount(context.Background(), ExQuoteClientOptions{Server: server})
	if err != nil {
		t.Fatal(err)
	}
	if count != 12345 {
		t.Fatalf("count = %d", count)
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

type exInstrumentRecord struct {
	Category byte
	Market   byte
	Code     string
	Name     string
	Desc     string
}

func exInstrumentInfoBody(records []exInstrumentRecord) []byte {
	body := make([]byte, 6)
	binary.LittleEndian.PutUint16(body[4:6], uint16(len(records)))
	for _, item := range records {
		record := make([]byte, 64)
		record[0] = item.Category
		record[1] = item.Market
		copy(record[5:14], item.Code)
		copy(record[14:31], item.Name)
		copy(record[31:40], item.Desc)
		body = append(body, record...)
	}
	return body
}

func exPackedDay(year, month, day int) uint16 {
	return uint16((year-2004)*2048 + month*100 + day)
}

func exBarsBody(category int) []byte {
	body := make([]byte, 20)
	binary.LittleEndian.PutUint16(body[18:20], 1)
	if category < ExKLineDaily || category == ExKLineExHQ1Min || category == ExKLine1Min {
		body = binary.LittleEndian.AppendUint16(body, exPackedDay(2026, 6, 5))
		body = binary.LittleEndian.AppendUint16(body, uint16(9*60+30))
	} else {
		body = binary.LittleEndian.AppendUint32(body, 20260605)
	}
	for _, value := range []float32{3717.2, 3724, 3696.6, 3703} {
		body = appendExFloat32(body, value)
	}
	body = binary.LittleEndian.AppendUint32(body, 2043)
	body = binary.LittleEndian.AppendUint32(body, 1728)
	body = appendExFloat32(body, 3704.4)
	return body
}

func exMinuteTimeBody(history bool, date int) []byte {
	body := make([]byte, 0, 38)
	body = append(body, 47)
	code := make([]byte, 9)
	copy(code, "IF1709")
	body = append(body, code...)
	if history {
		body = append(body, make([]byte, 8)...)
	}
	body = binary.LittleEndian.AppendUint16(body, 1)
	body = binary.LittleEndian.AppendUint16(body, uint16(9*60+30))
	body = appendExFloat32(body, 3706.2)
	body = appendExFloat32(body, 3705.91)
	body = binary.LittleEndian.AppendUint32(body, 27)
	body = binary.LittleEndian.AppendUint32(body, 13336)
	_ = date
	return body
}

func exTransactionBody(date int) []byte {
	body := make([]byte, 0, 32)
	body = append(body, 31)
	code := make([]byte, 9)
	copy(code, "00020")
	body = append(body, code...)
	body = append(body, 0, 0, 0, 0)
	body = binary.LittleEndian.AppendUint16(body, 1)
	body = binary.LittleEndian.AppendUint16(body, uint16(9*60+30))
	body = binary.LittleEndian.AppendUint32(body, 123456)
	body = binary.LittleEndian.AppendUint32(body, 100)
	body = binary.LittleEndian.AppendUint32(body, uint32(int32(10)))
	body = binary.LittleEndian.AppendUint16(body, 0)
	_ = date
	return body
}

func exHistoryBarsRangeBody() []byte {
	body := make([]byte, 14)
	binary.LittleEndian.PutUint16(body[12:14], 1)
	body = binary.LittleEndian.AppendUint16(body, exPackedDay(2026, 6, 5))
	body = binary.LittleEndian.AppendUint16(body, uint16(9*60+30))
	for _, value := range []float32{3717.2, 3724, 3696.6, 3703} {
		body = appendExFloat32(body, value)
	}
	body = binary.LittleEndian.AppendUint32(body, 2043)
	body = binary.LittleEndian.AppendUint32(body, 1728)
	body = appendExFloat32(body, 3704.4)
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
