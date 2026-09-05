package querier

import (
	"context"
	"net/http"
	"sort"
)

type LimitMatrixDay struct {
	TradeDate          string                     `json:"trade_date"`
	Summary            *LimitDailySummary         `json:"summary"`
	MarketBreadth      *MarketBreadthDaily        `json:"market_breadth"`
	PerformanceIndices []LimitPerformanceIndexBar `json:"performance_indices"`
	Themes             []LimitThemeDaily          `json:"themes"`
}

type LimitMatrixCell struct {
	TradeDate string            `json:"trade_date"`
	Events    []LimitEvent      `json:"events"`
	Relay     []LimitRelayEvent `json:"relay"`
}

type LimitMatrixRow struct {
	Market string            `json:"market"`
	Symbol string            `json:"symbol"`
	Cells  []LimitMatrixCell `json:"cells"`
}

type LimitReviewMatrix struct {
	Query     LimitQuery       `json:"query"`
	Days      []LimitMatrixDay `json:"days"`
	Rows      []LimitMatrixRow `json:"rows"`
	TotalRows int              `json:"total_rows"`
	HasMore   bool             `json:"has_more"`
	Warnings  []string         `json:"warnings"`
}

func normalizeMatrixQuery(q LimitQuery) (LimitQuery, error) {
	if q.Limit == 0 {
		q.Limit = 100
	}
	if q.Limit > 500 {
		return q, validationError("matrix limit must be <= 500 stocks")
	}
	return NormalizeLimitQuery(q, "matrix")
}

func (s *Server) handleLimitReviewMatrix(w http.ResponseWriter, r *http.Request) {
	q, err := limitQueryFromRequest(r, "matrix")
	if !r.URL.Query().Has("limit") {
		q.Limit = 100
	}
	if err == nil {
		q, err = normalizeMatrixQuery(q)
	}
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	result, err := ReadLimitReviewMatrix(r.Context(), s.repo, q)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Filters select stocks; their cells retain all events and relay samples in the range.
func ReadLimitReviewMatrix(ctx context.Context, repo LimitReviewRepository, q LimitQuery) (LimitReviewMatrix, error) {
	q, err := normalizeMatrixQuery(q)
	result := LimitReviewMatrix{Query: q, Days: []LimitMatrixDay{}, Rows: []LimitMatrixRow{}, Warnings: []string{"missing_cells_are_unknown", "dates_only_include_available_records", "prices_and_charts_use_bars_api"}}
	if err != nil {
		return result, err
	}
	base := LimitQuery{TradeDate: q.TradeDate, Since: q.Since, Until: q.Until}
	eventQuery := base
	eventQuery.Market, eventQuery.Symbol = q.Market, q.Symbol
	events, err := ReadCompleteLimitRows(ctx, eventQuery, repo.LimitEvents)
	if err != nil {
		return result, err
	}
	relay, err := ReadCompleteLimitRows(ctx, eventQuery, repo.LimitRelay)
	if err != nil {
		return result, err
	}
	summaries, err := ReadCompleteLimitRows(ctx, base, repo.LimitSummaries)
	if err != nil {
		return result, err
	}
	breadth, err := ReadCompleteLimitRows(ctx, base, repo.MarketBreadth)
	if err != nil {
		return result, err
	}
	indices, err := ReadCompleteLimitRows(ctx, base, repo.LimitPerformanceIndices)
	if err != nil {
		return result, err
	}
	themes, err := ReadCompleteLimitRows(ctx, base, repo.LimitThemes)
	if err != nil {
		return result, err
	}
	days := map[string]*LimitMatrixDay{}
	day := func(date string) *LimitMatrixDay {
		if days[date] == nil {
			days[date] = &LimitMatrixDay{TradeDate: date, PerformanceIndices: []LimitPerformanceIndexBar{}, Themes: []LimitThemeDaily{}}
		}
		return days[date]
	}
	for i := range summaries {
		row := &summaries[i]
		day(row.TradeDate).Summary = row
	}
	for i := range breadth {
		row := &breadth[i]
		day(row.TradeDate).MarketBreadth = row
	}
	for _, row := range indices {
		d := day(row.TradeDate)
		d.PerformanceIndices = append(d.PerformanceIndices, row)
	}
	for _, row := range themes {
		d := day(row.TradeDate)
		d.Themes = append(d.Themes, row)
	}
	selected := map[string]bool{}
	for _, event := range events {
		day(event.TradeDate)
		matchesTheme := q.Theme == "" || event.ThemePrimary == q.Theme
		for _, tag := range event.ThemeTags {
			matchesTheme = matchesTheme || tag == q.Theme
		}
		if matchesTheme && (q.EventType == "" || event.EventType == q.EventType) {
			selected[event.Market+":"+event.Symbol] = true
		}
	}
	for _, row := range relay {
		day(row.TradeDate)
		if q.EventType == "" && (q.Theme == "" || row.PrevThemePrimary == q.Theme) {
			selected[row.Market+":"+row.Symbol] = true
		}
	}
	keys := make([]string, 0, len(selected))
	for key := range selected {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result.TotalRows = len(keys)
	if q.Offset >= len(keys) {
		keys = nil
	} else {
		keys = keys[q.Offset:]
	}
	result.HasMore = len(keys) > q.Limit
	if result.HasMore {
		keys = keys[:q.Limit]
	}
	cells := map[string]map[string]*LimitMatrixCell{}
	for _, key := range keys {
		cells[key] = map[string]*LimitMatrixCell{}
	}
	cell := func(key, date string) *LimitMatrixCell {
		if cells[key] == nil {
			return nil
		}
		if cells[key][date] == nil {
			cells[key][date] = &LimitMatrixCell{TradeDate: date, Events: []LimitEvent{}, Relay: []LimitRelayEvent{}}
		}
		return cells[key][date]
	}
	for _, event := range events {
		if c := cell(event.Market+":"+event.Symbol, event.TradeDate); c != nil {
			c.Events = append(c.Events, event)
		}
	}
	for _, row := range relay {
		if c := cell(row.Market+":"+row.Symbol, row.TradeDate); c != nil {
			c.Relay = append(c.Relay, row)
		}
	}
	for _, key := range keys {
		row := LimitMatrixRow{Market: key[:2], Symbol: key[3:], Cells: []LimitMatrixCell{}}
		for _, c := range cells[key] {
			row.Cells = append(row.Cells, *c)
		}
		sort.Slice(row.Cells, func(i, j int) bool { return row.Cells[i].TradeDate < row.Cells[j].TradeDate })
		result.Rows = append(result.Rows, row)
	}
	for _, d := range days {
		result.Days = append(result.Days, *d)
		if d.Summary == nil {
			result.Warnings = append(result.Warnings, d.TradeDate+":summary_missing")
		}
		if d.MarketBreadth == nil {
			result.Warnings = append(result.Warnings, d.TradeDate+":market_breadth_missing")
		}
		if len(d.PerformanceIndices) == 0 {
			result.Warnings = append(result.Warnings, d.TradeDate+":performance_indices_missing")
		}
	}
	sort.Slice(result.Days, func(i, j int) bool { return result.Days[i].TradeDate < result.Days[j].TradeDate })
	sort.Strings(result.Warnings)
	return result, nil
}

func (c *HTTPClient) LimitReviewMatrix(ctx context.Context, q LimitQuery) (LimitReviewMatrix, error) {
	var result LimitReviewMatrix
	err := c.getJSON(ctx, "/api/v1/limit-review-matrix", limitQueryValues(q), &result)
	return result, err
}
