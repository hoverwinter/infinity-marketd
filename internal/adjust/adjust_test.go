package adjust

import (
	"math"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

func TestGenerateFactorsNoEvents(t *testing.T) {
	bars := []model.DailyBar{
		daily("2026-01-02", 10),
		daily("2026-01-03", 11),
	}
	factors, issues := GenerateFactors(bars, nil, time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC))
	if len(issues) != 0 {
		t.Fatalf("issues = %#v", issues)
	}
	if len(factors) != 2 {
		t.Fatalf("factors = %d", len(factors))
	}
	for _, factor := range factors {
		if *factor.QFQFactor != 1 || *factor.HFQFactor != 1 {
			t.Fatalf("factor = %#v", factor)
		}
	}
}

func TestGenerateFactorsOneCategoryOneEvent(t *testing.T) {
	bars := []model.DailyBar{
		daily("2026-01-02", 10),
		daily("2026-01-05", 5),
		daily("2026-01-06", 6),
	}
	zero := 0.0
	ten := 10.0
	event := model.XDXREvent{
		Market:      "sh",
		Symbol:      "600519",
		EventDate:   date("2026-01-05"),
		Category:    1,
		FenHong:     &zero,
		PeiGu:       &zero,
		PeiGuJia:    &zero,
		SongZhuanGu: &ten,
	}
	factors, issues := GenerateFactors(bars, []model.XDXREvent{event}, time.Time{})
	if len(issues) != 0 {
		t.Fatalf("issues = %#v", issues)
	}
	if !near(*factors[0].QFQFactor, 0.5) || !near(*factors[0].HFQFactor, 1) {
		t.Fatalf("first factor = %#v", factors[0])
	}
	if !near(*factors[1].QFQFactor, 1) || !near(*factors[1].HFQFactor, 2) {
		t.Fatalf("event date factor = %#v", factors[1])
	}
	if !near(*factors[2].QFQFactor, 1) || !near(*factors[2].HFQFactor, 2) {
		t.Fatalf("last factor = %#v", factors[2])
	}
}

func TestGenerateFactorsMultipleEvents(t *testing.T) {
	bars := []model.DailyBar{
		daily("2026-01-02", 10),
		daily("2026-01-05", 5),
		daily("2026-01-06", 6),
		daily("2026-01-07", 3),
	}
	zero := 0.0
	ten := 10.0
	events := []model.XDXREvent{
		{Market: "sh", Symbol: "600519", EventDate: date("2026-01-05"), Category: 1, FenHong: &zero, PeiGu: &zero, PeiGuJia: &zero, SongZhuanGu: &ten},
		{Market: "sh", Symbol: "600519", EventDate: date("2026-01-07"), Category: 1, FenHong: &zero, PeiGu: &zero, PeiGuJia: &zero, SongZhuanGu: &ten},
	}
	factors, issues := GenerateFactors(bars, events, time.Time{})
	if len(issues) != 0 {
		t.Fatalf("issues = %#v", issues)
	}
	if !near(*factors[0].QFQFactor, 0.25) || !near(*factors[3].HFQFactor, 4) {
		t.Fatalf("factors = %#v", factors)
	}
}

func TestGenerateFactorsFutureEventIgnored(t *testing.T) {
	bars := []model.DailyBar{
		daily("2026-01-02", 10),
		daily("2026-01-05", 11),
	}
	zero := 0.0
	ten := 10.0
	events := []model.XDXREvent{
		{Market: "sh", Symbol: "600519", EventDate: date("2026-01-06"), Category: 1, FenHong: &zero, PeiGu: &zero, PeiGuJia: &zero, SongZhuanGu: &ten},
	}
	factors, issues := GenerateFactors(bars, events, time.Time{})
	if len(issues) != 1 || issues[0].Type != "future_xdxr_event" {
		t.Fatalf("issues = %#v", issues)
	}
	if !near(*factors[0].QFQFactor, 1) || !near(*factors[1].QFQFactor, 1) {
		t.Fatalf("qfq factors = %#v", factors)
	}
	if !near(*factors[0].HFQFactor, 1) || !near(*factors[1].HFQFactor, 1) {
		t.Fatalf("hfq factors = %#v", factors)
	}
}

func TestGenerateFactorsMissingPreviousCloseProducesNilFactors(t *testing.T) {
	zero := 0.0
	ten := 10.0
	factors, issues := GenerateFactors(
		[]model.DailyBar{daily("2026-01-05", 5)},
		[]model.XDXREvent{{Market: "sh", Symbol: "600519", EventDate: date("2026-01-05"), Category: 1, FenHong: &zero, PeiGu: &zero, PeiGuJia: &zero, SongZhuanGu: &ten}},
		time.Time{},
	)
	if len(issues) == 0 || issues[0].Type != "missing_previous_close" {
		t.Fatalf("issues = %#v", issues)
	}
	if factors[0].QFQFactor != nil || factors[0].HFQFactor != nil {
		t.Fatalf("factor = %#v", factors[0])
	}
}

func TestGenerateFactorsUnsupportedCategoryIsIgnored(t *testing.T) {
	factors, issues := GenerateFactors(
		[]model.DailyBar{daily("2026-01-02", 10), daily("2026-01-03", 11)},
		[]model.XDXREvent{{Market: "sh", Symbol: "600519", EventDate: date("2026-01-03"), Category: 99}},
		time.Time{},
	)
	if len(issues) != 1 || issues[0].Type != "unsupported_xdxr_category" {
		t.Fatalf("issues = %#v", issues)
	}
	if *factors[0].QFQFactor != 1 || *factors[1].HFQFactor != 1 {
		t.Fatalf("factors = %#v", factors)
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

func near(a, b float64) bool {
	return math.Abs(a-b) < 0.000001
}
