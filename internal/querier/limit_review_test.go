package querier

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func (r *fakeRepo) LimitEvents(_ context.Context, q LimitQuery) (LimitResult[LimitEvent], error) {
	normalized, err := NormalizeLimitQuery(q, "events")
	return LimitResult[LimitEvent]{Query: normalized, Rows: []LimitEvent{}}, err
}
func (r *fakeRepo) LimitSummaries(_ context.Context, q LimitQuery) (LimitResult[LimitDailySummary], error) {
	normalized, err := NormalizeLimitQuery(q, "summary")
	return LimitResult[LimitDailySummary]{Query: normalized, Rows: []LimitDailySummary{}}, err
}
func (r *fakeRepo) LimitRelay(_ context.Context, q LimitQuery) (LimitResult[LimitRelayEvent], error) {
	normalized, err := NormalizeLimitQuery(q, "relay")
	return LimitResult[LimitRelayEvent]{Query: normalized, Rows: []LimitRelayEvent{}}, err
}
func (r *fakeRepo) LimitThemes(_ context.Context, q LimitQuery) (LimitResult[LimitThemeDaily], error) {
	normalized, err := NormalizeLimitQuery(q, "themes")
	return LimitResult[LimitThemeDaily]{Query: normalized, Rows: []LimitThemeDaily{}}, err
}
func (r *fakeRepo) LimitPerformanceIndices(_ context.Context, q LimitQuery) (LimitResult[LimitPerformanceIndexBar], error) {
	normalized, err := NormalizeLimitQuery(q, "indices")
	return LimitResult[LimitPerformanceIndexBar]{Query: normalized, Rows: []LimitPerformanceIndexBar{}}, err
}
func (r *fakeRepo) MarketBreadth(_ context.Context, q LimitQuery) (LimitResult[MarketBreadthDaily], error) {
	normalized, err := NormalizeLimitQuery(q, "breadth")
	return LimitResult[MarketBreadthDaily]{Query: normalized, Rows: []MarketBreadthDaily{}}, err
}

func TestLimitHTTPValidation(t *testing.T) {
	h := NewServer(&fakeRepo{}).Handler()
	for _, path := range []string{"/limit-events?trade_date=2026-09-04&limit=0", "/limit-events?trade_date=2026-02-30", "/limit-events?since=2026-09-04&until=2026-09-01", "/limit-events?trade_date=2026-09-04&symbol=000001", "/market-breadth?trade_date=2026-09-04&theme=AI", "/limit-relay?trade_date=2026-09-04&sample_group=bad", "/limit-review"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1"+path, nil))
		if w.Code != 400 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
}
func TestLimitHTTPEndpointsAndEmptyJSON(t *testing.T) {
	h := NewServer(&fakeRepo{}).Handler()
	for _, path := range []string{"limit-events", "limit-summary", "limit-relay", "limit-themes", "limit-performance-indices", "market-breadth", "limit-review", "limit-review-matrix"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/"+path+"?trade_date=2026-09-04", nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if path != "limit-review" && !strings.Contains(w.Body.String(), `"rows":[]`) {
			t.Fatal(w.Body.String())
		}
		if path == "limit-review" {
			var v LimitReview
			if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
				t.Fatal(err)
			}
			if v.Summary != nil || v.LimitUpPool == nil || v.PerformanceIndices == nil || len(v.Warnings) == 0 {
				t.Fatalf("%+v", v)
			}
		}
	}
}

func TestMatrixRangeValidationAndFiltering(t *testing.T) {
	h := NewServer(&fakeRepo{}).Handler()
	for _, query := range []string{"since=2016-01-01&until=2026-09-04", "trade_date=2026-09-04&limit=501", "trade_date=2026-09-04&sample_group=prev_ladder"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/limit-review-matrix?"+query, nil))
		if w.Code != 400 {
			t.Fatalf("%s: %d %s", query, w.Code, w.Body.String())
		}
	}
	repo := &reviewFixtureRepo{events: []LimitEvent{
		{TradeDate: "2026-09-03", Market: "sz", Symbol: "000001", EventType: "limit_up", BoardCount: 2, ThemePrimary: "AI", ReasonText: "corrected"},
		{TradeDate: "2026-09-04", Market: "sz", Symbol: "000001", EventType: "limit_down", ThemePrimary: "finance"},
		{TradeDate: "2026-09-04", Market: "sh", Symbol: "600001", EventType: "limit_up", BoardCount: 1, ThemePrimary: "other"},
	}}
	q := LimitQuery{Since: "2026-09-03", Until: "2026-09-04", Theme: "AI", EventType: "limit_up"}
	m, err := ReadLimitReviewMatrix(context.Background(), repo, q)
	if err != nil || len(m.Rows) != 1 || len(m.Days) != 2 || len(m.Rows[0].Cells) != 2 || m.Rows[0].Cells[1].Events[0].EventType != "limit_down" || m.Rows[0].Cells[0].Events[0].ReasonText != "corrected" {
		t.Fatalf("%+v %v", m, err)
	}
	q.Theme, q.EventType, q.Limit = "", "", 1
	m, err = ReadLimitReviewMatrix(context.Background(), repo, q)
	if err != nil || !m.HasMore || m.TotalRows != 2 || m.Rows[0].Symbol != "600001" {
		t.Fatalf("%+v %v", m, err)
	}
	q.Offset = 1
	m, err = ReadLimitReviewMatrix(context.Background(), repo, q)
	if err != nil || m.HasMore || len(m.Rows) != 1 || m.Rows[0].Symbol != "000001" {
		t.Fatalf("%+v %v", m, err)
	}
	repo.more = true
	if _, err := ReadLimitReviewMatrix(context.Background(), repo, q); err == nil {
		t.Fatal("partial matrix accepted")
	}
}

func TestThemesRankResetsEachDate(t *testing.T) {
	rows := AggregateLimitThemes([]LimitEvent{
		{TradeDate: "2026-09-03", ThemePrimary: "AI", EventType: "limit_up"},
		{TradeDate: "2026-09-04", ThemePrimary: "AI", EventType: "limit_up"},
		{TradeDate: "2026-09-04", ThemePrimary: "finance", EventType: "limit_up"},
	})
	if len(rows) != 3 || rows[0].LimitUpCount != 1 || rows[1].StrengthRank != 1 || rows[2].StrengthRank != 2 {
		t.Fatal(rows)
	}
}

func TestReviewCompletePagination(t *testing.T) {
	calls := 0
	rows, err := ReadCompleteLimitRows(context.Background(), LimitQuery{TradeDate: "2026-09-04"}, func(_ context.Context, q LimitQuery) (LimitResult[int], error) {
		calls++
		if calls == 1 {
			if q.Offset != 0 || q.Limit != 20000 {
				t.Fatal(q)
			}
			return LimitResult[int]{Rows: make([]int, 20000), HasMore: true}, nil
		}
		if q.Offset != 20000 {
			t.Fatal(q)
		}
		return LimitResult[int]{Rows: []int{1}}, nil
	})
	if err != nil || len(rows) != 20001 || calls != 2 {
		t.Fatalf("%d %d %v", len(rows), calls, err)
	}
}

type reviewFixtureRepo struct {
	fakeRepo
	events []LimitEvent
	more   bool
}

func TestYesterdayLimitDownRelayUsesTradingDayAndKeepsUnknown(t *testing.T) {
	pct := 2.5
	prev := []LimitEvent{{TradeDate: "2026-09-04", Market: "sh", Symbol: "600001", EventType: "limit_down", ReasonText: "reason"}, {TradeDate: "2026-09-04", Market: "sz", Symbol: "000001", EventType: "limit_down"}}
	q := LimitQuery{TradeDate: "2026-09-07", SampleGroup: "prev_limit_down", Limit: 100}
	result := ReconstructLimitRelay(q, "2026-09-04", prev, nil, nil, []ReviewDailyPerformance{{Market: "sh", Symbol: "600001", PctChg: &pct}})
	if len(result.Rows) != 2 || result.Rows[0].PrevTradeDate != "2026-09-04" || *result.Rows[0].TodayPctChg != 2.5 || result.Rows[1].TodayStatus != "unknown" || result.Rows[1].TodayPctChg != nil {
		t.Fatalf("%+v", result)
	}
	current := []LimitEvent{{Market: "sh", Symbol: "600001", EventType: "open_limit"}, {Market: "sh", Symbol: "600001", EventType: "limit_down"}}
	result = ReconstructLimitRelay(q, "2026-09-04", prev, current, nil, nil)
	if result.Rows[0].TodayStatus != "limit_down" {
		t.Fatal(result.Rows)
	}
}

func TestThemeLeaderRank(t *testing.T) {
	events := []LimitEvent{{TradeDate: "2026-09-04", Market: "sh", Symbol: "600001", EventType: "limit_up", BoardCount: 3, ThemePrimary: "AI"}, {TradeDate: "2026-09-04", Market: "sz", Symbol: "000001", EventType: "limit_up", BoardCount: 1, ThemePrimary: "AI"}, {TradeDate: "2026-09-04", EventType: "open_limit", CloseStatus: "broken_reseal"}}
	themes := AggregateLimitThemes(events)
	if len(themes) != 1 || themes[0].LeaderSymbol != "600001" || themes[0].StrengthRank != 1 {
		t.Fatalf("%+v", themes)
	}
}

func TestRelaySealedDoesNotImplyPromotion(t *testing.T) {
	q := LimitQuery{TradeDate: "2026-09-04", Limit: 100}
	for _, previousType := range []string{"limit_down", "limit_up"} {
		prev := []LimitEvent{{Market: "sz", Symbol: "000001", EventType: previousType, BoardCount: 2}}
		current := []LimitEvent{{Market: "sz", Symbol: "000001", EventType: "limit_up", BoardCount: 2}}
		result := ReconstructLimitRelay(q, "2026-09-03", prev, current, nil, nil)
		if len(result.Rows) == 0 || result.Rows[0].TodayStatus != "sealed" {
			t.Fatalf("%+v", result)
		}
	}
	values := limitQueryValues(LimitQuery{TradeDate: "2026-09-04", Theme: "AI", Offset: 10})
	if values.Get("theme") != "AI" || values.Get("offset") != "10" {
		t.Fatal(values)
	}
}

func TestRelayMissingDerivedReturnKeepsSavedValue(t *testing.T) {
	pct := 1.5
	q := LimitQuery{TradeDate: "2026-09-04", Limit: 100}
	previous := []LimitEvent{{Market: "sz", Symbol: "000001", EventType: "limit_up", BoardCount: 1}}
	stored := []LimitRelayEvent{{Market: "sz", Symbol: "000001", PrevTradeDate: "2026-09-03", SampleGroup: "prev_limit_up", TodayStatus: "broken", TodayPctChg: &pct}}
	result := ReconstructLimitRelay(q, "2026-09-03", previous, nil, stored, []ReviewDailyPerformance{{Market: "sz", Symbol: "000001"}})
	if len(result.Rows) != 1 || result.Rows[0].TodayPctChg == nil || *result.Rows[0].TodayPctChg != pct || result.Rows[0].TodayStatus != "broken" {
		t.Fatalf("%+v", result)
	}
	result = ReconstructLimitRelay(q, "2026-09-03", previous, nil, nil, []ReviewDailyPerformance{{Market: "sz", Symbol: "000001", PctChg: &pct}})
	if result.Rows[0].TodayStatus != "unknown" {
		t.Fatal("daily return alone cannot prove the event pool is complete")
	}
}

func (r *reviewFixtureRepo) LimitEvents(_ context.Context, q LimitQuery) (LimitResult[LimitEvent], error) {
	return LimitResult[LimitEvent]{Rows: r.events, HasMore: r.more}, nil
}
func TestDailyReviewLadderAndTruncation(t *testing.T) {
	repo := &reviewFixtureRepo{events: []LimitEvent{{TradeDate: "2026-09-04", EventType: "limit_up", BoardCount: 1, Symbol: "000001"}, {TradeDate: "2026-09-04", EventType: "limit_up", BoardCount: 3, Symbol: "600001"}, {TradeDate: "2026-09-04", EventType: "limit_down", Symbol: "600002"}}}
	result, err := ReadLimitReview(context.Background(), repo, "2026-09-04")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.LimitUpPool) != 2 || len(result.LimitDown) != 1 || result.Ladder[0].Height != 3 {
		t.Fatalf("%+v", result)
	}
	repo.more = true
	if _, err := ReadLimitReview(context.Background(), repo, "2026-09-04"); err == nil {
		t.Fatal("silently truncated")
	}
}
