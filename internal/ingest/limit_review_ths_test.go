package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestTHSPoolReadsAllPages(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		rows := []map[string]string{}
		start, end := (page-1)*200, page*200
		if end > 201 {
			end = 201
		}
		for i := start; i < end; i++ {
			rows = append(rows, map[string]string{"code": fmt.Sprintf("%06d", i+1), "high_days": "首板"})
		}
		json.NewEncoder(w).Encode(map[string]any{"status_code": 0, "data": map[string]any{"date": "20260904", "page": map[string]int{"limit": 200, "page": page, "count": 2, "total": 201}, "info": rows}})
	}))
	defer srv.Close()
	c := &thsReviewClient{client: srv.Client(), base: srv.URL, pools: map[string][]thsPoolStock{}}
	rows, err := c.pool(context.Background(), "limit_up_pool", "20260904")
	if err != nil || len(rows) != 201 || calls != 2 {
		t.Fatalf("len=%d calls=%d %v", len(rows), calls, err)
	}
}

func TestTHSRefreshPoolsAndConsecutiveSuffix(t *testing.T) {
	for _, dry := range []bool{true, false} {
		t.Run(fmt.Sprint(dry), func(t *testing.T) {
			requests := map[string]int{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("User-Agent") == "" {
					t.Error("missing user agent")
				}
				date := r.URL.Query().Get("date")
				requests[r.URL.Path+date]++
				var data any
				if r.URL.Path == "/trade_day" {
					if r.URL.Query().Get("next") != "1" {
						t.Error("THS rejects next=0")
					}
					data = map[string]any{"code": 0, "trade_day": true, "prev_dates": []string{"20260903"}}
				} else {
					rows := []map[string]any{}
					if r.URL.Path == "/limit_up_pool" && date == "20260904" {
						rows = []map[string]any{{"code": "000001", "high_days": "11天7板", "reason_type": "reason", "first_limit_up_time": "1788485400"}, {"code": "600001", "high_days": "3天3板"}}
					}
					if r.URL.Path == "/limit_up_pool" && date == "20260903" {
						rows = []map[string]any{{"code": "000001", "high_days": "首板"}}
					}
					data = map[string]any{"date": date, "page": map[string]int{"page": 1, "limit": 200, "total": len(rows), "count": 1}, "info": rows}
				}
				json.NewEncoder(w).Encode(map[string]any{"status_code": 0, "data": data})
			}))
			defer srv.Close()
			store := &reviewMemoryStore{}
			out, err := RefreshTHSLimitReview(context.Background(), THSReviewOptions{LoadEvents: emptyLimitEvents, Date: "2026-09-04", DryRun: dry, Store: store, BaseURL: srv.URL, RequestInterval: -1, Now: func() time.Time { return time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC) }})
			if err != nil || out.Events != 2 || out.RowsWritten != 3 || out.DailySummaries != 1 {
				t.Fatalf("%+v %v", out, err)
			}
			if requests["/limit_up_pool20260903"] != 1 {
				t.Fatal(requests)
			}
			if dry {
				if len(store.events) != 0 || len(store.runs) != 0 {
					t.Fatal("dry run wrote data")
				}
			} else {
				if len(store.events) != 2 || store.events[0].BoardCount != 2 || store.events[1].BoardCount != 3 || *store.events[0].FirstLimitMinute != "09:30" || store.runs[0].RowsWritten != 3 {
					t.Fatalf("%+v %+v", store.events, store.runs)
				}
				if store.summaries[0].BigNoodleCount != nil || store.summaries[0].StrongThemeCount != nil {
					t.Fatal("unavailable statistics must remain null")
				}
			}
		})
	}
}

func TestTHSPaginationRejectsIncompleteWrongDateAndHTTPFailure(t *testing.T) {
	for _, kind := range []string{"wrong-date", "incomplete", "duplicate", "forbidden"} {
		t.Run(kind, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if kind == "forbidden" {
					w.WriteHeader(403)
					return
				}
				date := "20260904"
				if kind == "wrong-date" {
					date = "20260903"
				}
				rows := []map[string]string{{"code": "000001", "high_days": "首板"}}
				total := 1
				if kind == "incomplete" {
					total = 2
				}
				if kind == "duplicate" {
					rows = append(rows, rows[0])
					total = 2
				}
				json.NewEncoder(w).Encode(map[string]any{"status_code": 0, "data": map[string]any{"date": date, "page": map[string]int{"limit": 200, "total": total, "count": 1, "page": 1}, "info": rows}})
			}))
			defer srv.Close()
			c := &thsReviewClient{client: srv.Client(), base: srv.URL, pools: map[string][]thsPoolStock{}}
			if _, err := c.pool(context.Background(), "limit_up_pool", "20260904"); err == nil {
				t.Fatal("invalid pool accepted")
			}
		})
	}
}

func TestTHSMinuteAndBoardLabels(t *testing.T) {
	for _, label := range []string{"", "7板", "2天3板", "0天0板"} {
		if _, _, err := thsKnownStreak(label); err == nil {
			t.Fatalf("accepted %q", label)
		}
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	date := time.Date(2026, 9, 4, 0, 0, 0, 0, loc)
	bad := "1788399000"
	if _, err := thsMinute(&bad, date); err == nil {
		t.Fatal("previous-day timestamp accepted")
	}
}

func TestTHSOverlappingPools(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(fmt.Sprint(conflict), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var data any
				if r.URL.Path == "/trade_day" {
					data = map[string]any{"code": 0, "trade_day": true, "prev_dates": []string{"20260903"}}
				} else {
					rows := []map[string]string{}
					if r.URL.Path != "/lower_limit_pool" || conflict {
						rows = append(rows, map[string]string{"code": "000001", "high_days": "首板"})
					}
					data = map[string]any{"date": "20260904", "page": map[string]int{"page": 1, "limit": 200, "total": len(rows), "count": 1}, "info": rows}
				}
				json.NewEncoder(w).Encode(map[string]any{"status_code": 0, "data": data})
			}))
			defer srv.Close()
			store := &reviewMemoryStore{}
			_, err := RefreshTHSLimitReview(context.Background(), THSReviewOptions{LoadEvents: emptyLimitEvents, Date: "2026-09-04", Store: store, BaseURL: srv.URL, RequestInterval: -1, Now: func() time.Time { return time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC) }})
			if conflict {
				if err == nil || len(store.events) != 0 || len(store.runs) != 1 || store.runs[0].Status != "failed" {
					t.Fatalf("err=%v store=%+v", err, store)
				}
			} else if err != nil || len(store.events) != 2 || store.events[1].CloseStatus != "broken_reseal" || store.summaries[0].SealSuccessRate == nil || *store.summaries[0].SealSuccessRate != 1 {
				t.Fatalf("err=%v store=%+v", err, store)
			}
		})
	}
}
