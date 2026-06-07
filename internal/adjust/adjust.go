package adjust

import (
	"fmt"
	"sort"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

type Issue struct {
	Type      string
	Market    string
	Symbol    string
	TradeDate time.Time
	Message   string
}

func NormalizeXDXR(rows []tdx.HQXDXRInfo) ([]model.XDXREvent, []Issue) {
	events := make([]model.XDXREvent, 0, len(rows))
	var issues []Issue
	loc, _ := time.LoadLocation("Asia/Shanghai")
	for _, row := range rows {
		eventDate, err := time.ParseInLocation("2006-01-02", row.Date, loc)
		if err != nil {
			issues = append(issues, Issue{Type: "invalid_xdxr_date", Market: row.Market, Symbol: row.Symbol, Message: fmt.Sprintf("invalid xdxr date %q", row.Date)})
			continue
		}
		events = append(events, model.XDXREvent{
			Market:         row.Market,
			Symbol:         row.Symbol,
			EventDate:      eventDate,
			Category:       uint8(row.Category),
			CategoryName:   row.Name,
			FenHong:        row.FenHong,
			PeiGuJia:       row.PeiGuJia,
			SongZhuanGu:    row.SongZhuanGu,
			PeiGu:          row.PeiGu,
			SuoGu:          row.SuoGu,
			PanQianLiuTong: row.PanQianLiuTong,
			PanHouLiuTong:  row.PanHouLiuTong,
			QianZongGuBen:  row.QianZongGuBen,
			HouZongGuBen:   row.HouZongGuBen,
			FenShu:         row.FenShu,
			XingQuanJia:    row.XingQuanJia,
		})
	}
	return events, issues
}

func GenerateFactors(bars []model.DailyBar, events []model.XDXREvent, computedAt time.Time) ([]model.AdjustFactor, []Issue) {
	if len(bars) == 0 {
		return nil, []Issue{{Type: "zero_daily_bars", Message: "no raw daily bars for factor refresh"}}
	}
	bars = append([]model.DailyBar(nil), bars...)
	sort.Slice(bars, func(i, j int) bool { return bars[i].TradeDate.Before(bars[j].TradeDate) })
	events = append([]model.XDXREvent(nil), events...)
	sort.Slice(events, func(i, j int) bool {
		if events[i].EventDate.Equal(events[j].EventDate) {
			return events[i].Category < events[j].Category
		}
		return events[i].EventDate.Before(events[j].EventDate)
	})

	ratios, issues := eventRatios(bars, events)
	if hasBlockingIssue(issues) {
		return nilFactors(bars, computedAt), issues
	}
	factors := make([]model.AdjustFactor, 0, len(bars))
	for _, bar := range bars {
		qfq := 1.0
		hfq := 1.0
		for _, ratio := range ratios {
			if bar.TradeDate.Before(ratio.EventDate) {
				qfq *= ratio.Value
			}
			if !bar.TradeDate.Before(ratio.EventDate) {
				hfq *= 1 / ratio.Value
			}
		}
		factors = append(factors, model.AdjustFactor{
			Market:     bar.Market,
			Symbol:     bar.Symbol,
			TradeDate:  bar.TradeDate,
			QFQFactor:  floatPtr(qfq),
			HFQFactor:  floatPtr(hfq),
			ComputedAt: computedAt,
		})
	}
	return factors, issues
}

type eventRatio struct {
	EventDate time.Time
	Value     float64
}

func eventRatios(bars []model.DailyBar, events []model.XDXREvent) ([]eventRatio, []Issue) {
	var ratios []eventRatio
	var issues []Issue
	for _, event := range events {
		if event.Category != 1 {
			issues = append(issues, Issue{Type: "unsupported_xdxr_category", Market: event.Market, Symbol: event.Symbol, TradeDate: event.EventDate, Message: fmt.Sprintf("unsupported xdxr category %d ignored by factor refresh", event.Category)})
			continue
		}
		prevClose, ok := previousClose(bars, event.EventDate)
		if !ok || prevClose <= 0 {
			issues = append(issues, Issue{Type: "missing_previous_close", Market: event.Market, Symbol: event.Symbol, TradeDate: event.EventDate, Message: "missing positive previous raw close for xdxr event"})
			continue
		}
		if event.FenHong == nil || event.PeiGu == nil || event.PeiGuJia == nil || event.SongZhuanGu == nil {
			issues = append(issues, Issue{Type: "missing_xdxr_fields", Market: event.Market, Symbol: event.Symbol, TradeDate: event.EventDate, Message: "category 1 xdxr event missing required fields"})
			continue
		}
		denominator := 10 + *event.PeiGu + *event.SongZhuanGu
		if denominator <= 0 {
			issues = append(issues, Issue{Type: "invalid_xdxr_denominator", Market: event.Market, Symbol: event.Symbol, TradeDate: event.EventDate, Message: "non-positive xdxr adjustment denominator"})
			continue
		}
		theoreticalPreClose := (prevClose*10 - *event.FenHong + (*event.PeiGu)*(*event.PeiGuJia)) / denominator
		ratio := theoreticalPreClose / prevClose
		if ratio <= 0 {
			issues = append(issues, Issue{Type: "invalid_adjust_ratio", Market: event.Market, Symbol: event.Symbol, TradeDate: event.EventDate, Message: "non-positive adjustment ratio"})
			continue
		}
		ratios = append(ratios, eventRatio{EventDate: event.EventDate, Value: ratio})
	}
	return ratios, issues
}

func previousClose(bars []model.DailyBar, eventDate time.Time) (float64, bool) {
	for i := len(bars) - 1; i >= 0; i-- {
		if bars[i].TradeDate.Before(eventDate) && bars[i].Close > 0 {
			return bars[i].Close, true
		}
	}
	return 0, false
}

func hasBlockingIssue(issues []Issue) bool {
	for _, issue := range issues {
		switch issue.Type {
		case "missing_previous_close", "missing_xdxr_fields", "invalid_xdxr_denominator", "invalid_adjust_ratio":
			return true
		}
	}
	return false
}

func nilFactors(bars []model.DailyBar, computedAt time.Time) []model.AdjustFactor {
	factors := make([]model.AdjustFactor, 0, len(bars))
	for _, bar := range bars {
		factors = append(factors, model.AdjustFactor{Market: bar.Market, Symbol: bar.Symbol, TradeDate: bar.TradeDate, ComputedAt: computedAt})
	}
	return factors
}

func floatPtr(v float64) *float64 {
	return &v
}
