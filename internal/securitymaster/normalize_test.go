package securitymaster

import "testing"

func TestNormalizeSourceRowForBeijingSecurity(t *testing.T) {
	row, err := NormalizeSourceRow(SourceRow{
		Market:         " BJ ",
		Symbol:         "920001",
		Name:           " 北证测试 ",
		ListingDate:    "20260610",
		LotSize:        100,
		PricePrecision: 2,
		Aliases:        []Alias{{Alias: "BJTEST", AliasType: "english", Priority: 60}},
	}, SourceTDX)
	if err != nil {
		t.Fatal(err)
	}
	security := row.Security
	if security.Market != "bj" || security.Exchange != "BSE" || security.Board != "bse" {
		t.Fatalf("security = %+v", security)
	}
	if security.CurrentName != "北证测试" || security.CurrentNameNorm != "北证测试" || security.Status != StatusUnknown {
		t.Fatalf("security = %+v", security)
	}
	if len(row.Aliases) != 2 {
		t.Fatalf("aliases = %+v", row.Aliases)
	}
	if len(row.History) != 1 || row.History[0].ValidFrom != "2026-06-10" {
		t.Fatalf("history = %+v", row.History)
	}
}

func TestNormalizeSourceRowRejectsInvalidDate(t *testing.T) {
	_, err := NormalizeSourceRow(SourceRow{Market: "sh", Symbol: "600519", Name: "贵州茅台", ListingDate: "2026/06/10"}, SourceFile)
	if err == nil {
		t.Fatal("expected invalid date error")
	}
}

func TestNormalizeMarketsDefaultsToAShareMarkets(t *testing.T) {
	markets, err := NormalizeMarkets(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(markets) != 3 || markets[0] != "sh" || markets[1] != "sz" || markets[2] != "bj" {
		t.Fatalf("markets = %#v", markets)
	}
}
