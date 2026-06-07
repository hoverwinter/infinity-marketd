package finance

import (
	"os"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

func TestParseFinancialDAT(t *testing.T) {
	loc := mustShanghai(t)
	dict, err := LoadFinancialItemDictionary()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../testdata/finance/gpcw_one_stock.dat")
	if err != nil {
		t.Fatal(err)
	}
	result := ParseFinancialDAT(raw, "gpcw_one_stock.dat", loc, FinancialItemDictionaryMap(dict))
	if len(result.Issues) != 0 {
		t.Fatalf("issues = %+v", result.Issues)
	}
	if len(result.Rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(result.Rows))
	}
	first := result.Rows[0]
	if first.Market != "sz" || first.Symbol != "000001" || first.ItemID != 1 {
		t.Fatalf("unexpected first row: %+v", first)
	}
	if got := first.ReportDate.Format("2006-01-02"); got != "2025-12-31" {
		t.Fatalf("report date = %s", got)
	}
}

func TestParseGPMetricDAT(t *testing.T) {
	loc := mustShanghai(t)
	dict, err := LoadGPMetricDictionary()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../testdata/finance/gpbj430017_two_records.dat")
	if err != nil {
		t.Fatal(err)
	}
	result := ParseGPMetricDAT(raw, "gpbj430017.dat", "bj", "430017", loc, GPMetricDictionaryMap(dict))
	if len(result.Issues) != 0 {
		t.Fatalf("issues = %+v", result.Issues)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(result.Rows))
	}
	first := result.Rows[0]
	if first.Market != "bj" || first.Symbol != "430017" || first.MetricType != 1 {
		t.Fatalf("unexpected first row: %+v", first)
	}
	if got := first.EventDate.Format("2006-01-02"); got != "2012-12-31" {
		t.Fatalf("event date = %s", got)
	}
	if first.Value1 != 16 || first.Value2 != 0 {
		t.Fatalf("values = %f/%f", first.Value1, first.Value2)
	}
}

func TestParseGPMetricDATUnknownDictionaryID(t *testing.T) {
	loc := mustShanghai(t)
	raw := []byte{
		99,
		0x8f, 0x06, 0x33, 0x01,
		0x00, 0x00, 0x80, 0x41,
		0x00, 0x00, 0x00, 0x00,
	}
	result := ParseGPMetricDAT(raw, "gp.dat", "bj", "430017", loc, modelGPStub(1))
	if len(result.Rows) != 0 {
		t.Fatalf("rows = %d, want 0", len(result.Rows))
	}
	if len(result.Issues) < 2 || result.Issues[0].Type != "unknown_dictionary_id" || result.Issues[len(result.Issues)-1].Type != "zero_valid_rows" {
		t.Fatalf("issues = %+v", result.Issues)
	}
}

func TestParseFinancialDATZeroValidRows(t *testing.T) {
	loc := mustShanghai(t)
	raw := []byte{
		0x01, 0x00,
		0x5f, 0x02, 0x35, 0x01,
		0x00, 0x00,
		0x00, 0x00, 0x0b, 0x00,
		0x10, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	result := ParseFinancialDAT(raw, "gpcw_zero.dat", loc, nil)
	if len(result.Rows) != 0 {
		t.Fatalf("rows = %d, want 0", len(result.Rows))
	}
	if len(result.Issues) == 0 || result.Issues[0].Type != "zero_valid_rows" {
		t.Fatalf("issues = %+v", result.Issues)
	}
}

func mustShanghai(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func modelGPStub(metricType uint16) map[uint16]model.GPMetricDictionaryEntry {
	return map[uint16]model.GPMetricDictionaryEntry{
		metricType: {MetricType: metricType, Name: "gp01", Title: "stub", Value1Meaning: "v1", Status: "confirmed", SourceRef: "test"},
	}
}
