package finance

import "testing"

func TestLoadFinancialItemDictionary(t *testing.T) {
	items, err := LoadFinancialItemDictionary()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 584 {
		t.Fatalf("financial items = %d, want at least 584", len(items))
	}
	if items[0].ItemID != 1 || items[0].Name != "fn1" || items[0].Title == "" {
		t.Fatalf("unexpected first item: %+v", items[0])
	}
}

func TestLoadGPMetricDictionary(t *testing.T) {
	items, err := LoadGPMetricDictionary()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 46 {
		t.Fatalf("gp metrics = %d, want 46", len(items))
	}
	if items[0].MetricType != 1 || items[0].Name != "gp01" || items[0].Title == "" {
		t.Fatalf("unexpected first metric: %+v", items[0])
	}
}
