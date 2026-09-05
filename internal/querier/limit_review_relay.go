package querier

import "sort"

type ReviewDailyPerformance struct {
	TradeDate string   `ch:"trade_date"`
	Market    string   `ch:"market"`
	Symbol    string   `ch:"symbol"`
	PctChg    *float64 `ch:"pct_chg"`
}

func ReconstructLimitRelay(q LimitQuery, prev string, previous, current []LimitEvent, stored []LimitRelayEvent, performance []ReviewDailyPerformance) LimitResult[LimitRelayEvent] {
	identity := func(market, symbol string) string { return market + ":" + symbol }
	today := map[string]LimitEvent{}
	priority := map[string]int{"limit_down": 3, "limit_up": 2, "open_limit": 1}
	for _, r := range current {
		key := identity(r.Market, r.Symbol)
		if prior, ok := today[key]; !ok || priority[r.EventType] > priority[prior.EventType] {
			today[key] = r
		}
	}
	returns := map[string]*float64{}
	for _, r := range performance {
		returns[identity(r.Market, r.Symbol)] = r.PctChg
	}
	old := map[string]LimitRelayEvent{}
	for _, r := range stored {
		if r.PrevTradeDate == prev {
			old[r.SampleGroup+"|"+identity(r.Market, r.Symbol)] = r
		}
	}
	result := LimitResult[LimitRelayEvent]{Query: q, Rows: []LimitRelayEvent{}}
	for _, r := range previous {
		if (q.Market != "" && q.Market != r.Market) || (q.Symbol != "" && q.Symbol != r.Symbol) || (q.Theme != "" && q.Theme != r.ThemePrimary) {
			continue
		}
		groups := []string{}
		switch r.EventType {
		case "limit_up":
			groups = append(groups, "prev_limit_up")
			if r.BoardCount >= 2 {
				groups = append(groups, "prev_ladder")
			}
		case "open_limit":
			if r.CloseStatus != "broken_reseal" {
				groups = append(groups, "prev_broken")
			}
		case "limit_down":
			groups = append(groups, "prev_limit_down")
		}
		key := identity(r.Market, r.Symbol)
		for _, group := range groups {
			if q.SampleGroup != "" && q.SampleGroup != group {
				continue
			}
			item := LimitRelayEvent{TradeDate: q.TradeDate, PrevTradeDate: prev, Market: r.Market, Symbol: r.Symbol, SampleGroup: group, PrevBoardCount: r.BoardCount, PrevReasonText: r.ReasonText, PrevThemePrimary: r.ThemePrimary, PrevFirstLimitMinute: r.FirstLimitMinute, TodayStatus: "unknown"}
			if prior, ok := old[group+"|"+key]; ok {
				item.TodayStatus = prior.TodayStatus
				item.TodayPctChg = prior.TodayPctChg
				item.TodayBoardCount = prior.TodayBoardCount
			}
			if pct, ok := returns[key]; ok && pct != nil {
				item.TodayPctChg = pct
			}
			if event, ok := today[key]; ok {
				item.TodayBoardCount = event.BoardCount
				switch event.EventType {
				case "limit_up":
					item.TodayStatus = "sealed"
					if r.EventType == "limit_up" && event.BoardCount > r.BoardCount {
						item.TodayStatus = "promoted"
					}
				case "limit_down":
					item.TodayStatus = "limit_down"
				case "open_limit":
					item.TodayStatus = "open_limit"
				}
			}
			result.Rows = append(result.Rows, item)
		}
	}
	sort.Slice(result.Rows, func(i, j int) bool {
		a, b := result.Rows[i], result.Rows[j]
		if a.SampleGroup != b.SampleGroup {
			return a.SampleGroup < b.SampleGroup
		}
		if a.Market != b.Market {
			return a.Market < b.Market
		}
		return a.Symbol < b.Symbol
	})
	if q.Offset >= len(result.Rows) {
		result.Rows = []LimitRelayEvent{}
		return result
	}
	result.Rows = result.Rows[q.Offset:]
	if len(result.Rows) > q.Limit {
		result.HasMore = true
		result.Rows = result.Rows[:q.Limit]
	}
	return result
}
