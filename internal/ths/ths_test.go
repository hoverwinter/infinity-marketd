package ths

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/marketdata"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func fixedNow() time.Time { return time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC) }

func thsQuery() marketdata.BarsQuery {
	return marketdata.BarsQuery{Instrument: marketdata.Instrument{Kind: "index", Market: "board", Symbol: "881270"}, Period: "1d", Since: "2026-09-03", Until: "2026-09-04"}
}

func envelope(symbol string, year int, rows string) string {
	return fmt.Sprintf("quotebridge_v4_line_bk_%s_01_%d({\"data\":%q})", symbol, year, rows)
}

func TestBoardCatalogAndResolutionGBK(t *testing.T) {
	page := fixture(t, "concept.html")
	gbk, err := simplifiedchinese.GBK.NewEncoder().Bytes(page)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gn/detail/code/301558/" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Cookie") != "v=test" || r.Referer() == "" {
			t.Error("missing configured request headers")
		}
		_, _ = w.Write(gbk)
	}))
	defer server.Close()
	c := NewClient(Options{PageURL: server.URL, Cookie: "v=test", RequestInterval: -1})
	list, err := c.Boards(context.Background(), "concept")
	if err != nil || len(list.Boards) != 2 || list.Scope != "current_page_catalog" {
		t.Fatalf("%+v %v", list, err)
	}
	resolved, err := c.ResolveBoard(context.Background(), "concept", "301558")
	if err != nil || resolved.Board.Instrument.Symbol != "885611" || resolved.Board.Code != "301558" || resolved.Board.Name != "阿里巴巴概念" {
		t.Fatalf("%+v %v", resolved, err)
	}
	if list.Boards[1].Instrument != nil {
		t.Fatal("catalog inferred quotation code")
	}
	for _, broken := range []string{
		strings.ReplaceAll(string(page), "value='885611'", "value='885612'"),
		strings.ReplaceAll(string(page), "<h3>阿里巴巴概念", "<h3>Wrong board"),
		"<html>please complete verification</html>",
		string(page) + "<input id='clid' value='885611'>",
	} {
		if _, err := parseBoard(broken, "concept", "301558"); !errors.Is(err, marketdata.ErrPayload) {
			t.Fatalf("accepted bad identity/page: %v", err)
		}
	}
}

func TestAnnualFixtureAndMalformedPayloads(t *testing.T) {
	raw := fixture(t, "index_881270_2026.js")
	rows, err := parseYear(raw, "881270", 2026)
	if err != nil || len(rows) != 3 || rows[2].Close != 23527.366 || rows[2].Volume != 1981191800 {
		t.Fatalf("%+v %v", rows, err)
	}
	for _, tc := range []struct {
		data string
		want error
	}{
		{envelope("881270", 2026, "20260904,10,12,9,11,100,1000,,,,0"), nil},
		{envelope("881270", 2026, "20260904,10,12,9,11,100,1000,,,,0,"), nil},
		{envelope("881270", 2026, ""), nil},
		{envelope("881270", 2026, "20260904,10,12,9,11,100,1000"), marketdata.ErrPayload},
		{envelope("881270", 2026, "20250904,10,12,9,11,100,1000,,,,0"), marketdata.ErrPayload},
		{envelope("881270", 2026, "20260230,10,12,9,11,100,1000,,,,0"), marketdata.ErrPayload},
		{envelope("881270", 2026, "20260904,10,12,9,11,--,1000,,,,0"), marketdata.ErrPayload},
		{envelope("885611", 2026, ""), marketdata.ErrPayload},
		{"quotebridge_v4_line_bk_881270_01_2026({})", marketdata.ErrPayload},
		{"<html>challenge</html>", marketdata.ErrPayload},
	} {
		_, err := parseYear([]byte(tc.data), "881270", 2026)
		if !errors.Is(err, tc.want) {
			t.Fatalf("%s: got %v want %v", tc.data, err, tc.want)
		}
	}
}

func TestBarsYearsAndFailureAreAtomic(t *testing.T) {
	var paths []string
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/v4/line/bk_881270/01/2025.js":
			fmt.Fprint(w, envelope("881270", 2025, "20251231,10,12,9,11,100,1000,,,,0"))
		case "/v4/line/bk_881270/01/2026.js":
			if fail.Load() {
				w.WriteHeader(401)
				return
			}
			fmt.Fprint(w, envelope("881270", 2026, "20260106,10,12,9,11,200,2000,,,,0;20260105,10,12,9,11,100,1000,,,,0"))
		default:
			t.Errorf("unrequested year: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c := NewClient(Options{ChartURL: server.URL, RequestInterval: -1, Now: fixedNow})
	q := thsQuery()
	q.Since, q.Until = "2025-12-31", "2026-01-05"
	r, err := c.Bars(context.Background(), q)
	if err != nil || len(r.Bars) != 2 || len(paths) != 2 || r.Bars[0].Time != "2025-12-31" || r.Bars[1].Time != "2026-01-05" || r.VolumeUnit != "provider_native" {
		t.Fatalf("%+v %v paths=%v", r, err, paths)
	}
	fail.Store(true)
	r, err = c.Bars(context.Background(), q)
	if !errors.Is(err, marketdata.ErrUpstream) || len(r.Bars) != 0 {
		t.Fatalf("partial success: %+v %v", r, err)
	}
}

func TestBarsRejectsBeforeIOAndValidatesWholeResponse(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		// The invalid row is outside the requested range, but still must fail.
		fmt.Fprint(w, envelope("881270", 2026, "20260105,10,8,9,11,100,1000,,,,0;20260904,10,12,9,11,100,1000,,,,0"))
	}))
	defer server.Close()
	c := NewClient(Options{PageURL: server.URL, ChartURL: server.URL, RequestInterval: -1, Now: fixedNow})
	for _, mutate := range []func(*marketdata.BarsQuery){
		func(q *marketdata.BarsQuery) { q.Period = "1m" },
		func(q *marketdata.BarsQuery) { q.Instrument.Symbol = "301558" },
		func(q *marketdata.BarsQuery) { q.Instrument.Market = "sh" },
		func(q *marketdata.BarsQuery) { q.Until = "2026-02-30" },
		func(q *marketdata.BarsQuery) { q.Adjust = "qfq" },
	} {
		q := thsQuery()
		mutate(&q)
		if _, err := c.Bars(context.Background(), q); err == nil {
			t.Fatal("expected validation error")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid request reached network")
	}
	if _, err := c.Bars(context.Background(), thsQuery()); !errors.Is(err, marketdata.ErrPayload) {
		t.Fatal(err)
	}
}

func TestTransportLimitsCancellationAndConcurrency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/large" {
			fmt.Fprint(w, strings.Repeat("x", maxResponseBytes+1))
			return
		}
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()
	c := NewClient(Options{RequestInterval: 20 * time.Millisecond})
	if _, err := c.get(context.Background(), server.URL, "/large"); !errors.Is(err, marketdata.ErrPayload) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.get(ctx, server.URL, "/"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.get(context.Background(), server.URL, "/"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	// A caller canceled while waiting for the gate must not wait for its owner.
	c.gate <- struct{}{}
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := c.get(ctx, server.URL, "/"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	<-c.gate
}

func TestLiveTHS(t *testing.T) {
	if os.Getenv("MARKETD_THS_PROBE") != "1" {
		t.Skip("set MARKETD_THS_PROBE=1 for live THS read-only verification")
	}
	c := NewClient(Options{Cookie: os.Getenv("INFINITY_THS_COOKIE")})
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	for _, item := range []struct{ kind, code string }{{"industry", "881270"}, {"concept", "301558"}} {
		catalog, err := c.Boards(ctx, item.kind)
		if err != nil {
			t.Fatal(err)
		}
		board, err := c.ResolveBoard(ctx, item.kind, item.code)
		if err != nil {
			t.Fatal(err)
		}
		q := thsQuery()
		q.Instrument = *board.Board.Instrument
		bars, err := c.Bars(ctx, q)
		if err != nil || len(bars.Bars) == 0 {
			t.Fatalf("%+v %v", bars, err)
		}
		t.Logf("kind=%s catalog=%d name=%s page=%s quote=%s bars=%d first=%s last=%s", item.kind, len(catalog.Boards), board.Board.Name, item.code, q.Instrument.Symbol, len(bars.Bars), bars.Bars[0].Time, bars.Bars[len(bars.Bars)-1].Time)
	}
}
