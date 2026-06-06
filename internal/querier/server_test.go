package querier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeRepo struct {
	healthErr error
	query     BarQuery
}

func (r *fakeRepo) Health(context.Context) error {
	return r.healthErr
}

func (r *fakeRepo) Bars(_ context.Context, query BarQuery) (BarResult, error) {
	normalized, err := NormalizeQuery(query)
	if err != nil {
		return BarResult{}, err
	}
	r.query = normalized
	return BarResult{
		Query: normalized,
		Bars: []Bar{
			{
				Market:    normalized.Market,
				Symbol:    normalized.Symbol,
				Period:    normalized.Period,
				TradeDate: "2026-06-05",
				Open:      12.34,
				High:      13.00,
				Low:       12.00,
				Close:     12.88,
				Volume:    123456,
				Amount:    100000,
			},
		},
	}, nil
}

func TestServerBars(t *testing.T) {
	repo := &fakeRepo{}
	server := httptest.NewServer(NewServer(repo).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/bars?market=sh&symbol=600519&period=1d&limit=5")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var result BarResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Bars) != 1 {
		t.Fatalf("bars=%d", len(result.Bars))
	}
	if repo.query.Limit != 5 {
		t.Fatalf("limit=%d", repo.query.Limit)
	}
}

func TestServerBarsValidation(t *testing.T) {
	repo := &fakeRepo{}
	server := httptest.NewServer(NewServer(repo).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/bars?market=bad&symbol=600519&period=1d")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestServerHealthIncludesVersion(t *testing.T) {
	repo := &fakeRepo{}
	server := httptest.NewServer(NewServer(repo).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var health Health
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatal(err)
	}
	if health.Status != "ok" || health.Version == "" || health.SchemaVersion == "" {
		t.Fatalf("health=%+v", health)
	}
}

func TestServerResolveSymbol(t *testing.T) {
	repo := &fakeRepo{}
	server := httptest.NewServer(NewServer(repo).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/resolve-symbol?symbol=920002")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var result SymbolResolution
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Market != "bj" || result.Symbol != "920002" {
		t.Fatalf("result=%+v", result)
	}
}

func TestHTTPClientBars(t *testing.T) {
	repo := &fakeRepo{}
	server := httptest.NewServer(NewServer(repo).Handler())
	defer server.Close()

	client := NewHTTPClient(server.URL, server.Client())
	result, err := client.Bars(context.Background(), BarQuery{Market: "sh", Symbol: "600519", Period: "1d", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bars) != 1 {
		t.Fatalf("bars=%d", len(result.Bars))
	}
}

func TestHTTPClientResolveSymbol(t *testing.T) {
	repo := &fakeRepo{}
	server := httptest.NewServer(NewServer(repo).Handler())
	defer server.Close()

	client := NewHTTPClient(server.URL, server.Client())
	result, err := client.ResolveSymbol(context.Background(), "600519")
	if err != nil {
		t.Fatal(err)
	}
	if result.Market != "sh" {
		t.Fatalf("result=%+v", result)
	}
}
