package querier

import (
	"context"
	"fmt"
	"net/http"
	"sort"
)

type LimitLadder struct {
	Height uint16       `json:"height"`
	Stocks []LimitEvent `json:"stocks"`
}

type LimitReview struct {
	TradeDate          string                     `json:"trade_date"`
	Summary            *LimitDailySummary         `json:"summary"`
	MarketBreadth      *MarketBreadthDaily        `json:"market_breadth"`
	PerformanceIndices []LimitPerformanceIndexBar `json:"performance_indices"`
	LimitUpPool        []LimitEvent               `json:"limit_up_pool"`
	Broken             []LimitEvent               `json:"broken"`
	LimitDown          []LimitEvent               `json:"limit_down"`
	Ladder             []LimitLadder              `json:"ladder"`
	Relay              []LimitRelayEvent          `json:"relay"`
	ThemeOverview      []LimitThemeDaily          `json:"theme_overview"`
	Warnings           []string                   `json:"warnings"`
}

func (s *Server) handleLimitReview(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("trade_date") == "" {
		writeError(w, http.StatusBadRequest, validationError("trade_date is required"))
		return
	}
	q, err := limitQueryFromRequest(r, "summary")
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	result, err := ReadLimitReview(r.Context(), s.repo, q.TradeDate)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func ReadLimitReview(ctx context.Context, repo LimitReviewRepository, date string) (LimitReview, error) {
	q, err := NormalizeLimitQuery(LimitQuery{TradeDate: date, Limit: 20000}, "summary")
	result := LimitReview{TradeDate: date, LimitUpPool: []LimitEvent{}, Broken: []LimitEvent{}, LimitDown: []LimitEvent{}, Ladder: []LimitLadder{}, Relay: []LimitRelayEvent{}, ThemeOverview: []LimitThemeDaily{}, PerformanceIndices: []LimitPerformanceIndexBar{}, Warnings: []string{}}
	if err != nil {
		return result, err
	}
	events, err := repo.LimitEvents(ctx, q)
	if err != nil {
		return result, err
	}
	if events.HasMore {
		return result, fmt.Errorf("daily events exceed reconstruction limit")
	}
	summaries, err := repo.LimitSummaries(ctx, q)
	if err != nil {
		return result, err
	}
	if len(summaries.Rows) > 0 {
		result.Summary = &summaries.Rows[0]
	} else {
		result.Warnings = append(result.Warnings, "summary_missing")
	}
	breadth, err := repo.MarketBreadth(ctx, q)
	if err != nil {
		return result, err
	}
	if len(breadth.Rows) > 0 {
		result.MarketBreadth = &breadth.Rows[0]
	} else {
		result.Warnings = append(result.Warnings, "market_breadth_missing")
	}
	indices, err := repo.LimitPerformanceIndices(ctx, q)
	if err != nil {
		return result, err
	}
	if indices.HasMore {
		return result, fmt.Errorf("daily indices exceed reconstruction limit")
	}
	if len(indices.Rows) > 0 {
		result.PerformanceIndices = indices.Rows
	} else {
		result.Warnings = append(result.Warnings, "performance_indices_missing")
	}
	relay, err := repo.LimitRelay(ctx, q)
	if err != nil {
		return result, err
	}
	if relay.HasMore {
		return result, fmt.Errorf("daily relay exceeds reconstruction limit")
	}
	if len(relay.Rows) > 0 {
		result.Relay = relay.Rows
	} else {
		result.Warnings = append(result.Warnings, "relay_missing")
	}
	themes, err := repo.LimitThemes(ctx, q)
	if err != nil {
		return result, err
	}
	if themes.HasMore {
		return result, fmt.Errorf("daily themes exceed reconstruction limit")
	}
	if len(themes.Rows) > 0 {
		result.ThemeOverview = themes.Rows
	}
	ladder := map[uint16][]LimitEvent{}
	for _, row := range events.Rows {
		switch row.EventType {
		case "limit_up":
			result.LimitUpPool = append(result.LimitUpPool, row)
			ladder[row.BoardCount] = append(ladder[row.BoardCount], row)
		case "open_limit":
			result.Broken = append(result.Broken, row)
		case "limit_down":
			result.LimitDown = append(result.LimitDown, row)
		}
	}
	for height, stocks := range ladder {
		result.Ladder = append(result.Ladder, LimitLadder{Height: height, Stocks: stocks})
	}
	sort.Slice(result.Ladder, func(i, j int) bool { return result.Ladder[i].Height > result.Ladder[j].Height })
	if len(events.Rows) > 0 {
		if result.Summary != nil {
			s := result.Summary
			if s.LimitUpCount != uint32(len(result.LimitUpPool)) || s.LimitDownCount != uint32(len(result.LimitDown)) || s.OpenLimitCount != uint32(len(result.Broken)) {
				result.Warnings = append(result.Warnings, "summary_event_count_mismatch")
			}
		}
	} else {
		result.Warnings = append(result.Warnings, "events_missing")
	}
	if result.MarketBreadth != nil && result.Summary != nil && ((result.MarketBreadth.LimitUpCount != nil && *result.MarketBreadth.LimitUpCount != result.Summary.LimitUpCount) || (result.MarketBreadth.LimitDownCount != nil && *result.MarketBreadth.LimitDownCount != result.Summary.LimitDownCount)) {
		result.Warnings = append(result.Warnings, "breadth_limit_count_mismatch")
	}
	return result, nil
}
