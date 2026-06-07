package querier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeRepo struct {
	healthErr     error
	query         BarQuery
	intradayQuery IntradayPointQuery
	limit         int
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

func (r *fakeRepo) IntradayPoints(_ context.Context, query IntradayPointQuery) (IntradayPointResult, error) {
	normalized, err := NormalizeIntradayPointQuery(query)
	if err != nil {
		return IntradayPointResult{}, err
	}
	r.intradayQuery = normalized
	r.limit = normalized.Limit
	return IntradayPointResult{
		Query: normalized,
		Points: []IntradayPoint{
			{
				Market:     normalized.Market,
				Symbol:     normalized.Symbol,
				TradeDate:  "2026-06-05",
				PointTime:  time.Date(2026, 6, 5, 9, 30, 0, 0, time.UTC),
				PointIndex: 1,
				Price:      12.34,
				Volume:     100,
			},
		},
	}, nil
}

func (r *fakeRepo) ConsoleWatermarks(_ context.Context, limit int) ([]ConsoleWatermark, error) {
	r.limit = limit
	return []ConsoleWatermark{{
		Dataset:     "tdx-day",
		Asset:       "sh:600519",
		Status:      "succeeded",
		RowsWritten: 10,
		Message:     "ok",
		UpdatedAt:   time.Date(2026, 6, 7, 9, 30, 0, 0, time.UTC),
	}}, nil
}

func (r *fakeRepo) ConsoleTaskRuns(_ context.Context, limit int) ([]ConsoleTaskRun, error) {
	r.limit = limit
	return []ConsoleTaskRun{{
		RunID:       "run-1",
		Dataset:     "tdx-day",
		TaskType:    "import",
		Status:      "succeeded",
		TargetTable: "a_share_bars_1d",
		StartedAt:   time.Date(2026, 6, 7, 9, 0, 0, 0, time.UTC),
		RowsWritten: 10,
		UpdatedAt:   time.Date(2026, 6, 7, 9, 1, 0, 0, time.UTC),
	}}, nil
}

func (r *fakeRepo) ConsoleDataQualityIssues(_ context.Context, limit int) ([]ConsoleDataQualityIssue, error) {
	r.limit = limit
	return []ConsoleDataQualityIssue{{
		IssueID:    "issue-1",
		RunID:      "run-1",
		Dataset:    "tdx-day",
		Severity:   "warn",
		IssueType:  "duplicate",
		Market:     "sh",
		Symbol:     "600519",
		LogicalKey: "sh:600519:2026-06-07",
		ObservedAt: time.Date(2026, 6, 7, 9, 0, 0, 0, time.UTC),
		Message:    "duplicate row",
	}}, nil
}

func (r *fakeRepo) ConsoleDataQualityIssueStats(_ context.Context, limit int) ([]ConsoleQualityIssueStat, error) {
	r.limit = limit
	return []ConsoleQualityIssueStat{{Dataset: "tdx-day", Severity: "warn", IssueType: "duplicate", Count: 1}}, nil
}

func (r *fakeRepo) ConsoleQuoteServiceRuns(_ context.Context, limit int) ([]ConsoleQuoteServiceRun, error) {
	r.limit = limit
	return []ConsoleQuoteServiceRun{{
		RunID:          "quote-run-1",
		Status:         "succeeded",
		Markets:        []string{"sh", "sz"},
		SymbolSource:   "online",
		BatchSize:      80,
		PlannedBatches: 1,
		RowsFetched:    80,
		StartedAt:      time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 6, 7, 10, 1, 0, 0, time.UTC),
	}}, nil
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
	if result.Query.Adjust != "none" {
		t.Fatalf("adjust=%q", result.Query.Adjust)
	}
}

func TestServerBarsAdjust(t *testing.T) {
	repo := &fakeRepo{}
	server := httptest.NewServer(NewServer(repo).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/bars?market=sh&symbol=600519&period=1d&adjust=qfq&limit=5")
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
	if repo.query.Adjust != "qfq" || result.Query.Adjust != "qfq" {
		t.Fatalf("adjust repo=%q result=%q", repo.query.Adjust, result.Query.Adjust)
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

func TestServerBarsAdjustValidation(t *testing.T) {
	repo := &fakeRepo{}
	server := httptest.NewServer(NewServer(repo).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/bars?market=sh&symbol=600519&period=1d&adjust=bad")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if repo.query.Symbol != "" {
		t.Fatalf("repo was called: %+v", repo.query)
	}
}

func TestServerIntradayPoints(t *testing.T) {
	repo := &fakeRepo{}
	server := httptest.NewServer(NewServer(repo).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/intraday-points?market=sh&symbol=600519&date=20260605&limit=240")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var result IntradayPointResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Points) != 1 {
		t.Fatalf("points=%d", len(result.Points))
	}
	if result.Query.Date != "2026-06-05" || repo.intradayQuery.Limit != 240 {
		t.Fatalf("query=%+v repo=%+v", result.Query, repo.intradayQuery)
	}
}

func TestServerIntradayPointsValidation(t *testing.T) {
	repo := &fakeRepo{}
	server := httptest.NewServer(NewServer(repo).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/intraday-points?market=bad&symbol=600519&date=2026-06-05")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if repo.intradayQuery.Symbol != "" {
		t.Fatalf("repo was called: %+v", repo.intradayQuery)
	}
}

func TestMissingAdjustmentFactorMapsToConflict(t *testing.T) {
	err := MissingAdjustmentFactorError{Message: "missing factor"}
	if got := statusForError(err); got != http.StatusConflict {
		t.Fatalf("status=%d", got)
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
	result, err := client.Bars(context.Background(), BarQuery{Market: "sh", Symbol: "600519", Period: "1d", Adjust: "hfq", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bars) != 1 {
		t.Fatalf("bars=%d", len(result.Bars))
	}
	if repo.query.Adjust != "hfq" {
		t.Fatalf("adjust=%q", repo.query.Adjust)
	}
}

func TestHTTPClientIntradayPoints(t *testing.T) {
	repo := &fakeRepo{}
	server := httptest.NewServer(NewServer(repo).Handler())
	defer server.Close()

	client := NewHTTPClient(server.URL, server.Client())
	result, err := client.IntradayPoints(context.Background(), IntradayPointQuery{Market: "sh", Symbol: "600519", Since: "2026-06-05 09:30:00", Until: "2026-06-05 15:00:00", Limit: 240})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Points) != 1 {
		t.Fatalf("points=%d", len(result.Points))
	}
	if repo.intradayQuery.Since == "" || repo.intradayQuery.Until == "" {
		t.Fatalf("query=%+v", repo.intradayQuery)
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

func TestConsoleStaticServingWithSPAFallback(t *testing.T) {
	dist := t.TempDir()
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte("<html>console</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dist, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "assets", "app.js"), []byte("console.log('ok')"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewServer(&fakeRepo{}).HandlerWithConsoleDist(dist))
	defer server.Close()

	for _, path := range []string{"/console/", "/console/ops/watermarks"} {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status=%d", path, resp.StatusCode)
		}
	}
	rootResp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer rootResp.Body.Close()
	if rootResp.Request.URL.Path != "/console/" || rootResp.StatusCode != http.StatusOK {
		t.Fatalf("root path=%s status=%d", rootResp.Request.URL.Path, rootResp.StatusCode)
	}
	resp, err := http.Get(server.URL + "/console/assets/app.js")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("asset status=%d", resp.StatusCode)
	}
}

func TestConsoleStaticDoesNotCaptureAPIRoutes(t *testing.T) {
	dist := t.TempDir()
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte("<html>console</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewServer(&fakeRepo{}).HandlerWithConsoleDist(dist))
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
	if health.Status != "ok" {
		t.Fatalf("health=%+v", health)
	}
}
