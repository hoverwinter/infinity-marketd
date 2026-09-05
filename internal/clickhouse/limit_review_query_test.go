package clickhouse

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hoverwinter/infinity-marketd/internal/querier"
)

type reviewQueryConn struct {
	fakeConn
	fill func(any)
}

func (c *reviewQueryConn) Select(_ context.Context, dest any, query string, args ...any) error {
	c.queries = append(c.queries, query)
	c.queryArgs = append(c.queryArgs, args)
	if c.fill != nil {
		c.fill(dest)
	}
	return nil
}
func TestLimitQueriesBoundedParameterizedAndFinal(t *testing.T) {
	q := querier.LimitQuery{Since: "2016-01-01", Until: "2026-09-04", Limit: 17, Offset: 20}
	for _, kind := range []string{"events", "summary", "relay", "themes", "indices", "breadth"} {
		stmt, args, err := limitReviewSQL("infinity_market", kind, q)
		if err != nil {
			t.Fatal(err)
		}
		for _, part := range []string{" FINAL WHERE ", "trade_date >= toDate(?)", "trade_date <= toDate(?)", "LIMIT 18 OFFSET 20"} {
			if !strings.Contains(stmt, part) {
				t.Fatalf("%s: %s", kind, stmt)
			}
		}
		if !reflect.DeepEqual(args, []any{q.Since, q.Until}) {
			t.Fatalf("%+v", args)
		}
	}
	q = querier.LimitQuery{TradeDate: "2026-09-04", Theme: "'; DROP TABLE x; --", Limit: 10}
	stmt, args, err := limitReviewSQL("infinity_market", "events", q)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stmt, q.Theme) || !strings.Contains(stmt, "has(theme_tags, ?)") || args[1] != q.Theme {
		t.Fatalf("%s %+v", stmt, args)
	}
	if _, _, err := limitReviewSQL("bad;db", "events", q); err == nil {
		t.Fatal("invalid identifier accepted")
	}
}
func TestLimitPaginationAndValidationBeforeSQL(t *testing.T) {
	conn := &reviewQueryConn{fill: func(dest any) {
		if rows, ok := dest.(*[]querier.LimitEvent); ok {
			*rows = []querier.LimitEvent{{Symbol: "000001"}, {Symbol: "000002"}}
		}
	}}
	s := &Store{conn: conn, marketDB: "market", opsDB: "ops"}
	result, err := s.LimitEvents(context.Background(), querier.LimitQuery{TradeDate: "2026-09-04", Limit: 1})
	if err != nil || !result.HasMore || len(result.Rows) != 1 {
		t.Fatalf("%+v %v", result, err)
	}
	before := len(conn.queries)
	if _, err := s.LimitEvents(context.Background(), querier.LimitQuery{TradeDate: "2026-02-30"}); err == nil {
		t.Fatal("invalid date accepted")
	}
	if len(conn.queries) != before {
		t.Fatal("invalid query reached ClickHouse")
	}
}

func TestSummaryCountsRefreshUsesScalarDateBindings(t *testing.T) {
	prev := "2026-09-03"
	promotion := 0.25
	calls := 0
	conn := &reviewQueryConn{fill: func(dest any) {
		rows := dest.(*[]querier.LimitDailySummary)
		calls++
		if calls == 1 {
			*rows = []querier.LimitDailySummary{{TradeDate: "2026-09-04", PrevTradeDate: &prev, LimitUpCount: 99, PrevLimitUpPromotionRate: &promotion}}
		} else {
			*rows = []querier.LimitDailySummary{{TradeDate: "2026-09-04", LimitUpCount: 2, MaxBoardHeight: 3}}
		}
	}}
	s := &Store{conn: conn, marketDB: "market", opsDB: "ops"}
	result, err := s.LimitSummaries(context.Background(), querier.LimitQuery{TradeDate: "2026-09-04"})
	if err != nil || calls != 2 || len(result.Rows) != 1 || result.Rows[0].LimitUpCount != 2 || *result.Rows[0].PrevTradeDate != prev || *result.Rows[0].PrevLimitUpPromotionRate != promotion {
		t.Fatalf("%+v %v", result, err)
	}
	if !strings.Contains(conn.queries[1], "r.trade_date IN (toDate(?))") || !reflect.DeepEqual(conn.queryArgs[1], []any{"2026-09-04"}) {
		t.Fatalf("%s %+v", conn.queries[1], conn.queryArgs[1])
	}
}
