package querier

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hoverwinter/infinity-marketd/internal/eastmoney"
	"github.com/hoverwinter/infinity-marketd/internal/marketdata"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
	"github.com/hoverwinter/infinity-marketd/internal/ths"
)

type fakeMarketData struct {
	err   error
	calls int
	query marketdata.BarsQuery
}

// Embedding an unset interface satisfies route registration, but any storage
// method invocation panics. Live routes must never call these methods.
type unusedMarketDataRepository struct{ Repository }

// An additional provider requires no changes to the HTTP routing or common DTOs.
func (*fakeMarketData) ID() string { return "example" }
func (*fakeMarketData) BarsCapabilities() []marketdata.BarsCapability {
	return []marketdata.BarsCapability{{Kind: "index", Markets: []string{"board"}, Periods: []string{"1d"}}}
}

func TestDefaultThreeProvidersAndCookiePreservesRegistration(t *testing.T) {
	s := NewServer(unusedMarketDataRepository{})
	for _, cookie := range []string{"", "v=test"} {
		s.WithTHSCookie(cookie)
		infos := s.marketDataProviders.Providers()
		if len(infos) != 3 || infos[0].ID != "eastmoney" || infos[1].ID != "tdx" || infos[2].ID != "ths" {
			t.Fatalf("default providers=%+v", infos)
		}
	}
	custom := &fakeMarketData{}
	r, err := marketdata.NewRegistry(custom, eastmoney.NewClient(eastmoney.Options{}))
	if err != nil {
		t.Fatal(err)
	}
	s.WithMarketDataProviders(r).WithTHSCookie("v=test")
	if infos := s.marketDataProviders.Providers(); len(infos) != 3 || infos[0].ID != "eastmoney" || infos[1].ID != "example" || infos[2].ID != "ths" {
		t.Fatalf("lost custom providers: %+v", infos)
	}
	if _, err := s.marketDataProviders.Bars(context.Background(), "example", marketdata.BarsQuery{}); err != nil || custom.calls != 1 {
		t.Fatalf("custom instance replaced: %v", err)
	}
	if len(r.Providers()) != 2 {
		t.Fatal("cookie configuration mutated original registry")
	}
}

func TestThreeAdaptersThroughCommonHTTP(t *testing.T) {
	var eastmoneyCalls, thsCalls, tdxCalls atomic.Int32
	var failEastmoney atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/qt/clist/get":
			_, _ = w.Write([]byte(`{"rc":0,"data":{"total":1,"diff":[{"f12":"BK1027","f13":90,"f14":"小金属"}]}}`))
		case "/api/qt/stock/kline/get":
			eastmoneyCalls.Add(1)
			if failEastmoney.Load() {
				w.WriteHeader(503)
				return
			}
			if r.URL.Query().Get("secid") != "90.BK1027" {
				t.Errorf("wrong EM identifier: %s", r.URL)
			}
			_, _ = w.Write([]byte(`{"rc":0,"data":{"code":"BK1027","market":90,"klines":["2026-09-04,10,11,12,9,100,1000,0,0,0,0"]}}`))
		case "/v4/line/bk_881270/01/2026.js":
			thsCalls.Add(1)
			_, _ = w.Write([]byte(`quotebridge_v4_line_bk_881270_01_2026({"data":"20260904,10,12,9,11,100,1000,,,,0"})`))
		default:
			t.Errorf("unexpected upstream path %s", r.URL)
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	tdxSource := tdx.NewMarketDataProvider(tdx.MarketDataOptions{FetchIndexBars: func(_ context.Context, req tdx.HQBarsRequest, _ tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
		tdxCalls.Add(1)
		return []tdx.HQBar{{Market: req.Market, Symbol: req.Symbol, Category: req.Category, DateTime: "2026-09-04 15:00", Open: 10, Close: 11, High: 12, Low: 9, Volume: 100, Amount: 1000}}, nil
	}})
	r, err := marketdata.NewRegistry(tdxSource, ths.NewClient(ths.Options{ChartURL: upstream.URL, RequestInterval: -1}), eastmoney.NewClient(eastmoney.Options{QuoteURL: upstream.URL, HistoryURL: upstream.URL, RequestInterval: -1}))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewServer(unusedMarketDataRepository{}).WithMarketDataProviders(r).Handler())
	defer server.Close()
	c := NewHTTPClient(server.URL, nil)
	q := marketdata.BarsQuery{Instrument: marketdata.Instrument{Kind: "index"}, Period: "1d", Since: "2026-09-04", Until: "2026-09-04"}
	for _, item := range []struct{ provider, market, symbol string }{{"eastmoney", "board", "BK1027"}, {"ths", "board", "881270"}, {"tdx", "sh", "000001"}} {
		q.Instrument.Market, q.Instrument.Symbol = item.market, item.symbol
		result, err := c.ProviderBars(context.Background(), item.provider, q)
		if err != nil || result.Provider != item.provider || len(result.Bars) != 1 || result.Bars[0].Close != 11 || result.Bars[0].High != 12 {
			t.Fatalf("%s: %+v %v", item.provider, result, err)
		}
	}
	boards, err := c.ProviderBoards(context.Background(), "eastmoney", "industry")
	if err != nil || len(boards.Boards) != 1 || boards.Scope != "current_provider_catalog" {
		t.Fatalf("%+v %v", boards, err)
	}
	board, err := c.ProviderBoard(context.Background(), "eastmoney", "industry", "BK1027")
	if err != nil || board.Board.Instrument == nil || board.Board.Instrument.Symbol != "BK1027" {
		t.Fatalf("%+v %v", board, err)
	}
	failEastmoney.Store(true)
	q.Instrument = *board.Board.Instrument
	if _, err := c.ProviderBars(context.Background(), "eastmoney", q); err == nil {
		t.Fatal("failed source returned success")
	}
	if eastmoneyCalls.Load() != 2 || thsCalls.Load() != 1 || tdxCalls.Load() != 1 {
		t.Fatal("unexpected cross-source fallback")
	}
}
func (p *fakeMarketData) Bars(_ context.Context, q marketdata.BarsQuery) (marketdata.BarsResult, error) {
	p.calls++
	p.query = q
	return marketdata.BarsResult{Provider: p.ID(), Query: q, Bars: []marketdata.Bar{}}, p.err
}

func TestMarketDataHTTPClientAndStorageIsolation(t *testing.T) {
	p := &fakeMarketData{}
	r, err := marketdata.NewRegistry(p)
	if err != nil {
		t.Fatal(err)
	}
	repo := &fakeRepo{}
	server := httptest.NewServer(NewServer(repo).WithMarketDataProviders(r).Handler())
	defer server.Close()
	client := NewHTTPClient(server.URL, nil)
	infos, err := client.Providers(context.Background())
	if err != nil || len(infos) != 1 || infos[0].ID != "example" {
		t.Fatalf("%+v %v", infos, err)
	}
	q := marketdata.BarsQuery{Instrument: marketdata.Instrument{Kind: "index", Market: "board", Symbol: "BK1234"}, Period: "1d", Adjust: "none", Since: "2026-09-03", Until: "2026-09-04"}
	result, err := client.ProviderBars(context.Background(), "example", q)
	if err != nil || p.query != q || result.Provider != "example" || p.calls != 1 {
		t.Fatalf("%+v %v", result, err)
	}
	if repo.query.Symbol != "" {
		t.Fatal("live provider accessed canonical repository")
	}
	if _, err := client.Bars(context.Background(), BarQuery{Market: "sh", Symbol: "600519", Period: "1d"}); err != nil {
		t.Fatal(err)
	}
	if p.calls != 1 {
		t.Fatal("canonical query invoked live provider")
	}
}

func TestMarketDataErrorMappingAndStrictParams(t *testing.T) {
	p := &fakeMarketData{}
	r, _ := marketdata.NewRegistry(p)
	handler := NewServer(unusedMarketDataRepository{}).WithMarketDataProviders(r).Handler()
	for _, tc := range []struct {
		err    error
		status int
	}{
		{marketdata.ErrInvalid, 400}, {marketdata.ErrNotFound, 404}, {marketdata.ErrUnsupported, 422},
		{marketdata.ErrLimit, 422}, {marketdata.ErrPayload, 502}, {marketdata.ErrUpstream, 503},
		{context.DeadlineExceeded, 504}, {errors.New("unexpected"), 500},
	} {
		p.err = tc.err
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/providers/example/bars", nil))
		if w.Code != tc.status {
			t.Fatalf("%v: %d %s", tc.err, w.Code, w.Body.String())
		}
	}
	p.err = nil
	before := p.calls
	for _, path := range []string{"/api/providers/example/bars?server=evil", "/api/providers/example/bars?period=1d&period=1m", "/api/providers/example/bars?until=2026-09-04&limit=2"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 400 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	if p.calls != before {
		t.Fatal("unknown parameters reached provider")
	}
	for path, status := range map[string]int{"/api/providers/missing/bars": 404, "/api/providers/example/boards?kind=concept": 422} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != status {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
}

func TestMarketDataTDXUsesExistingInjectionAndLegacyRoute(t *testing.T) {
	p := DefaultTDXProvider()
	calls := 0
	p.FetchHQIndexBars = func(_ context.Context, r tdx.HQBarsRequest, _ tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
		calls++
		return []tdx.HQBar{{Market: r.Market, Symbol: r.Symbol, Category: r.Category, DateTime: "2026-09-03 15:00", Open: 10, High: 12, Low: 9, Close: 11}}, nil
	}
	s := NewServerWithTDXProvider(unusedMarketDataRepository{}, p)
	for _, path := range []string{
		"/api/providers/tdx/bars?kind=index&market=sh&symbol=000001&since=2026-09-03&until=2026-09-04",
		"/api/tdx/hq/bars?index=true&market=sh&symbol=000001&count=1",
	} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestMarketDataTHSHTTPIntegration(t *testing.T) {
	page := `<div class="cate_inner"><div><a href="/gn/detail/code/301558/">阿里巴巴概念</a></div></div><div class="board-hq"><h3>阿里巴巴概念<span>885611</span></h3></div><input id="clid" value="885611">`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/gn/") {
			_, _ = w.Write([]byte(page))
			return
		}
		_, _ = w.Write([]byte(`quotebridge_v4_line_bk_885611_01_2026({"data":"20260904,10,12,9,11,100,1000,,,,0"})`))
	}))
	defer upstream.Close()
	r, _ := marketdata.NewRegistry(ths.NewClient(ths.Options{PageURL: upstream.URL, ChartURL: upstream.URL, RequestInterval: -1}))
	server := httptest.NewServer(NewServer(unusedMarketDataRepository{}).WithMarketDataProviders(r).Handler())
	defer server.Close()
	c := NewHTTPClient(server.URL, nil)
	boards, err := c.ProviderBoards(context.Background(), "ths", "concept")
	if err != nil || len(boards.Boards) != 1 {
		t.Fatalf("%+v %v", boards, err)
	}
	board, err := c.ProviderBoard(context.Background(), "ths", "concept", "301558")
	if err != nil || board.Board.Instrument.Symbol != "885611" {
		t.Fatalf("%+v %v", board, err)
	}
	q := marketdata.BarsQuery{Instrument: *board.Board.Instrument, Period: "1d", Since: "2026-09-04", Until: "2026-09-04"}
	bars, err := c.ProviderBars(context.Background(), "ths", q)
	if err != nil || len(bars.Bars) != 1 || bars.Bars[0].Close != 11 {
		t.Fatalf("%+v %v", bars, err)
	}
}
