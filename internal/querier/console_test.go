package querier

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

func TestConsoleSummaryRoute(t *testing.T) {
	repo := &fakeRepo{}
	server := httptest.NewServer(NewServer(repo).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/console/summary?limit=3")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var summary ConsoleSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		t.Fatal(err)
	}
	if summary.Health.Status != "ok" || summary.Health.SchemaVersion == "" {
		t.Fatalf("summary health=%+v", summary.Health)
	}
	if len(summary.Watermarks) != 1 || len(summary.TaskRuns) != 1 || len(summary.DataQualityIssueCounts) != 1 || len(summary.QuoteServiceRuns) != 1 {
		t.Fatalf("summary=%+v", summary)
	}
	if repo.limit != 3 {
		t.Fatalf("limit=%d", repo.limit)
	}
}

func TestConsoleSummaryRouteReturnsArraysForEmptyData(t *testing.T) {
	repo := &fakeRepo{emptyConsole: true}
	server := httptest.NewServer(NewServer(repo).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/console/summary")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var payload map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"watermarks", "task_runs", "data_quality_issue_counts", "quote_service_runs"} {
		if string(payload[field]) != "[]" {
			t.Fatalf("%s=%s", field, payload[field])
		}
	}
}

func TestConsoleQuoteRunsReturnArrayMarkets(t *testing.T) {
	repo := &fakeRepo{nilQuoteRunMarkets: true}
	server := httptest.NewServer(NewServer(repo).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/console/quote-service/runs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var runs []ConsoleQuoteServiceRun
	if err := json.NewDecoder(resp.Body).Decode(&runs); err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs=%+v", runs)
	}
	if runs[0].Markets == nil || len(runs[0].Markets) != 0 {
		t.Fatalf("markets=%+v", runs[0].Markets)
	}
}

func TestConsoleLimitValidation(t *testing.T) {
	server := httptest.NewServer(NewServer(&fakeRepo{}).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/console/task-runs?limit=201")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestConsoleTDXProbeRoute(t *testing.T) {
	provider := DefaultTDXProvider()
	var gotServers []string
	provider.ProbeHQServers = func(_ context.Context, servers []string, _ tdx.QuoteClientOptions) []tdx.ServerProbeResult {
		gotServers = append([]string(nil), servers...)
		return []tdx.ServerProbeResult{
			{Server: "slow:7709", Success: true, LatencyMS: 20},
			{Server: "fast:7709", Success: true, LatencyMS: 3},
		}
	}
	server := httptest.NewServer(NewServerWithTDXProvider(&fakeRepo{}, provider).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/console/tdx/hq/probe?server=slow:7709,fast:7709")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var result ConsoleProbeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if strings.Join(gotServers, ",") != "slow:7709,fast:7709" || result.Results[0].Server != "fast:7709" || !result.Results[0].Preferred {
		t.Fatalf("servers=%+v result=%+v", gotServers, result)
	}
}

func TestConsoleTDXProbeRejectsInvalidServer(t *testing.T) {
	server := httptest.NewServer(NewServer(&fakeRepo{}).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/console/tdx/hq/probe?server=bad")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestConsoleTDXQuoteSmokeRoute(t *testing.T) {
	provider := DefaultTDXProvider()
	var gotRequests []tdx.QuoteRequest
	provider.FetchRealtimeQuotes = func(_ context.Context, requests []tdx.QuoteRequest, _ tdx.QuoteClientOptions) ([]tdx.Quote, error) {
		gotRequests = append([]tdx.QuoteRequest(nil), requests...)
		return []tdx.Quote{{Market: "sh", Symbol: "600519", Price: 12.34}}, nil
	}
	server := httptest.NewServer(NewServerWithTDXProvider(&fakeRepo{}, provider).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/console/tdx/hq/quotes?symbol=sh:600519")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var result ConsoleQuoteSmokeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(gotRequests) != 1 || gotRequests[0].Symbol != "600519" || len(result.Quotes) != 1 {
		t.Fatalf("requests=%+v result=%+v", gotRequests, result)
	}
}

func TestConsoleTDXQuoteSmokeUpstreamError(t *testing.T) {
	provider := DefaultTDXProvider()
	provider.FetchRealtimeQuotes = func(context.Context, []tdx.QuoteRequest, tdx.QuoteClientOptions) ([]tdx.Quote, error) {
		return nil, errors.New("upstream unavailable")
	}
	server := httptest.NewServer(NewServerWithTDXProvider(&fakeRepo{}, provider).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/console/tdx/hq/quotes?symbol=sh:600519")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestConsoleBestIPStatusAndRefresh(t *testing.T) {
	provider := DefaultTDXProvider()
	now := time.Now().UTC() // anchored to real time so the cache ExpiresAt stays in the future
	provider.LoadHQBestIPCache = func(path string) (tdx.HQBestIPCache, error) {
		if path != "/tmp/bestip.json" {
			t.Fatalf("path=%s", path)
		}
		return tdx.HQBestIPCache{
			GeneratedAt: now,
			ExpiresAt:   now.Add(time.Hour),
			Preferred:   "fast:7709",
			Results:     []tdx.ServerProbeResult{{Server: "fast:7709", Success: true, LatencyMS: 1, Preferred: true}},
		}, nil
	}
	var refreshedServers []string
	provider.RefreshHQBestIPCache = func(_ context.Context, servers []string, opts tdx.QuoteClientOptions) (tdx.HQBestIPCache, error) {
		refreshedServers = append([]string(nil), servers...)
		if opts.BestIPCachePath != "/tmp/bestip.json" || opts.BestIPMaxAge != 2*time.Hour {
			t.Fatalf("opts=%+v", opts)
		}
		return tdx.HQBestIPCache{
			GeneratedAt: now,
			ExpiresAt:   now.Add(2 * time.Hour),
			Preferred:   "fresh:7709",
			Results:     []tdx.ServerProbeResult{{Server: "fresh:7709", Success: true, LatencyMS: 2, Preferred: true}},
		}, nil
	}
	server := httptest.NewServer(NewServerWithTDXProvider(&fakeRepo{}, provider).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/console/bestip?cache=/tmp/bestip.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var status ConsoleBestIPStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if !status.Usable || status.Preferred != "fast:7709" {
		t.Fatalf("status=%+v", status)
	}

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/console/bestip/refresh?cache=/tmp/bestip.json&max-age=2h&server=fresh:7709", nil)
	if err != nil {
		t.Fatal(err)
	}
	refreshResp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer refreshResp.Body.Close()
	if refreshResp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", refreshResp.StatusCode)
	}
	if strings.Join(refreshedServers, ",") != "fresh:7709" {
		t.Fatalf("servers=%+v", refreshedServers)
	}
}

func TestConsoleBestIPMissingCacheReturnsEmptyResults(t *testing.T) {
	provider := DefaultTDXProvider()
	provider.LoadHQBestIPCache = func(string) (tdx.HQBestIPCache, error) {
		return tdx.HQBestIPCache{}, os.ErrNotExist
	}
	server := httptest.NewServer(NewServerWithTDXProvider(&fakeRepo{}, provider).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/console/bestip")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var status ConsoleBestIPStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if status.Usable || status.Error != "" {
		t.Fatalf("status=%+v", status)
	}
	if status.Results == nil || len(status.Results) != 0 {
		t.Fatalf("results=%+v", status.Results)
	}
}

func TestConsoleImportHQDailyRoute(t *testing.T) {
	var got ConsoleHQDailyImportRequest
	importer := func(_ context.Context, req ConsoleHQDailyImportRequest) (ConsoleHQDailyImportSummary, error) {
		got = req
		return ConsoleHQDailyImportSummary{
			RunID:        "run-1",
			Dataset:      "a_share_bars_1d",
			TargetTable:  "a_share_bars_1d",
			Market:       req.Market,
			Symbol:       req.Symbol,
			Since:        req.Since,
			Until:        req.Until,
			Start:        req.Start,
			Count:        req.Count,
			PagesFetched: 1,
			RowsFetched:  2,
			RowsWritten:  2,
			DryRun:       req.DryRun,
		}, nil
	}
	server := httptest.NewServer(NewServer(&fakeRepo{}).WithConsoleHQDailyImporter(importer).Handler())
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/console/imports/tdx-hq-day?market=sh&symbol=600519&since=20260604&until=2026-06-05&start=10&count=2&server=a:7709,b:7709&dry_run=true", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var summary ConsoleHQDailyImportSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		t.Fatal(err)
	}
	if got.Market != "sh" || got.Symbol != "600519" || got.Start != 10 || got.Count != 2 || !got.DryRun || strings.Join(got.Servers, ",") != "a:7709,b:7709" {
		t.Fatalf("got=%+v", got)
	}
	if summary.RowsWritten != 2 || summary.Dataset != "a_share_bars_1d" {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestConsoleImportHQDailyRejectsInvalidRequest(t *testing.T) {
	called := false
	importer := func(context.Context, ConsoleHQDailyImportRequest) (ConsoleHQDailyImportSummary, error) {
		called = true
		return ConsoleHQDailyImportSummary{}, nil
	}
	server := httptest.NewServer(NewServer(&fakeRepo{}).WithConsoleHQDailyImporter(importer).Handler())
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/console/imports/tdx-hq-day?market=sh&symbol=600519&since=2026-06-06&until=2026-06-05", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if called {
		t.Fatal("importer should not be called")
	}
}

func TestTDXProviderBarsDoesNotRunConsoleImporter(t *testing.T) {
	provider := DefaultTDXProvider()
	provider.FetchHQSecurityBars = func(_ context.Context, req tdx.HQBarsRequest, _ tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
		return []tdx.HQBar{{Market: req.Market, Symbol: req.Symbol, Category: req.Category, DateTime: "2026-06-05 00:00", Year: 2026, Month: 6, Day: 5}}, nil
	}
	importer := func(context.Context, ConsoleHQDailyImportRequest) (ConsoleHQDailyImportSummary, error) {
		t.Fatal("importer should not be called")
		return ConsoleHQDailyImportSummary{}, nil
	}
	server := httptest.NewServer(NewServerWithTDXProvider(&fakeRepo{}, provider).WithConsoleHQDailyImporter(importer).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/tdx/hq/bars?market=sh&symbol=600519&category=9&count=1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}
