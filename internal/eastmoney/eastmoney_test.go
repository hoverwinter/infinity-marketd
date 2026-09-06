package eastmoney

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/marketdata"
)

func fixedNow() time.Time { return time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC) }
func query() marketdata.BarsQuery {
	return marketdata.BarsQuery{Instrument: marketdata.Instrument{Kind: "index", Market: "board", Symbol: "BK1027"}, Period: "1d", Since: "2026-09-03", Until: "2026-09-04"}
}
func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func testClient(server *httptest.Server) *Client {
	return NewClient(Options{QuoteURL: server.URL, HistoryURL: server.URL, RequestInterval: -1, Now: fixedNow})
}
func catalogResponse(total int, items []map[string]any) string {
	b, _ := json.Marshal(map[string]any{"rc": 0, "data": map[string]any{"total": total, "diff": items}})
	return string(b)
}
func catalogRow(code string) map[string]any {
	return map[string]any{"f12": code, "f13": 90, "f14": "板块" + code}
}

func TestCatalogUsesEffectivePageSizeAndChecksCategory(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		v := r.URL.Query()
		if r.URL.Path != "/api/qt/clist/get" || v.Get("pz") != "100" || v.Get("fid") != "f12" || v.Get("po") != "0" || v.Get("np") != "1" || v.Get("fields") != "f12,f13,f14" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		if strings.Contains(v.Get("fs"), "t:3") {
			fmt.Fprint(w, catalogResponse(1, []map[string]any{catalogRow("BK0715")}))
			return
		}
		if v.Get("fs") != "m:90 t:2 f:!50" {
			t.Errorf("filter=%s", v.Get("fs"))
		}
		page, _ := strconv.Atoi(v.Get("pn"))
		rows := []map[string]any{}
		for i := (page - 1) * 20; i < min(page*20, 45); i++ {
			rows = append(rows, catalogRow(fmt.Sprintf("BK%04d", 1000+i)))
		}
		fmt.Fprint(w, catalogResponse(45, rows))
	}))
	defer server.Close()
	c := testClient(server)
	rows, err := c.Boards(context.Background(), "industry")
	if err != nil || calls.Load() != 3 || len(rows.Boards) != 45 || rows.Scope != "current_provider_catalog" {
		t.Fatalf("calls=%d rows=%d err=%v", calls.Load(), len(rows.Boards), err)
	}
	resolved, err := c.ResolveBoard(context.Background(), "industry", "BK1044")
	if err != nil || resolved.Board.Instrument == nil || resolved.Board.Instrument.Symbol != "BK1044" || calls.Load() != 6 {
		t.Fatalf("%+v %v", resolved, err)
	}
	if _, err := c.ResolveBoard(context.Background(), "concept", "BK1044"); !errors.Is(err, marketdata.ErrNotFound) {
		t.Fatalf("wrong category accepted: %v", err)
	}
}

func TestCatalogFailureNeverReturnsPartialRows(t *testing.T) {
	for _, tc := range []struct {
		name, second string
		status       int
		want         error
	}{
		{"changed-total", catalogResponse(4, []map[string]any{catalogRow("BK1002")}), 200, marketdata.ErrPayload},
		{"empty-page", catalogResponse(3, []map[string]any{}), 200, marketdata.ErrPayload},
		{"duplicate", catalogResponse(3, []map[string]any{catalogRow("BK1001")}), 200, marketdata.ErrPayload},
		{"wrong-market", catalogResponse(3, []map[string]any{{"f12": "BK1002", "f13": 1, "f14": "wrong"}}), 200, marketdata.ErrPayload},
		{"missing-market", catalogResponse(3, []map[string]any{{"f12": "BK1002", "f14": "wrong"}}), 200, marketdata.ErrPayload},
		{"http-failure", "", 503, marketdata.ErrUpstream},
		{"null-data", `{"rc":0,"data":null}`, 200, marketdata.ErrUpstream},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("pn") == "1" {
					fmt.Fprint(w, catalogResponse(3, []map[string]any{catalogRow("BK1000"), catalogRow("BK1001")}))
					return
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.second)
			}))
			defer server.Close()
			result, err := testClient(server).Boards(context.Background(), "industry")
			if !errors.Is(err, tc.want) || len(result.Boards) != 0 {
				t.Fatalf("partial result %+v err=%v", result, err)
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, catalogResponse(101, []map[string]any{catalogRow("BK1000"), catalogRow("BK1001")}))
	}))
	defer server.Close()
	if _, err := testClient(server).Boards(context.Background(), "industry"); !errors.Is(err, marketdata.ErrLimit) {
		t.Fatal(err)
	}
}

func TestHistoryFieldOrderAndFailures(t *testing.T) {
	good := fixture(t, "history.json")
	for _, tc := range []struct {
		name, data string
		want       error
	}{
		{"valid", good, nil},
		{"wrong-code", strings.ReplaceAll(good, "BK1027", "BK0715"), marketdata.ErrPayload},
		{"wrong-market", strings.ReplaceAll(good, `"market":90`, `"market":1`), marketdata.ErrPayload},
		{"missing-klines", `{"rc":0,"data":{"code":"BK1027","market":90}}`, marketdata.ErrPayload},
		{"empty", `{"rc":0,"data":{"code":"BK1027","market":90,"klines":[]}}`, nil},
		{"null-klines", `{"rc":0,"data":{"code":"BK1027","market":90,"klines":null}}`, marketdata.ErrPayload},
		{"null-data", `{"rc":0,"data":null}`, marketdata.ErrUpstream},
		{"rc-failure", `{"rc":102,"data":null}`, marketdata.ErrUpstream},
		{"missing-rc", `{"data":null}`, marketdata.ErrPayload},
		{"html", `<html>challenge</html>`, marketdata.ErrPayload},
		{"outside-window", strings.ReplaceAll(good, "2026-09-03", "2026-09-02"), marketdata.ErrPayload},
		{"invalid-OHLC", strings.ReplaceAll(good, "10,11,12,9", "10,11,8,9"), marketdata.ErrPayload},
		{"not-finite", strings.ReplaceAll(good, "100,1000", "NaN,1000"), marketdata.ErrPayload},
		{"bad-field", strings.ReplaceAll(good, "100,1000", "--,1000"), marketdata.ErrPayload},
		{"conflicting-date", strings.ReplaceAll(good, "2026-09-03", "2026-09-04"), marketdata.ErrPayload},
		{"extra-field", strings.ReplaceAll(good, "100,1000", "100,1000,5"), marketdata.ErrPayload},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				v := r.URL.Query()
				if v.Get("secid") != "90.BK1027" || v.Get("klt") != "101" || v.Get("fqt") != "0" || v.Get("lmt") != "1000" || v.Get("beg") != "20260903" || v.Get("end") != "20260904" {
					t.Errorf("request=%s", r.URL)
				}
				fmt.Fprint(w, tc.data)
			}))
			defer server.Close()
			r, err := testClient(server).Bars(context.Background(), query())
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			if err != nil && len(r.Bars) != 0 {
				t.Fatal("partial history")
			}
			if tc.name == "valid" {
				if len(r.Bars) != 2 || r.Bars[1].Open != 10 || r.Bars[1].Close != 11 || r.Bars[1].High != 12 || r.Bars[1].Low != 9 || r.Bars[0].Time != "2026-09-03" || r.VolumeUnit != "provider_native" {
					t.Fatalf("wrong normalized result %+v", r)
				}
			}
		})
	}
}

func TestHistoryDateChunkingAndAtomicFailure(t *testing.T) {
	var calls atomic.Int32
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		v := r.URL.Query()
		date := ""
		switch v.Get("beg") {
		case "20251231":
			if v.Get("end") != "20251231" {
				t.Error("year boundary not clipped")
			}
			date = "2025-12-31"
		case "20260101":
			if v.Get("end") != "20260105" {
				t.Error("upper bound not clipped")
			}
			date = "2026-01-05"
			if fail.Load() {
				w.WriteHeader(503)
				return
			}
		default:
			t.Errorf("unexpected interval %s", r.URL)
		}
		fmt.Fprintf(w, `{"rc":0,"data":{"code":"BK1027","market":90,"klines":["%s,10,11,12,9,100,1000,0,0,0,0"]}}`, date)
	}))
	defer server.Close()
	c := testClient(server)
	q := query()
	q.Since, q.Until = "2025-12-31", "2026-01-05"
	r, err := c.Bars(context.Background(), q)
	if err != nil || len(r.Bars) != 2 || calls.Load() != 2 {
		t.Fatalf("%+v %v", r, err)
	}
	fail.Store(true)
	r, err = c.Bars(context.Background(), q)
	if !errors.Is(err, marketdata.ErrUpstream) || len(r.Bars) != 0 {
		t.Fatalf("partial %+v %v", r, err)
	}
}

func TestValidationAndTransportBounds(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, strings.Repeat("x", maxResponseBytes+1))
	}))
	defer server.Close()
	c := testClient(server)
	for _, mutate := range []func(*marketdata.BarsQuery){
		func(q *marketdata.BarsQuery) { q.Period = "1m" }, func(q *marketdata.BarsQuery) { q.Instrument.Symbol = "881270" },
		func(q *marketdata.BarsQuery) { q.Instrument.Market = "sh" }, func(q *marketdata.BarsQuery) { q.Adjust = "qfq" },
		func(q *marketdata.BarsQuery) { q.Until = "2026-02-30" },
	} {
		q := query()
		mutate(&q)
		if _, err := c.Bars(context.Background(), q); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
	if _, err := c.Boards(context.Background(), "bad"); !errors.Is(err, marketdata.ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := c.ResolveBoard(context.Background(), "industry", "../BK1027"); !errors.Is(err, marketdata.ErrInvalid) {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("invalid request reached upstream")
	}
	if _, err := c.Bars(context.Background(), query()); !errors.Is(err, marketdata.ErrPayload) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Bars(ctx, query()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	c.gate <- struct{}{}
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := c.Bars(ctx, query()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	<-c.gate
}

func TestConcurrentCatalogCalls(t *testing.T) {
	body := fixture(t, "industry.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
	defer server.Close()
	c := NewClient(Options{QuoteURL: server.URL, RequestInterval: 10 * time.Millisecond})
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Boards(context.Background(), "industry"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}

func TestLiveEastmoney(t *testing.T) {
	if os.Getenv("MARKETD_EASTMONEY_PROBE") != "1" {
		t.Skip("set MARKETD_EASTMONEY_PROBE=1 for live read-only verification")
	}
	c := NewClient(Options{QuoteURL: os.Getenv("MARKETD_EASTMONEY_QUOTE_URL"), HistoryURL: os.Getenv("MARKETD_EASTMONEY_HISTORY_URL")})
	for _, item := range []struct{ kind, code string }{{"industry", "BK1027"}, {"concept", "BK0715"}} {
		t.Run(item.kind, func(t *testing.T) {
			boards, err := c.Boards(context.Background(), item.kind)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, b := range boards.Boards {
				if b.Code == item.code {
					found = true
				}
			}
			if !found {
				t.Fatalf("%s not in %s catalog", item.code, item.kind)
			}
			q := query()
			q.Instrument.Symbol = item.code
			bars, err := c.Bars(context.Background(), q)
			if err != nil || len(bars.Bars) == 0 {
				t.Fatalf("rows=%d err=%v", len(bars.Bars), err)
			}
			t.Logf("kind=%s catalog=%d symbol=%s bars=%d first=%s last=%s", item.kind, len(boards.Boards), item.code, len(bars.Bars), bars.Bars[0].Time, bars.Bars[len(bars.Bars)-1].Time)
		})
	}
}
