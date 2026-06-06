package querier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

func TestTDXProviderRoutesDoNotAffectV1Bars(t *testing.T) {
	var tdxCalls int
	provider := DefaultTDXProvider()
	provider.FetchRealtimeQuotes = func(context.Context, []tdx.QuoteRequest, tdx.QuoteClientOptions) ([]tdx.Quote, error) {
		tdxCalls++
		return nil, nil
	}
	repo := &fakeRepo{}
	server := httptest.NewServer(NewServerWithTDXProvider(repo, provider).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/bars?market=sh&symbol=600519&period=1d")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if tdxCalls != 0 {
		t.Fatalf("tdx calls=%d", tdxCalls)
	}
}

func TestTDXHQQuotesRoute(t *testing.T) {
	provider := DefaultTDXProvider()
	var gotRequests []tdx.QuoteRequest
	var gotOpts tdx.QuoteClientOptions
	provider.FetchRealtimeQuotes = func(_ context.Context, requests []tdx.QuoteRequest, opts tdx.QuoteClientOptions) ([]tdx.Quote, error) {
		gotRequests = requests
		gotOpts = opts
		return []tdx.Quote{{Market: "sh", Symbol: "600519", Price: 12.34}}, nil
	}
	server := httptest.NewServer(NewServerWithTDXProvider(&fakeRepo{}, provider).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/tdx/hq/quotes?symbol=sh:600519&symbol=000001&server=a:7709,b:7709&batch-size=2&trade_date=2026-06-05")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var quotes []tdx.Quote
	if err := json.NewDecoder(resp.Body).Decode(&quotes); err != nil {
		t.Fatal(err)
	}
	if len(quotes) != 1 || quotes[0].Symbol != "600519" {
		t.Fatalf("quotes=%+v", quotes)
	}
	if len(gotRequests) != 2 || gotRequests[1].Market != "sz" || gotRequests[1].Symbol != "000001" {
		t.Fatalf("requests=%+v", gotRequests)
	}
	if gotOpts.BatchSize != 2 || len(gotOpts.Servers) != 2 || gotOpts.TradeDate.IsZero() {
		t.Fatalf("opts=%+v", gotOpts)
	}
}

func TestTDXHQQuotesRouteEnforcesRequestLimit(t *testing.T) {
	server := httptest.NewServer(NewServerWithTDXProvider(&fakeRepo{}, DefaultTDXProvider()).Handler())
	defer server.Close()

	var parts []string
	for i := 0; i < maxHTTPQuoteSymbols+1; i++ {
		parts = append(parts, "600519")
	}
	resp, err := http.Get(server.URL + "/api/tdx/hq/quotes?symbols=" + strings.Join(parts, ","))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestTDXHQProbeRoute(t *testing.T) {
	provider := DefaultTDXProvider()
	var gotServers []string
	provider.ProbeHQServers = func(_ context.Context, servers []string, _ tdx.QuoteClientOptions) []tdx.ServerProbeResult {
		gotServers = servers
		return []tdx.ServerProbeResult{
			{Server: "slow:7709", Success: true, LatencyMS: 10},
			{Server: "fast:7709", Success: true, LatencyMS: 1},
		}
	}
	server := httptest.NewServer(NewServerWithTDXProvider(&fakeRepo{}, provider).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/tdx/hq/probe?server=slow:7709,fast:7709")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var results []tdx.ServerProbeResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatal(err)
	}
	if len(gotServers) != 2 || results[0].Server != "fast:7709" || !results[0].Preferred {
		t.Fatalf("servers=%+v results=%+v", gotServers, results)
	}
}

func TestTDXHQSecuritiesRoute(t *testing.T) {
	provider := DefaultTDXProvider()
	provider.FetchSecurityList = func(_ context.Context, market string, _ tdx.QuoteClientOptions) ([]tdx.Security, error) {
		if market != "sh" {
			t.Fatalf("market=%s", market)
		}
		return []tdx.Security{{Market: "sh", Symbol: "600519", Name: "name"}}, nil
	}
	server := httptest.NewServer(NewServerWithTDXProvider(&fakeRepo{}, provider).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/tdx/hq/securities?market=sh")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestTDXHQAuxiliaryRoutes(t *testing.T) {
	provider := DefaultTDXProvider()
	provider.FetchHQSecurityBars = func(_ context.Context, req tdx.HQBarsRequest, _ tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
		return []tdx.HQBar{{Market: req.Market, Symbol: req.Symbol, Category: req.Category}}, nil
	}
	provider.FetchHQMinuteTime = func(_ context.Context, req tdx.HQMinuteRequest, _ tdx.QuoteClientOptions) ([]tdx.HQMinutePoint, error) {
		return []tdx.HQMinutePoint{{Market: req.Market, Symbol: req.Symbol}}, nil
	}
	provider.FetchHQHistoryTransactions = func(_ context.Context, req tdx.HQMinuteRequest, date, start, count int, _ tdx.QuoteClientOptions) ([]tdx.HQTransaction, error) {
		return []tdx.HQTransaction{{Market: req.Market, Symbol: req.Symbol, Date: fmt.Sprint(date)}}, nil
	}
	provider.FetchHQCompanyInfoCategories = func(_ context.Context, req tdx.HQMinuteRequest, _ tdx.QuoteClientOptions) ([]tdx.HQCompanyInfoCategory, error) {
		return []tdx.HQCompanyInfoCategory{{Market: req.Market, Symbol: req.Symbol, Name: "cat"}}, nil
	}
	provider.FetchHQXDXRInfo = func(_ context.Context, req tdx.HQMinuteRequest, _ tdx.QuoteClientOptions) ([]tdx.HQXDXRInfo, error) {
		return []tdx.HQXDXRInfo{{Market: req.Market, Symbol: req.Symbol}}, nil
	}
	provider.FetchHQFinanceInfo = func(_ context.Context, req tdx.HQMinuteRequest, _ tdx.QuoteClientOptions) (tdx.HQFinanceInfo, error) {
		return tdx.HQFinanceInfo{Market: req.Market, Symbol: req.Symbol}, nil
	}
	provider.FetchHQBlockMeta = func(context.Context, string, tdx.QuoteClientOptions) (tdx.HQBlockMeta, error) {
		return tdx.HQBlockMeta{File: "block.dat", Size: 1}, nil
	}
	provider.FetchHQBlockChunk = func(context.Context, string, uint32, uint32, tdx.QuoteClientOptions) (tdx.HQBlockChunk, error) {
		return tdx.HQBlockChunk{File: "block.dat", Size: 1}, nil
	}
	provider.FetchHQBlockMembers = func(context.Context, string, tdx.QuoteClientOptions) ([]tdx.HQBlockMember, error) {
		return []tdx.HQBlockMember{{BlockName: "block"}}, nil
	}
	server := httptest.NewServer(NewServerWithTDXProvider(&fakeRepo{}, provider).Handler())
	defer server.Close()

	paths := []string{
		"/api/tdx/hq/bars?market=sh&symbol=600519&category=9&count=1",
		"/api/tdx/hq/minute?market=sh&symbol=600519",
		"/api/tdx/hq/transactions?market=sh&symbol=600519&date=20260605&count=1",
		"/api/tdx/hq/company-categories?market=sh&symbol=600519",
		"/api/tdx/hq/xdxr?market=sh&symbol=600519",
		"/api/tdx/hq/finance?market=sh&symbol=600519",
		"/api/tdx/hq/block-meta?file=block.dat",
		"/api/tdx/hq/block-chunk?file=block.dat&size=1",
		"/api/tdx/hq/block?file=block.dat",
	}
	for _, path := range paths {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status=%d", path, resp.StatusCode)
		}
	}
}

func TestTDXExHQRoutes(t *testing.T) {
	provider := DefaultTDXProvider()
	provider.FetchExMarkets = func(context.Context, tdx.ExQuoteClientOptions) ([]tdx.ExMarket, error) {
		return []tdx.ExMarket{{Market: 47}}, nil
	}
	provider.FetchExInstrumentCount = func(context.Context, tdx.ExQuoteClientOptions) (int, error) {
		return 123, nil
	}
	provider.FetchExInstruments = func(context.Context, int, int, tdx.ExQuoteClientOptions) ([]tdx.ExInstrument, error) {
		return []tdx.ExInstrument{{Market: 47, Code: "IF1709"}}, nil
	}
	provider.FetchExQuote = func(_ context.Context, req tdx.ExQuoteRequest, _ tdx.ExQuoteClientOptions) (tdx.ExQuote, error) {
		return tdx.ExQuote{Market: req.Market, Code: req.Code, Price: 1}, nil
	}
	provider.FetchExBars = func(_ context.Context, req tdx.ExBarsRequest, _ tdx.ExQuoteClientOptions) ([]tdx.ExBar, error) {
		return []tdx.ExBar{{Market: req.Market, Code: req.Code}}, nil
	}
	provider.FetchExHistoryMinuteTime = func(_ context.Context, req tdx.ExQuoteRequest, date int, _ tdx.ExQuoteClientOptions) ([]tdx.ExMinutePoint, error) {
		return []tdx.ExMinutePoint{{Market: req.Market, Code: req.Code, Date: fmt.Sprint(date)}}, nil
	}
	provider.FetchExTransactions = func(_ context.Context, req tdx.ExQuoteRequest, start, count int, _ tdx.ExQuoteClientOptions) ([]tdx.ExTransaction, error) {
		return []tdx.ExTransaction{{Market: req.Market, Code: req.Code}}, nil
	}
	provider.FetchExHistoryBarsRange = func(_ context.Context, req tdx.ExQuoteRequest, startDate, endDate int, _ tdx.ExQuoteClientOptions) ([]tdx.ExBar, error) {
		return []tdx.ExBar{{Market: req.Market, Code: req.Code}}, nil
	}
	server := httptest.NewServer(NewServerWithTDXProvider(&fakeRepo{}, provider).Handler())
	defer server.Close()

	paths := []string{
		"/api/tdx/exhq/markets",
		"/api/tdx/exhq/count",
		"/api/tdx/exhq/instruments?start=0&count=1",
		"/api/tdx/exhq/quote?market=47&code=IF1709",
		"/api/tdx/exhq/bars?market=47&code=IF1709&count=1",
		"/api/tdx/exhq/minute?market=47&code=IF1709&date=20260605",
		"/api/tdx/exhq/transactions?market=47&code=IF1709&count=1",
		"/api/tdx/exhq/history-bars?market=47&code=IF1709&start_date=20260601&end_date=20260605",
	}
	for _, path := range paths {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status=%d", path, resp.StatusCode)
		}
	}
}

func TestTDXProviderErrorMapping(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{"upstream", fmt.Errorf("TDX HQ quote request failed on 1 server(s): connect TDX HQ server x: timeout"), http.StatusServiceUnavailable},
		{"protocol", fmt.Errorf("decode TDX HQ quote response from x: response too short"), http.StatusBadGateway},
		{"validation", TDXValidationError{"bad input"}, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := DefaultTDXProvider()
			provider.FetchRealtimeQuotes = func(context.Context, []tdx.QuoteRequest, tdx.QuoteClientOptions) ([]tdx.Quote, error) {
				return nil, tt.err
			}
			server := httptest.NewServer(NewServerWithTDXProvider(&fakeRepo{}, provider).Handler())
			defer server.Close()
			resp, err := http.Get(server.URL + "/api/tdx/hq/quotes?symbol=600519")
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tt.status {
				t.Fatalf("status=%d want=%d", resp.StatusCode, tt.status)
			}
		})
	}
}

func TestTDXProviderRoutesDoNotUseRepository(t *testing.T) {
	provider := DefaultTDXProvider()
	provider.FetchRealtimeQuotes = func(context.Context, []tdx.QuoteRequest, tdx.QuoteClientOptions) ([]tdx.Quote, error) {
		return []tdx.Quote{{Market: "sh", Symbol: "600519"}}, nil
	}
	repo := &fakeRepo{}
	server := httptest.NewServer(NewServerWithTDXProvider(repo, provider).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/tdx/hq/quotes?symbol=600519")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if repo.query.Symbol != "" {
		t.Fatalf("repo bars query was used: %+v", repo.query)
	}
}
