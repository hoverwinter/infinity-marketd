package querier

import "sort"

func AggregateLimitThemes(events []LimitEvent) []LimitThemeDaily {
	byName := map[string]LimitThemeDaily{}
	for _, e := range events {
		if e.ThemePrimary == "" {
			continue
		}
		key := e.TradeDate + "|" + e.ThemePrimary
		r := byName[key]
		r.TradeDate = e.TradeDate
		r.ThemeName = e.ThemePrimary
		switch e.EventType {
		case "limit_up":
			r.LimitUpCount++
			if e.BoardCount >= 2 {
				r.LadderCount++
			}
			if r.LeaderSymbol == "" || e.BoardCount > r.LeaderBoardCount || (e.BoardCount == r.LeaderBoardCount && e.Symbol < r.LeaderSymbol) {
				r.LeaderMarket = e.Market
				r.LeaderSymbol = e.Symbol
				r.LeaderBoardCount = e.BoardCount
			}
		case "limit_down":
			r.LimitDownCount++
		case "open_limit":
			r.BrokenCount++
		}
		byName[key] = r
	}
	rows := make([]LimitThemeDaily, 0, len(byName))
	for _, r := range byName {
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.TradeDate != b.TradeDate {
			return a.TradeDate < b.TradeDate
		}
		if a.LimitUpCount != b.LimitUpCount {
			return a.LimitUpCount > b.LimitUpCount
		}
		if a.LadderCount != b.LadderCount {
			return a.LadderCount > b.LadderCount
		}
		return a.ThemeName < b.ThemeName
	})
	var rank uint16
	for i := range rows {
		if i == 0 || rows[i].TradeDate != rows[i-1].TradeDate {
			rank = 0
		}
		rank++
		rows[i].StrengthRank = rank
	}
	return rows
}
