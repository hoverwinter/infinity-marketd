package derived

import (
	"math"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

func TestGenerateDailyFirstRowNull(t *testing.T) {
	rows := GenerateDaily([]model.DailyBar{
		daily("2026-06-01", 10),
		daily("2026-06-02", 11),
	}, DailyRange{}, time.Time{})
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[0].PrevClose != nil || rows[0].PctChg != nil {
		t.Fatalf("first row = %#v", rows[0])
	}
	if !near(*rows[1].PrevClose, 10) || !near(*rows[1].PctChg, 10) {
		t.Fatalf("second row = %#v", rows[1])
	}
}

func TestGenerateDailyUsesPreviousValidCloseAcrossGaps(t *testing.T) {
	rows := GenerateDaily([]model.DailyBar{
		daily("2026-06-01", 10),
		daily("2026-06-04", 12),
	}, DailyRange{}, time.Time{})
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	if !near(*rows[1].PrevClose, 10) || !near(*rows[1].PctChg, 20) {
		t.Fatalf("gap row = %#v", rows[1])
	}
}

func TestGenerateDailySkipsNonPositiveCloseAsPreviousClose(t *testing.T) {
	rows := GenerateDaily([]model.DailyBar{
		daily("2026-06-01", 10),
		daily("2026-06-02", 0),
		daily("2026-06-03", 12),
	}, DailyRange{}, time.Time{})
	if len(rows) != 3 {
		t.Fatalf("rows=%d", len(rows))
	}
	if !near(*rows[1].PrevClose, 10) || !near(*rows[1].PctChg, -100) {
		t.Fatalf("zero-close row = %#v", rows[1])
	}
	if !near(*rows[2].PrevClose, 10) || !near(*rows[2].PctChg, 20) {
		t.Fatalf("after zero-close row = %#v", rows[2])
	}
}

func TestGenerateDailyRangeUsesLookback(t *testing.T) {
	since := date("2026-06-02")
	until := date("2026-06-02")
	rows := GenerateDaily([]model.DailyBar{
		daily("2026-06-01", 10),
		daily("2026-06-02", 11),
		daily("2026-06-03", 12),
	}, DailyRange{Since: &since, Until: &until}, time.Time{})
	if len(rows) != 1 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[0].TradeDate.Format("2006-01-02") != "2026-06-02" || !near(*rows[0].PrevClose, 10) || !near(*rows[0].PctChg, 10) {
		t.Fatalf("range row = %#v", rows[0])
	}
}

func TestGenerateDailyCorrectionRecomputesAffectedRows(t *testing.T) {
	original := GenerateDaily([]model.DailyBar{
		daily("2026-06-01", 10),
		daily("2026-06-02", 11),
		daily("2026-06-03", 12),
	}, DailyRange{}, time.Time{})
	corrected := GenerateDaily([]model.DailyBar{
		daily("2026-06-01", 10),
		daily("2026-06-02", 20),
		daily("2026-06-03", 12),
	}, DailyRange{}, time.Time{})
	if near(*original[2].PrevClose, *corrected[2].PrevClose) || near(*original[2].PctChg, *corrected[2].PctChg) {
		t.Fatalf("original=%#v corrected=%#v", original[2], corrected[2])
	}
	if !near(*corrected[2].PrevClose, 20) || !near(*corrected[2].PctChg, -40) {
		t.Fatalf("corrected row = %#v", corrected[2])
	}
}

func daily(day string, close float64) model.DailyBar {
	return model.DailyBar{Market: "sh", Symbol: "600519", TradeDate: date(day), Close: close}
}

func date(day string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", day, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}

func near(a float64, b float64) bool {
	return math.Abs(a-b) < 0.000001
}
