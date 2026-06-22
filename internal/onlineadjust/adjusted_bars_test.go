package onlineadjust

import (
	"context"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

func TestFetchHQAdjustedBarsOnlineNoneDoesNotFetchXDXR(t *testing.T) {
	oldBars := fetchHQSecurityBars
	oldXDXR := fetchHQXDXRInfo
	defer func() { fetchHQSecurityBars = oldBars; fetchHQXDXRInfo = oldXDXR }()

	fetchHQSecurityBars = func(_ context.Context, req tdx.HQBarsRequest, _ tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
		return []tdx.HQBar{{Market: req.Market, Symbol: req.Symbol, Category: req.Category, DateTime: "2026-06-05", Year: 2026, Month: 6, Day: 5, Open: 10, High: 11, Low: 9, Close: 10.5, Volume: 100, Amount: 1000}}, nil
	}
	fetchHQXDXRInfo = func(context.Context, tdx.HQMinuteRequest, tdx.QuoteClientOptions) ([]tdx.HQXDXRInfo, error) {
		t.Fatal("xdxr should not be fetched for adjust=none")
		return nil, nil
	}
	result, err := FetchHQAdjustedBarsOnline(context.Background(), HQAdjustedBarsOnlineRequest{Market: "sh", Symbol: "600519", Count: 1, Adjust: "none"}, tdx.QuoteClientOptions{})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(result.Bars) != 1 || result.Bars[0].Open != 10 || result.Bars[0].Factor != 1 {
		t.Fatalf("bars = %+v", result.Bars)
	}
}

func TestFetchHQAdjustedBarsOnlineQFQ(t *testing.T) {
	oldBars := fetchHQSecurityBars
	oldXDXR := fetchHQXDXRInfo
	defer func() { fetchHQSecurityBars = oldBars; fetchHQXDXRInfo = oldXDXR }()

	fetchHQSecurityBars = func(_ context.Context, req tdx.HQBarsRequest, _ tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
		return []tdx.HQBar{
			{Market: req.Market, Symbol: req.Symbol, Category: req.Category, DateTime: "2026-06-04", Year: 2026, Month: 6, Day: 4, Open: 10, High: 10, Low: 10, Close: 10, Volume: 100, Amount: 1000},
			{Market: req.Market, Symbol: req.Symbol, Category: req.Category, DateTime: "2026-06-05", Year: 2026, Month: 6, Day: 5, Open: 9, High: 9, Low: 9, Close: 9, Volume: 100, Amount: 1000},
		}, nil
	}
	zero := 0.0
	fenhong := 10.0
	fetchHQXDXRInfo = func(context.Context, tdx.HQMinuteRequest, tdx.QuoteClientOptions) ([]tdx.HQXDXRInfo, error) {
		return []tdx.HQXDXRInfo{{Market: "sh", Symbol: "600519", Date: "2026-06-05", Category: 1, Name: "dividend", FenHong: &fenhong, PeiGu: &zero, PeiGuJia: &zero, SongZhuanGu: &zero}}, nil
	}
	result, err := FetchHQAdjustedBarsOnline(context.Background(), HQAdjustedBarsOnlineRequest{Market: "sh", Symbol: "600519", Count: 2, Adjust: "qfq"}, tdx.QuoteClientOptions{})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(result.Bars) != 2 {
		t.Fatalf("bars len = %d", len(result.Bars))
	}
	if result.Bars[0].Factor != 0.9 || result.Bars[0].Close != 9 || result.Bars[1].Factor != 1 {
		t.Fatalf("adjusted bars = %+v", result.Bars)
	}
}

func TestNormalizeRequestRejectsBadAdjust(t *testing.T) {
	if _, err := NormalizeRequest(HQAdjustedBarsOnlineRequest{Market: "sh", Symbol: "600519", Count: 1, Adjust: "bad"}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestNormalizeRequestPreservesCategoryZero(t *testing.T) {
	req, err := NormalizeRequest(HQAdjustedBarsOnlineRequest{Market: "sh", Symbol: "600519", Category: tdx.HQKLine5Min, Count: 1, Adjust: "none"})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if req.Category != tdx.HQKLine5Min {
		t.Fatalf("category = %d", req.Category)
	}
}

func TestFetchHQAdjustedBarsOnlinePagesDailyFactorsToRequestedDate(t *testing.T) {
	oldBars := fetchHQSecurityBars
	oldXDXR := fetchHQXDXRInfo
	defer func() { fetchHQSecurityBars = oldBars; fetchHQXDXRInfo = oldXDXR }()

	calls := 0
	fetchHQSecurityBars = func(_ context.Context, req tdx.HQBarsRequest, _ tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
		calls++
		switch calls {
		case 1:
			if req.Start != 800 || req.Count != 1 {
				t.Fatalf("raw request = %+v", req)
			}
			return []tdx.HQBar{testHQBar("sh", "600519", req.Category, 2020, 1, 1, 10)}, nil
		case 2:
			if req.Category != tdx.HQKLineDayAlt || req.Start != 0 {
				t.Fatalf("first daily request = %+v", req)
			}
			return testHQDailyBars(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), tdx.MaxHQKLineCount), nil
		case 3:
			if req.Category != tdx.HQKLineDayAlt || req.Start != tdx.MaxHQKLineCount {
				t.Fatalf("second daily request = %+v", req)
			}
			return []tdx.HQBar{
				testHQBar("sh", "600519", req.Category, 2020, 1, 1, 10),
				testHQBar("sh", "600519", req.Category, 2020, 1, 2, 9),
			}, nil
		default:
			t.Fatalf("unexpected daily request %d: %+v", calls, req)
			return nil, nil
		}
	}
	zero := 0.0
	fenhong := 10.0
	fetchHQXDXRInfo = func(context.Context, tdx.HQMinuteRequest, tdx.QuoteClientOptions) ([]tdx.HQXDXRInfo, error) {
		return []tdx.HQXDXRInfo{{Market: "sh", Symbol: "600519", Date: "2020-01-02", Category: 1, Name: "dividend", FenHong: &fenhong, PeiGu: &zero, PeiGuJia: &zero, SongZhuanGu: &zero}}, nil
	}

	result, err := FetchHQAdjustedBarsOnline(context.Background(), HQAdjustedBarsOnlineRequest{
		Market: "sh", Symbol: "600519", Category: tdx.HQKLineDayAlt, Start: 800, Count: 1, Adjust: "qfq",
	}, tdx.QuoteClientOptions{})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(result.Bars) != 1 || result.Bars[0].Factor < 0.8999 || result.Bars[0].Factor > 0.9001 {
		t.Fatalf("adjusted bars = %+v", result.Bars)
	}
	if calls != 3 {
		t.Fatalf("fetch calls = %d", calls)
	}
}

func testHQDailyBars(start time.Time, count int) []tdx.HQBar {
	rows := make([]tdx.HQBar, 0, count)
	for i := 0; i < count; i++ {
		day := start.AddDate(0, 0, i)
		rows = append(rows, testHQBar("sh", "600519", tdx.HQKLineDayAlt, day.Year(), int(day.Month()), day.Day(), 20))
	}
	return rows
}

func testHQBar(market, symbol string, category, year, month, day int, close float64) tdx.HQBar {
	return tdx.HQBar{
		Market: market, Symbol: symbol, Category: category,
		DateTime: time.Date(year, time.Month(month), day, 15, 0, 0, 0, time.UTC).Format("2006-01-02 15:04"),
		Year:     year, Month: month, Day: day,
		Open: close, High: close, Low: close, Close: close, Volume: 100, Amount: 1000,
	}
}
