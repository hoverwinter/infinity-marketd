package infinitycli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	code := Run(context.Background(), []string{"querier", "bars", "--url", server.URL, "--market", "sh", "--symbol", "600519", "--period", "1d", "--limit", "1"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if !strings.Contains(out.String(), `"symbol": "600519"`) {
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
