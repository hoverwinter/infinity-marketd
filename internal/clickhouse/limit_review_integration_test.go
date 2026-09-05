package clickhouse

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/config"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/querier"
)

// Opt-in: creates isolated databases and deliberately never drops user data.
func TestLimitReviewClickHouseIntegration(t *testing.T) {
	path := os.Getenv("MARKETD_REVIEW_INTEGRATION_CONFIG")
	if path == "" {
		t.Skip("set MARKETD_REVIEW_INTEGRATION_CONFIG for isolated real ClickHouse tests")
	}
	cfg, err := config.Load(config.Overrides{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	prefix := fmt.Sprintf("review_test_%d", time.Now().UnixNano())
	cfg.ClickHouse.Databases = config.DatabaseConfig{Market: prefix + "_market", Ops: prefix + "_ops"}
	t.Logf("isolated databases retained: %s, %s", cfg.ClickHouse.Databases.Market, cfg.ClickHouse.Databases.Ops)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	s, err := Open(ctx, cfg.ClickHouse)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	prev := time.Date(2026, 9, 3, 0, 0, 0, 0, loc)
	date := prev.AddDate(0, 0, 1)
	event := model.LimitEvent{TradeDate: date, Market: "sz", Symbol: "000001", EventType: "limit_up", CloseStatus: "sealed", BoardCount: 2, ReasonText: "original", ThemePrimary: "AI", ThemeTags: []string{"AI"}}
	if err := s.InsertLimitEvents(ctx, []model.LimitEvent{event}); err != nil {
		t.Fatal(err)
	}
	event.ReasonText = "corrected"
	if err := s.InsertLimitEvents(ctx, []model.LimitEvent{event}); err != nil {
		t.Fatal(err)
	}
	prior := event
	prior.TradeDate, prior.BoardCount = prev, 1
	if err := s.InsertLimitEvents(ctx, []model.LimitEvent{prior}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertLimitDailySummaries(ctx, []model.LimitDailySummary{{TradeDate: date, PrevTradeDate: &prev, LimitUpCount: 99}}); err != nil {
		t.Fatal(err)
	}
	pct := 10.0
	if err := s.InsertLimitRelayEvents(ctx, []model.LimitRelayEvent{{TradeDate: date, PrevTradeDate: prev, Market: "sz", Symbol: "000001", SampleGroup: "prev_limit_up", TodayStatus: "promoted", TodayPctChg: &pct}}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertLimitThemeDaily(ctx, []model.LimitThemeDaily{{TradeDate: date, ThemeName: "AI", LimitUpCount: 99, StrengthRank: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertLimitPerformanceIndexBars(ctx, []model.LimitPerformanceIndexBar{{TradeDate: date, IndexCode: "prev_limit_up_perf", Open: 100, High: 102, Low: 99, Close: 101}}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertMarketBreadthDaily(ctx, []model.MarketBreadthDaily{{TradeDate: date, UpCount: 3, DownCount: 2, TotalCount: 6}}); err != nil {
		t.Fatal(err)
	}
	q := querier.LimitQuery{TradeDate: "2026-09-04"}
	events, err := s.LimitEvents(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(events.Rows) != 1 || events.Rows[0].ReasonText != "corrected" || events.Rows[0].FirstLimitMinute != nil {
		t.Fatalf("events: %+v", events)
	}
	review, err := querier.ReadLimitReview(ctx, s, q.TradeDate)
	if err != nil {
		t.Fatal(err)
	}
	if review.Summary == nil || review.Summary.LimitUpCount != 1 || len(review.Relay) != 1 || review.Relay[0].TodayPctChg == nil || *review.Relay[0].TodayPctChg != 10 || len(review.ThemeOverview) != 1 || review.ThemeOverview[0].LimitUpCount != 1 || len(review.PerformanceIndices) != 1 || review.MarketBreadth == nil || review.MarketBreadth.UpGT7Count != nil {
		t.Fatalf("review: %+v", review)
	}
	if review.Summary.BigNoodleCount != nil || review.Summary.HighLevelBreakCount != nil || review.Summary.StrongThemeCount != nil {
		t.Fatal("missing summary fields should remain null")
	}
	if review.Summary.SealSuccessRate != nil {
		t.Fatal("missing historical seal coverage must not become a 100% seal rate")
	}
	for _, kind := range []string{"summary", "relay", "themes", "indices", "breadth"} {
		switch kind {
		case "summary":
			_, err = s.LimitSummaries(ctx, querier.LimitQuery{Since: "2026-09-03", Until: "2026-09-04"})
		case "relay":
			_, err = s.LimitRelay(ctx, querier.LimitQuery{Since: "2026-09-03", Until: "2026-09-04"})
		case "themes":
			_, err = s.LimitThemes(ctx, querier.LimitQuery{Since: "2026-09-03", Until: "2026-09-04"})
		case "indices":
			_, err = s.LimitPerformanceIndices(ctx, querier.LimitQuery{Since: "2026-09-03", Until: "2026-09-04"})
		case "breadth":
			_, err = s.MarketBreadth(ctx, querier.LimitQuery{Since: "2026-09-03", Until: "2026-09-04"})
		}
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
	}
	prior.BoardCount, prior.ReasonText, prior.ThemePrimary = 3, "corrected previous reason", "robot"
	if err := s.InsertLimitEvents(ctx, []model.LimitEvent{prior}); err != nil {
		t.Fatal(err)
	}
	rangeQuery := querier.LimitQuery{Since: "2026-09-03", Until: "2026-09-04", SampleGroup: "prev_ladder", Theme: "robot"}
	relay, err := s.LimitRelay(ctx, rangeQuery)
	if err != nil || len(relay.Rows) != 1 || relay.Rows[0].PrevBoardCount != 3 || relay.Rows[0].PrevReasonText != "corrected previous reason" {
		t.Fatalf("range correction: %+v %v", relay, err)
	}
	rangeQuery.SampleGroup, rangeQuery.Theme = "", ""
	themes, err := s.LimitThemes(ctx, rangeQuery)
	if err != nil || len(themes.Rows) != 2 || themes.Rows[0].ThemeName != "robot" || themes.Rows[0].StrengthRank != 1 || themes.Rows[1].StrengthRank != 1 || themes.Rows[1].LimitUpCount != 1 {
		t.Fatalf("range themes: %+v %v", themes, err)
	}
	matrix, err := querier.ReadLimitReviewMatrix(ctx, s, rangeQuery)
	if err != nil || len(matrix.Rows) != 1 || len(matrix.Rows[0].Cells) != 2 || len(matrix.Days) != 2 || matrix.Rows[0].Cells[0].Events[0].ReasonText != "corrected previous reason" {
		t.Fatalf("matrix: %+v %v", matrix, err)
	}
}
