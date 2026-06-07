package infinitycli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/querier"
)

type cliRepo struct{}

func (cliRepo) Health(context.Context) error {
	return nil
}

func (cliRepo) Bars(_ context.Context, query querier.BarQuery) (querier.BarResult, error) {
	normalized, err := querier.NormalizeQuery(query)
	if err != nil {
		return querier.BarResult{}, err
	}
	return querier.BarResult{
		Query: normalized,
		Bars: []querier.Bar{
			{
				Market:    normalized.Market,
				Symbol:    normalized.Symbol,
				Period:    normalized.Period,
				TradeDate: "2026-06-05",
				Open:      12.34,
				High:      13,
				Low:       12,
				Close:     12.88,
				Volume:    123456,
				Amount:    100000,
			},
		},
	}, nil
}

func (cliRepo) IntradayPoints(_ context.Context, query querier.IntradayPointQuery) (querier.IntradayPointResult, error) {
	normalized, err := querier.NormalizeIntradayPointQuery(query)
	if err != nil {
		return querier.IntradayPointResult{}, err
	}
	return querier.IntradayPointResult{
		Query: normalized,
		Points: []querier.IntradayPoint{
			{
				Market:     normalized.Market,
				Symbol:     normalized.Symbol,
				TradeDate:  "2026-06-05",
				PointTime:  time.Date(2026, 6, 5, 9, 30, 0, 0, time.UTC),
				PointIndex: 0,
				Price:      12.34,
				Volume:     100,
			},
		},
	}, nil
}

func (cliRepo) ConsoleWatermarks(context.Context, int) ([]querier.ConsoleWatermark, error) {
	return nil, nil
}

func (cliRepo) ConsoleTaskRuns(context.Context, int) ([]querier.ConsoleTaskRun, error) {
	return nil, nil
}

func (cliRepo) ConsoleDataQualityIssues(context.Context, int) ([]querier.ConsoleDataQualityIssue, error) {
	return nil, nil
}

func (cliRepo) ConsoleDataQualityIssueStats(context.Context, int) ([]querier.ConsoleQualityIssueStat, error) {
	return nil, nil
}

func (cliRepo) ConsoleQuoteServiceRuns(context.Context, int) ([]querier.ConsoleQuoteServiceRun, error) {
	return []querier.ConsoleQuoteServiceRun{{UpdatedAt: time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)}}, nil
}

func TestQuerierHealthCommand(t *testing.T) {
	server := httptest.NewServer(querier.NewServer(cliRepo{}).Handler())
	defer server.Close()

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"querier", "health", "--url", server.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if !strings.Contains(out.String(), "querier: ok") {
		t.Fatalf("output=%s", out.String())
	}
}

func TestQuerierBarsCommand(t *testing.T) {
	server := httptest.NewServer(querier.NewServer(cliRepo{}).Handler())
	defer server.Close()

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"querier", "bars", "--url", server.URL, "--market", "sh", "--symbol", "600519", "--period", "1d", "--adjust", "qfq", "--limit", "1"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if !strings.Contains(out.String(), `"symbol": "600519"`) {
		t.Fatalf("output=%s", out.String())
	}
	if !strings.Contains(out.String(), `"adjust": "qfq"`) {
		t.Fatalf("output=%s", out.String())
	}
}

func TestQuerierIntradayPointsCommand(t *testing.T) {
	server := httptest.NewServer(querier.NewServer(cliRepo{}).Handler())
	defer server.Close()

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"querier", "intraday-points", "--url", server.URL, "--market", "sh", "--symbol", "600519", "--date", "2026-06-05", "--limit", "240"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if !strings.Contains(out.String(), `"points"`) || !strings.Contains(out.String(), `"symbol": "600519"`) {
		t.Fatalf("output=%s", out.String())
	}
}

func TestQuerierResolveSymbolCommand(t *testing.T) {
	server := httptest.NewServer(querier.NewServer(cliRepo{}).Handler())
	defer server.Close()

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"querier", "resolve-symbol", "--url", server.URL, "--symbol", "920002"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if !strings.Contains(out.String(), `"market": "bj"`) {
		t.Fatalf("output=%s", out.String())
	}
}

func TestQuerierBarsCommandReportsServiceError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
	}))
	defer server.Close()

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"querier", "bars", "--url", server.URL, "--market", "sh", "--symbol", "600519"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
}
