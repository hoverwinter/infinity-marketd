package derived

import (
	"sort"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

type DailyRange struct {
	Since *time.Time
	Until *time.Time
}

func GenerateDaily(bars []model.DailyBar, r DailyRange, computedAt time.Time) []model.DailyDerived {
	if len(bars) == 0 {
		return nil
	}
	bars = append([]model.DailyBar(nil), bars...)
	sort.Slice(bars, func(i, j int) bool { return bars[i].TradeDate.Before(bars[j].TradeDate) })
	out := make([]model.DailyDerived, 0, len(bars))
	var previousClose *float64
	for _, bar := range bars {
		var prevClose *float64
		var pctChg *float64
		if previousClose != nil && *previousClose > 0 {
			prev := *previousClose
			pct := (bar.Close - prev) / prev * 100
			prevClose = &prev
			pctChg = &pct
		}
		if inRange(bar.TradeDate, r) {
			out = append(out, model.DailyDerived{
				Market:     bar.Market,
				Symbol:     bar.Symbol,
				TradeDate:  bar.TradeDate,
				PrevClose:  prevClose,
				PctChg:     pctChg,
				ComputedAt: computedAt,
			})
		}
		if bar.Close > 0 {
			closeValue := bar.Close
			previousClose = &closeValue
		}
	}
	return out
}

func inRange(day time.Time, r DailyRange) bool {
	if r.Since != nil && day.Before(*r.Since) {
		return false
	}
	if r.Until != nil && day.After(*r.Until) {
		return false
	}
	return true
}
