package querier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/onlineadjust"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

func advancedTestServer(t *testing.T, configure func(*TDXProvider)) *httptest.Server {
	t.Helper()
	provider := DefaultTDXProvider()
	configure(provider)
	return httptest.NewServer(NewServerWithTDXProvider(&fakeRepo{}, provider).Handler())
}

func getStatus(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	return resp.StatusCode, string(buf[:n])
}

func TestTDXHQQuotesListRoute(t *testing.T) {
	var gotSort uint16
	server := advancedTestServer(t, func(p *TDXProvider) {
		p.FetchHQQuotesList = func(_ context.Context, req tdx.HQQuotesListRequest, _ tdx.QuoteClientOptions) ([]tdx.HQQuotesListItem, error) {
			gotSort = req.SortType
			return []tdx.HQQuotesListItem{{Market: "sh", Symbol: "600519"}}, nil
		}
	})
	defer server.Close()
	code, body := getStatus(t, server.URL+"/api/tdx/hq/quotes-list?category=0&sort=turnover&count=5&exclude=st,cyb")
	if code != http.StatusOK || !strings.Contains(body, "600519") {
		t.Fatalf("code=%d body=%s", code, body)
	}
	if gotSort != tdx.QuotesSortTurnover {
		t.Fatalf("sort=%#x", gotSort)
	}
}

func TestTDXHQCompactQuotesRoute(t *testing.T) {
	server := advancedTestServer(t, func(p *TDXProvider) {
		p.FetchHQCompactBatchQuotes = func(_ context.Context, requests []tdx.QuoteRequest, _ tdx.QuoteClientOptions) ([]tdx.HQCompactBatchQuote, error) {
			if len(requests) != 1 || requests[0].Symbol != "600519" {
				t.Fatalf("requests = %+v", requests)
			}
			return []tdx.HQCompactBatchQuote{{Market: "sh", Symbol: "600519"}}, nil
		}
	})
	defer server.Close()
	if code, body := getStatus(t, server.URL+"/api/tdx/hq/compact-quotes?symbol=sh:600519"); code != http.StatusOK || !strings.Contains(body, "600519") {
		t.Fatalf("code=%d body=%s", code, body)
	}
}

func TestTDXHQCompactQuotesRejectsTooManySymbols(t *testing.T) {
	server := advancedTestServer(t, func(p *TDXProvider) {
		p.FetchHQCompactBatchQuotes = func(context.Context, []tdx.QuoteRequest, tdx.QuoteClientOptions) ([]tdx.HQCompactBatchQuote, error) {
			t.Fatal("provider should not be called")
			return nil, nil
		}
	})
	defer server.Close()

	var path strings.Builder
	path.WriteString(server.URL + "/api/tdx/hq/compact-quotes")
	for i := 0; i < tdx.MaxCompactBatchQuoteCount+1; i++ {
		if i == 0 {
			path.WriteByte('?')
		} else {
			path.WriteByte('&')
		}
		path.WriteString("symbol=sh:600519")
	}
	if code, _ := getStatus(t, path.String()); code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
}

func TestTDXHQTickChartRoute(t *testing.T) {
	server := advancedTestServer(t, func(p *TDXProvider) {
		p.FetchHQTickChart = func(_ context.Context, req tdx.HQTickChartRequest, _ tdx.QuoteClientOptions) ([]tdx.HQTickChartPoint, error) {
			if req.Count != 2 || req.Symbol != "600519" {
				t.Fatalf("req = %+v", req)
			}
			return []tdx.HQTickChartPoint{{Market: "sh", Symbol: "600519", Time: "09:30"}}, nil
		}
	})
	defer server.Close()
	if code, body := getStatus(t, server.URL+"/api/tdx/hq/tick-chart?market=sh&symbol=600519&count=2"); code != http.StatusOK || !strings.Contains(body, "09:30") {
		t.Fatalf("code=%d body=%s", code, body)
	}
}

func TestTDXHQTickChartRejectsOverflowPage(t *testing.T) {
	server := advancedTestServer(t, func(p *TDXProvider) {
		p.FetchHQTickChart = func(context.Context, tdx.HQTickChartRequest, tdx.QuoteClientOptions) ([]tdx.HQTickChartPoint, error) {
			t.Fatal("provider should not be called")
			return nil, nil
		}
	})
	defer server.Close()
	if code, _ := getStatus(t, server.URL+"/api/tdx/hq/tick-chart?market=sh&symbol=600519&start=1&count=240"); code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
}

func TestTDXHQTopBoardRoute(t *testing.T) {
	server := advancedTestServer(t, func(p *TDXProvider) {
		p.FetchHQTopBoard = func(_ context.Context, _ uint16, _ int, _ tdx.QuoteClientOptions) ([]tdx.HQTopBoardGroup, error) {
			return []tdx.HQTopBoardGroup{{Name: "gainers"}}, nil
		}
	})
	defer server.Close()
	if code, body := getStatus(t, server.URL+"/api/tdx/hq/top-board?category=0&size=3"); code != http.StatusOK || !strings.Contains(body, "gainers") {
		t.Fatalf("code=%d body=%s", code, body)
	}
}

func TestTDXSPBoardMembersRequiresServer(t *testing.T) {
	server := advancedTestServer(t, func(p *TDXProvider) {})
	defer server.Close()
	if code, _ := getStatus(t, server.URL+"/api/tdx/sp/board-members?board=880472"); code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
}

func TestTDXSPBoardMembersRoute(t *testing.T) {
	server := advancedTestServer(t, func(p *TDXProvider) {
		p.FetchSPBoardMembers = func(_ context.Context, srv, board string, _ uint16, _ int, _ uint16, _ time.Duration) ([]tdx.HQBoardMember, error) {
			return []tdx.HQBoardMember{{Market: "sh", Symbol: "600519"}}, nil
		}
	})
	defer server.Close()
	if code, body := getStatus(t, server.URL+"/api/tdx/sp/board-members?server=x:7709&board=880472&count=1"); code != http.StatusOK || !strings.Contains(body, "600519") {
		t.Fatalf("code=%d body=%s", code, body)
	}
}

func TestTDXSPBoardMembersBestRoute(t *testing.T) {
	server := advancedTestServer(t, func(p *TDXProvider) {
		p.ProbeSPServers = func(context.Context, []string, time.Duration) []tdx.ServerProbeResult {
			return []tdx.ServerProbeResult{{Server: "best:7709", Success: true}}
		}
		p.FetchSPBoardMembers = func(_ context.Context, srv, board string, _ uint16, _ int, _ uint16, _ time.Duration) ([]tdx.HQBoardMember, error) {
			if srv != "best:7709" {
				t.Fatalf("server = %q", srv)
			}
			return []tdx.HQBoardMember{{Market: "sh", Symbol: "600519"}}, nil
		}
	})
	defer server.Close()
	if code, body := getStatus(t, server.URL+"/api/tdx/sp/board-members?best=true&board=880472&count=1"); code != http.StatusOK || !strings.Contains(body, "600519") {
		t.Fatalf("code=%d body=%s", code, body)
	}
}

func TestTDXSPBoardMembersBestNoReachableServer(t *testing.T) {
	server := advancedTestServer(t, func(p *TDXProvider) {
		p.ProbeSPServers = func(context.Context, []string, time.Duration) []tdx.ServerProbeResult {
			return []tdx.ServerProbeResult{{Server: "bad:7709", Success: false, Error: "timeout"}}
		}
		p.FetchSPBoardMembers = func(context.Context, string, string, uint16, int, uint16, time.Duration) ([]tdx.HQBoardMember, error) {
			t.Fatal("provider should not be called")
			return nil, nil
		}
	})
	defer server.Close()
	if code, _ := getStatus(t, server.URL+"/api/tdx/sp/board-members?best=true&board=880472"); code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", code)
	}
}

func TestTDXSPServerRoutes(t *testing.T) {
	server := advancedTestServer(t, func(p *TDXProvider) {
		p.SPServerCandidates = func() []tdx.TDXServerCandidate {
			return []tdx.TDXServerCandidate{{Protocol: "sp", Server: "x:7709"}}
		}
		p.ProbeSPServers = func(context.Context, []string, time.Duration) []tdx.ServerProbeResult {
			return []tdx.ServerProbeResult{{Server: "x:7709", Success: true}}
		}
	})
	defer server.Close()
	if code, body := getStatus(t, server.URL+"/api/tdx/sp/servers"); code != http.StatusOK || !strings.Contains(body, "x:7709") {
		t.Fatalf("servers: code=%d body=%s", code, body)
	}
	if code, body := getStatus(t, server.URL+"/api/tdx/sp/probe?server=x:7709"); code != http.StatusOK || !strings.Contains(body, "x:7709") {
		t.Fatalf("probe: code=%d body=%s", code, body)
	}
}

func TestTDXFundRoutes(t *testing.T) {
	server := advancedTestServer(t, func(p *TDXProvider) {
		p.FetchFundKline = func(_ context.Context, _, _, _ string, _ int, _ time.Duration) ([]tdx.HQFundBar, error) {
			return []tdx.HQFundBar{{Time: "2026-06-05", Close: 1.5}}, nil
		}
		p.FetchFundDetail = func(_ context.Context, _, code string, _ uint16, _ time.Duration) (tdx.HQFundDetail, error) {
			return tdx.HQFundDetail{Code: code}, nil
		}
	})
	defer server.Close()

	if code, _ := getStatus(t, server.URL+"/api/tdx/fund/kline?code=159915"); code != http.StatusBadRequest {
		t.Fatalf("fund kline without server should be 400, got %d", code)
	}
	if code, body := getStatus(t, server.URL+"/api/tdx/fund/kline?server=x:7727&code=159915&period=day"); code != http.StatusOK || !strings.Contains(body, "2026-06-05") {
		t.Fatalf("fund kline: code=%d body=%s", code, body)
	}
	if code, body := getStatus(t, server.URL+"/api/tdx/fund/detail?server=x:7727&code=159915"); code != http.StatusOK || !strings.Contains(body, "159915") {
		t.Fatalf("fund detail: code=%d body=%s", code, body)
	}
}

func TestTDXFundBestRoutes(t *testing.T) {
	server := advancedTestServer(t, func(p *TDXProvider) {
		p.ProbeFundServers = func(context.Context, []string, time.Duration) []tdx.ServerProbeResult {
			return []tdx.ServerProbeResult{{Server: "best:7727", Success: true}}
		}
		p.FetchFundKline = func(_ context.Context, srv, _, _ string, _ int, _ time.Duration) ([]tdx.HQFundBar, error) {
			if srv != "best:7727" {
				t.Fatalf("server = %q", srv)
			}
			return []tdx.HQFundBar{{Time: "2026-06-05"}}, nil
		}
		p.FetchFundDetail = func(_ context.Context, srv, code string, _ uint16, _ time.Duration) (tdx.HQFundDetail, error) {
			if srv != "best:7727" {
				t.Fatalf("server = %q", srv)
			}
			return tdx.HQFundDetail{Code: code}, nil
		}
	})
	defer server.Close()
	if code, body := getStatus(t, server.URL+"/api/tdx/fund/kline?best=true&code=159915&period=day"); code != http.StatusOK || !strings.Contains(body, "2026-06-05") {
		t.Fatalf("fund kline: code=%d body=%s", code, body)
	}
	if code, body := getStatus(t, server.URL+"/api/tdx/fund/detail?best=true&code=159915"); code != http.StatusOK || !strings.Contains(body, "159915") {
		t.Fatalf("fund detail: code=%d body=%s", code, body)
	}
}

func TestTDXFundBestNoReachableServer(t *testing.T) {
	server := advancedTestServer(t, func(p *TDXProvider) {
		p.ProbeFundServers = func(context.Context, []string, time.Duration) []tdx.ServerProbeResult {
			return []tdx.ServerProbeResult{{Server: "bad:7727", Success: false, Error: "timeout"}}
		}
		p.FetchFundKline = func(context.Context, string, string, string, int, time.Duration) ([]tdx.HQFundBar, error) {
			t.Fatal("provider should not be called")
			return nil, nil
		}
	})
	defer server.Close()
	if code, _ := getStatus(t, server.URL+"/api/tdx/fund/kline?best=true&code=159915&period=day"); code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", code)
	}
}

func TestTDXFundServerRoutes(t *testing.T) {
	server := advancedTestServer(t, func(p *TDXProvider) {
		p.FundServerCandidates = func() []tdx.TDXServerCandidate {
			return []tdx.TDXServerCandidate{{Protocol: "fund", Server: "x:7727"}}
		}
		p.ProbeFundServers = func(context.Context, []string, time.Duration) []tdx.ServerProbeResult {
			return []tdx.ServerProbeResult{{Server: "x:7727", Success: true}}
		}
	})
	defer server.Close()
	if code, body := getStatus(t, server.URL+"/api/tdx/fund/servers"); code != http.StatusOK || !strings.Contains(body, "x:7727") {
		t.Fatalf("servers: code=%d body=%s", code, body)
	}
	if code, body := getStatus(t, server.URL+"/api/tdx/fund/probe?server=x:7727"); code != http.StatusOK || !strings.Contains(body, "x:7727") {
		t.Fatalf("probe: code=%d body=%s", code, body)
	}
}

func TestTDXHQAdjustedBarsOnlineRoute(t *testing.T) {
	server := advancedTestServer(t, func(p *TDXProvider) {
		p.FetchHQAdjustedBarsOnline = func(_ context.Context, req onlineadjust.HQAdjustedBarsOnlineRequest, _ tdx.QuoteClientOptions) (onlineadjust.HQAdjustedBarsOnlineResult, error) {
			if req.Adjust != "qfq" || req.Symbol != "600519" {
				t.Fatalf("req = %+v", req)
			}
			return onlineadjust.HQAdjustedBarsOnlineResult{Query: req, Source: "tdx-live-provider", Bars: []onlineadjust.HQAdjustedBar{{Market: "sh", Symbol: "600519", Adjust: "qfq"}}}, nil
		}
	})
	defer server.Close()
	if code, body := getStatus(t, server.URL+"/api/tdx/hq/adjusted-bars?market=sh&symbol=600519&adjust=qfq"); code != http.StatusOK || !strings.Contains(body, "tdx-live-provider") {
		t.Fatalf("code=%d body=%s", code, body)
	}
}

func TestTDXHQAdjustedBarsOnlineRoutePreservesCategoryZero(t *testing.T) {
	server := advancedTestServer(t, func(p *TDXProvider) {
		p.FetchHQAdjustedBarsOnline = func(_ context.Context, req onlineadjust.HQAdjustedBarsOnlineRequest, _ tdx.QuoteClientOptions) (onlineadjust.HQAdjustedBarsOnlineResult, error) {
			if req.Category != tdx.HQKLine5Min {
				t.Fatalf("category = %d", req.Category)
			}
			return onlineadjust.HQAdjustedBarsOnlineResult{Query: req, Source: "tdx-live-provider"}, nil
		}
	})
	defer server.Close()
	if code, body := getStatus(t, server.URL+"/api/tdx/hq/adjusted-bars?market=sh&symbol=600519&category=0&count=1&adjust=none"); code != http.StatusOK || !strings.Contains(body, "tdx-live-provider") {
		t.Fatalf("code=%d body=%s", code, body)
	}
}

func TestTDXHQAdjustedBarsOnlineRejectsBadAdjust(t *testing.T) {
	server := advancedTestServer(t, func(p *TDXProvider) {
		p.FetchHQAdjustedBarsOnline = func(context.Context, onlineadjust.HQAdjustedBarsOnlineRequest, tdx.QuoteClientOptions) (onlineadjust.HQAdjustedBarsOnlineResult, error) {
			t.Fatal("provider should not be called")
			return onlineadjust.HQAdjustedBarsOnlineResult{}, nil
		}
	})
	defer server.Close()
	if code, _ := getStatus(t, server.URL+"/api/tdx/hq/adjusted-bars?market=sh&symbol=600519&adjust=bad"); code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
}
