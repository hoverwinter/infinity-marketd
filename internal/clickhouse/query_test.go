package clickhouse

import (
	"strings"
	"testing"

	"github.com/hoverwinter/infinity-marketd/internal/querier"
)

func TestDailyUntilIsInclusive(t *testing.T) {
	where, args, err := barWhere(
		querier.BarQuery{Market: "sh", Symbol: "600519", Since: "2026-06-01", Until: "2026-06-05"},
		"trade_date",
		parseDateBound,
		parseDateUntilBound,
	)
	if err != nil {
		t.Fatal(err)
	}
	if where != "market = ? AND symbol = ? AND trade_date >= ? AND trade_date <= ?" {
		t.Fatalf("where=%s", where)
	}
	if got := args[3].(interface{ Format(string) string }).Format("2006-01-02"); got != "2026-06-05" {
		t.Fatalf("until=%s", got)
	}
}

func TestMinuteDateUntilIncludesWholeDay(t *testing.T) {
	where, args, err := barWhere(
		querier.BarQuery{Market: "sh", Symbol: "600519", Since: "2026-06-01", Until: "2026-06-05"},
		"bar_time",
		parseDateTimeBound,
		parseDateTimeUntilBound,
	)
	if err != nil {
		t.Fatal(err)
	}
	if where != "market = ? AND symbol = ? AND bar_time >= ? AND bar_time < ?" {
		t.Fatalf("where=%s", where)
	}
	if got := args[3].(interface{ Format(string) string }).Format("2006-01-02 15:04:05"); got != "2026-06-06 00:00:00" {
		t.Fatalf("until=%s", got)
	}
}

func TestMinuteTimestampUntilIsInclusive(t *testing.T) {
	where, args, err := barWhere(
		querier.BarQuery{Market: "sh", Symbol: "600519", Until: "2026-06-05 15:00:00"},
		"bar_time",
		parseDateTimeBound,
		parseDateTimeUntilBound,
	)
	if err != nil {
		t.Fatal(err)
	}
	if where != "market = ? AND symbol = ? AND bar_time <= ?" {
		t.Fatalf("where=%s", where)
	}
	if got := args[2].(interface{ Format(string) string }).Format("2006-01-02 15:04:05"); got != "2026-06-05 15:00:00" {
		t.Fatalf("until=%s", got)
	}
}

func TestBarsSQLWithoutBoundsReturnsRecentLimitAscending(t *testing.T) {
	stmt := barsSQL("db.table", "market, symbol, trade_date, close", "market = ? AND symbol = ?", "trade_date", 120, false)
	for _, want := range []string{
		"FROM (SELECT market, symbol, trade_date, close FROM db.table",
		"ORDER BY trade_date DESC LIMIT 120",
		") ORDER BY trade_date ASC",
	} {
		if !strings.Contains(stmt, want) {
			t.Fatalf("stmt=%s missing %s", stmt, want)
		}
	}
}

func TestBarsSQLWithBoundsUsesAscendingLimit(t *testing.T) {
	stmt := barsSQL("db.table", "market, symbol, trade_date, close", "market = ? AND symbol = ? AND trade_date >= ?", "trade_date", 120, true)
	if stmt != "SELECT market, symbol, trade_date, close FROM db.table WHERE market = ? AND symbol = ? AND trade_date >= ? ORDER BY trade_date ASC LIMIT 120" {
		t.Fatalf("stmt=%s", stmt)
	}
}
