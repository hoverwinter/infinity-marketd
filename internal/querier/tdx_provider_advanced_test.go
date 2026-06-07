package querier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
