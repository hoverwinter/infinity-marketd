package tdx

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseG4DayPackageNormalizesTradedAShares(t *testing.T) {
	loc := mustShanghai(t)
	raw := testG4DayZIP(t, testG4DayEntries("260904"))
	expected := time.Date(2026, 9, 4, 0, 0, 0, 0, loc)

	result, err := ParseG4DayPackage(raw, "test.zip", &expected, loc)
	if err != nil {
		t.Fatalf("ParseG4DayPackage() error = %v", err)
	}
	if result.TradeDate.Format("2006-01-02") != "2026-09-04" {
		t.Fatalf("trade date = %s", result.TradeDate)
	}
	if result.Records != 6 || result.SHRecords != 2 || result.SZRecords != 2 || result.BJRecords != 2 {
		t.Fatalf("record counts = %#v", result)
	}
	if result.EquityRecords != 4 || result.NonEquityRecords != 2 || result.NoTradeRecords != 1 {
		t.Fatalf("classification counts = %#v", result)
	}
	if len(result.Bars) != 3 {
		t.Fatalf("bars = %d, want 3", len(result.Bars))
	}
	if got := result.Bars[0]; got.Market != "sh" || got.Symbol != "600001" || got.Open != 10 || got.High != 12 || got.Low != 9 || got.Close != 11 || got.Volume != 100 || got.Amount != 1100 {
		t.Fatalf("first bar = %#v", got)
	}
	if len(result.SHA256) != 64 || result.PackageBytes != uint64(len(raw)) {
		t.Fatalf("package identity = %#v", result)
	}
}

func TestParseG4DayPackageRejectsMissingMarketPair(t *testing.T) {
	entries := testG4DayEntries("260904")
	delete(entries, "bj260904.md1")
	_, err := ParseG4DayPackage(testG4DayZIP(t, entries), "missing.zip", nil, mustShanghai(t))
	if err == nil || !strings.Contains(err.Error(), "missing bj .cod/.md1 pair") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseG4DayPackageRejectsEntryDateMismatch(t *testing.T) {
	entries := testG4DayEntries("260904")
	entries["bj260905.md1"] = entries["bj260904.md1"]
	delete(entries, "bj260904.md1")
	_, err := ParseG4DayPackage(testG4DayZIP(t, entries), "date.zip", nil, mustShanghai(t))
	if err == nil || !strings.Contains(err.Error(), "does not match package date") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseG4DayPackageRejectsRecordCountMismatch(t *testing.T) {
	entries := testG4DayEntries("260904")
	entries["sh260904.md1"] = append(entries["sh260904.md1"], testG4DayQuote(10, 12, 9, 11, 100, 1100)...)
	_, err := ParseG4DayPackage(testG4DayZIP(t, entries), "count.zip", nil, mustShanghai(t))
	if err == nil || !strings.Contains(err.Error(), "record count mismatch") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseG4DayPackageRejectsDuplicateCode(t *testing.T) {
	entries := testG4DayEntries("260904")
	entries["sh260904.cod"] = testG4DayCodes("600001", "600001")
	_, err := ParseG4DayPackage(testG4DayZIP(t, entries), "duplicate.zip", nil, mustShanghai(t))
	if err == nil || !strings.Contains(err.Error(), "duplicates symbol 600001") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseG4DayPackageRejectsCorruptEligibleRow(t *testing.T) {
	entries := testG4DayEntries("260904")
	entries["sz260904.md1"] = append(testG4DayQuote(10, 8, 9, 11, 100, 1100), testG4DayQuote(10, 12, 9, 11, 100, 1100)...)
	_, err := ParseG4DayPackage(testG4DayZIP(t, entries), "corrupt.zip", nil, mustShanghai(t))
	if err == nil || !strings.Contains(err.Error(), "inconsistent OHLC values") {
		t.Fatalf("error = %v", err)
	}
}

func TestFetchG4DayPackageUsesDatePathAndBoundsResponse(t *testing.T) {
	payload := testG4DayZIP(t, testG4DayEntries("260904"))
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	date := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	raw, source, err := FetchG4DayPackage(context.Background(), date, G4DayFetchOptions{BaseURL: server.URL + "/g4day"})
	if err != nil {
		t.Fatal(err)
	}
	if requestPath != "/g4day/20260904.zip" || source != server.URL+"/g4day/20260904.zip" || !bytes.Equal(raw, payload) {
		t.Fatalf("path=%q source=%q bytes=%d", requestPath, source, len(raw))
	}

	oversized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "16777217")
		w.WriteHeader(http.StatusOK)
	}))
	defer oversized.Close()
	_, _, err = FetchG4DayPackage(context.Background(), date, G4DayFetchOptions{BaseURL: oversized.URL})
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("oversized error = %v", err)
	}
}

func TestReadG4DayPackageFileRejectsOversizedFile(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "large.zip")
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxG4DayPackageBytes + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = ReadG4DayPackageFile(filePath)
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("error = %v", err)
	}
}

func TestG4DayAShareClassificationExcludesBeijingIndices(t *testing.T) {
	tests := []struct {
		market string
		symbol string
		want   bool
	}{
		{market: "sh", symbol: "600519", want: true},
		{market: "sh", symbol: "000001", want: false},
		{market: "sz", symbol: "000001", want: true},
		{market: "sz", symbol: "399001", want: false},
		{market: "bj", symbol: "920982", want: true},
		{market: "bj", symbol: "899050", want: false},
		{market: "bj", symbol: "899601", want: false},
	}
	for _, tt := range tests {
		if got := isG4DayAShare(tt.market, tt.symbol); got != tt.want {
			t.Fatalf("isG4DayAShare(%q, %q) = %v, want %v", tt.market, tt.symbol, got, tt.want)
		}
	}
}

func testG4DayEntries(date string) map[string][]byte {
	return map[string][]byte{
		"sh" + date + ".cod": testG4DayCodes("600001", "110001"),
		"sh" + date + ".md1": append(testG4DayQuote(10, 12, 9, 11, 100, 1100), testG4DayQuote(100, 101, 99, 100, 200, 20000)...),
		"sz" + date + ".cod": testG4DayCodes("000001", "399001"),
		"sz" + date + ".md1": append(testG4DayQuote(20, 22, 19, 21, 300, 6300), testG4DayQuote(200, 202, 198, 201, 400, 80400)...),
		"bj" + date + ".cod": testG4DayCodes("920001", "920002"),
		"bj" + date + ".md1": append(testG4DayQuote(30, 33, 29, 32, 500, 16000), testG4DayQuote(0, 0, 0, 31, 0, 0)...),
	}
}

func testG4DayZIP(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, raw := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testG4DayCodes(symbols ...string) []byte {
	raw := make([]byte, len(symbols)*g4DayCodeRecordSize)
	for i, symbol := range symbols {
		copy(raw[i*g4DayCodeRecordSize:], symbol)
	}
	return raw
}

func testG4DayQuote(open float64, high float64, low float64, closeValue float64, volume uint64, amount float64) []byte {
	raw := make([]byte, g4DayQuoteRecordSize)
	binary.LittleEndian.PutUint64(raw[12:20], math.Float64bits(open))
	binary.LittleEndian.PutUint64(raw[20:28], math.Float64bits(high))
	binary.LittleEndian.PutUint64(raw[28:36], math.Float64bits(low))
	binary.LittleEndian.PutUint64(raw[36:44], math.Float64bits(closeValue))
	binary.LittleEndian.PutUint64(raw[56:64], volume)
	binary.LittleEndian.PutUint64(raw[72:80], math.Float64bits(amount))
	return raw
}
