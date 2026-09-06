package clickhouse

import (
	"context"
	"fmt"
	"os"
	"reflect"
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
	t.Run("reason keyword", func(t *testing.T) {
		// Filtering must use the replacement's final reason, never the original.
		for keyword, count := range map[string]int{"original": 0, "CORRECTED": 1} {
			result, err := s.LimitEvents(ctx, querier.LimitQuery{TradeDate: "2026-09-04", ReasonKeyword: keyword})
			if err != nil || len(result.Rows) != count {
				t.Fatalf("final reason %q: %+v %v", keyword, result, err)
			}
		}
		fixtures := []model.LimitEvent{
			{Symbol: "000010", ThemePrimary: "液冷", ThemeTags: []string{"液冷"}},
			{Symbol: "000011", ReasonText: "高端装备"},
			{Symbol: "000012", ReasonText: "液冷+AI算力+业绩增长", ThemePrimary: "算力硬件"},
			{Symbol: "000013", ReasonText: "Ai算力+液冷", ThemePrimary: "其他", ThemeTags: []string{"算力硬件"}},
			{Symbol: "000014", ReasonText: "增长50%_+O'Reilly"},
			{Symbol: "000015", ReasonText: "液冷", EventType: "limit_down"},
			{Symbol: "000016", TradeDate: prev, ReasonText: "液冷"},
		}
		for i := range fixtures {
			f := &fixtures[i]
			f.Market = "sz"
			if f.TradeDate.IsZero() {
				f.TradeDate = date
			}
			if f.EventType == "" {
				f.EventType, f.CloseStatus, f.BoardCount = "limit_up", "sealed", 1
			} else {
				f.CloseStatus = "limit_down"
			}
			if f.ThemeTags == nil {
				f.ThemeTags = []string{}
			}
		}
		if err := s.InsertLimitEvents(ctx, fixtures); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			name    string
			query   querier.LimitQuery
			symbols []string
			more    bool
		}{
			{"Chinese and first page", querier.LimitQuery{TradeDate: "2026-09-04", EventType: "limit_up", ReasonKeyword: " 液冷 ", Limit: 1}, []string{"000012"}, true},
			{"second page with theme and market", querier.LimitQuery{TradeDate: "2026-09-04", Market: "sz", Theme: "算力硬件", EventType: "limit_up", ReasonKeyword: "液冷", Limit: 1, Offset: 1}, []string{"000013"}, false},
			{"range and English case", querier.LimitQuery{Since: "2026-09-03", Until: "2026-09-04", ReasonKeyword: "ai"}, []string{"000012", "000013"}, false},
			{"range includes previous day", querier.LimitQuery{Since: "2026-09-03", Until: "2026-09-04", EventType: "limit_up", ReasonKeyword: "液冷"}, []string{"000016", "000012", "000013"}, false},
			{"literal punctuation", querier.LimitQuery{TradeDate: "2026-09-04", ReasonKeyword: "%_+O'Reilly"}, []string{"000014"}, false},
			{"no wildcard interpretation", querier.LimitQuery{TradeDate: "2026-09-04", ReasonKeyword: "%液冷%"}, []string{}, false},
			{"empty reason does not match theme", querier.LimitQuery{TradeDate: "2026-09-04", Market: "sz", Symbol: "000010", ReasonKeyword: "液冷"}, []string{}, false},
			{"blank keyword disables filter", querier.LimitQuery{TradeDate: "2026-09-04", Market: "sz", Symbol: "000010", ReasonKeyword: " \t "}, []string{"000010"}, false},
			{"no matches", querier.LimitQuery{TradeDate: "2026-09-04", ReasonKeyword: "不存在的归因"}, []string{}, false},
			{"offset past matches", querier.LimitQuery{TradeDate: "2026-09-04", EventType: "limit_up", ReasonKeyword: "液冷", Offset: 2}, []string{}, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				result, err := s.LimitEvents(ctx, tc.query)
				if err != nil {
					t.Fatal(err)
				}
				symbols := []string{}
				for _, row := range result.Rows {
					symbols = append(symbols, row.Symbol)
				}
				if !reflect.DeepEqual(symbols, tc.symbols) || result.HasMore != tc.more {
					t.Fatalf("got %+v, want symbols=%v has_more=%v", result, tc.symbols, tc.more)
				}
			})
		}
	})
}
