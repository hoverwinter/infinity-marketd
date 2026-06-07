package tdx

import "testing"

const lhbFixture = `【1.交易龙虎榜】
●交易日期:2026-06-05 信息类型:日涨幅偏离值达7%
涨跌幅(%):8.12 成交量(万股):1234.56 成交额(万元):7890.12
买入前五
│营业部名称│买入金额│卖出金额│
│某某证券A │ 1000.00│ 0.00│
│某某证券B │ 500.00│ 0.00│
卖出前五
│营业部名称│买入金额│卖出金额│
│某某证券C │ 0.00│ 800.00│
`

func TestParseLHBNormal(t *testing.T) {
	records := ParseLHB(extractLHBContent(lhbFixture))
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	r := records[0]
	if r.Date != "2026-06-05" {
		t.Fatalf("date = %q", r.Date)
	}
	if r.ChangePct != 8.12 || r.Volume != 1234.56 || r.Amount != 7890.12 {
		t.Fatalf("summary wrong: %+v", r)
	}
	if len(r.BuySeats) != 2 || r.BuySeats[0].Name != "某某证券A" || r.BuySeats[0].BuyAmt != 1000 {
		t.Fatalf("buy seats wrong: %+v", r.BuySeats)
	}
	if len(r.SellSeats) != 1 || r.SellSeats[0].Name != "某某证券C" || r.SellSeats[0].SellAmt != 800 {
		t.Fatalf("sell seats wrong: %+v", r.SellSeats)
	}
}

func TestParseLHBEmpty(t *testing.T) {
	if got := ParseLHB("no records here\n"); len(got) != 0 {
		t.Fatalf("expected no records, got %d", len(got))
	}
}

func TestFindLHBSection(t *testing.T) {
	cats := []HQCompanyInfoCategory{
		{Name: "最新提示", Filename: "a.txt"},
		{Name: "资金动向", Filename: "lhb.txt", Start: 0, Length: 100},
	}
	cat, ok := findLHBSection(cats, nil)
	if !ok || cat.Filename != "lhb.txt" {
		t.Fatalf("expected to find 资金动向, got %+v ok=%v", cat, ok)
	}
	if _, ok := findLHBSection(cats[:1], nil); ok {
		t.Fatalf("expected missing section")
	}
	// alias match
	if _, ok := findLHBSection([]HQCompanyInfoCategory{{Name: "龙虎榜"}}, []string{"龙虎榜"}); !ok {
		t.Fatalf("expected alias match")
	}
}
