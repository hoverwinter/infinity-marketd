package clickhouse

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

func TestInsertLimitReviewTables(t *testing.T) {
	ctx := context.Background()
	date := time.Date(2026, 9, 4, 0, 0, 0, 0, time.Local)
	prev := date.AddDate(0, 0, -1)
	minute := "09:30"
	f := 1.2
	u := uint16(1)
	volume := uint64(100)

	tests := []struct {
		name      string
		insert    func(*Store) error
		wantTable string
		wantCols  string
	}{
		{
			name: "events",
			insert: func(store *Store) error {
				return store.InsertLimitEvents(ctx, []model.LimitEvent{{TradeDate: date, Market: "sh", Symbol: "600519", EventType: "limit_up", CloseStatus: "sealed", BoardCount: 1, ThemeTags: []string{"消费"}, FirstLimitMinute: &minute, OpenCount: &u, Amount: &f}})
			},
			wantTable: "a_share_limit_events",
			wantCols:  "(trade_date, market, symbol, event_type, close_status, board_count",
		},
		{
			name: "summary",
			insert: func(store *Store) error {
				return store.InsertLimitDailySummaries(ctx, []model.LimitDailySummary{{TradeDate: date, PrevTradeDate: &prev, LimitUpCount: 1}})
			},
			wantTable: "a_share_limit_daily_summary",
			wantCols:  "(trade_date, prev_trade_date, limit_up_count",
		},
		{
			name: "relay",
			insert: func(store *Store) error {
				return store.InsertLimitRelayEvents(ctx, []model.LimitRelayEvent{{TradeDate: date, PrevTradeDate: prev, Market: "sh", Symbol: "600519", SampleGroup: "prev_limit_up", TodayStatus: "promoted", TodayPctChg: &f}})
			},
			wantTable: "a_share_limit_relay_events",
			wantCols:  "(trade_date, prev_trade_date, market, symbol, sample_group",
		},
		{
			name: "performance indices",
			insert: func(store *Store) error {
				return store.InsertLimitPerformanceIndexBars(ctx, []model.LimitPerformanceIndexBar{{IndexCode: "prev_limit_up_perf", TradeDate: date, Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: &volume}})
			},
			wantTable: "a_share_limit_performance_index_bars_1d",
			wantCols:  "(index_code, trade_date, open, high, low, close, volume, amount)",
		},
		{
			name: "breadth",
			insert: func(store *Store) error {
				return store.InsertMarketBreadthDaily(ctx, []model.MarketBreadthDaily{{TradeDate: date, UpCount: 3000, DownCount: 2000, TotalCount: 5000}})
			},
			wantTable: "a_share_market_breadth_daily",
			wantCols:  "(trade_date, up_count, down_count, flat_count",
		},
		{
			name: "themes",
			insert: func(store *Store) error {
				return store.InsertLimitThemeDaily(ctx, []model.LimitThemeDaily{{TradeDate: date, ThemeName: "AI", LimitUpCount: 3, LeaderMarket: "sh", LeaderSymbol: "600519"}})
			},
			wantTable: "a_share_limit_theme_daily",
			wantCols:  "(trade_date, theme_name, limit_up_count, ladder_count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &fakeConn{}
			store := &Store{conn: conn, marketDB: "infinity_market", opsDB: "infinity_ops"}
			if err := tt.insert(store); err != nil {
				t.Fatal(err)
			}
			if len(conn.queries) != 1 || !strings.Contains(conn.queries[0], "INSERT INTO `infinity_market`.`"+tt.wantTable+"`") || !strings.Contains(conn.queries[0], tt.wantCols) {
				t.Fatalf("unexpected insert query: %#v", conn.queries)
			}
			if len(conn.batches) != 1 || !conn.batches[0].sent || len(conn.batches[0].rows) != 1 {
				t.Fatalf("batch not sent: %#v", conn.batches)
			}
		})
	}
}

func TestInsertLimitReviewTablesIgnoreEmptyRows(t *testing.T) {
	conn := &fakeConn{}
	store := &Store{conn: conn, marketDB: "infinity_market", opsDB: "infinity_ops"}
	ctx := context.Background()
	if err := store.InsertLimitEvents(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertLimitDailySummaries(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertLimitRelayEvents(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertLimitPerformanceIndexBars(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertMarketBreadthDaily(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertLimitThemeDaily(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if len(conn.queries) != 0 {
		t.Fatalf("queries=%d", len(conn.queries))
	}
}
