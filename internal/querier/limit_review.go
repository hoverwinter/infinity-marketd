package querier

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

// LoadLimitEventFacts provides bounded current-row reads for write-plane guards.
// CLI callers pass HTTPClient; in-process console callers pass their repository.
func LoadLimitEventFacts(ctx context.Context, repo interface {
	LimitEvents(context.Context, LimitQuery) (LimitResult[LimitEvent], error)
}, day string) ([]model.LimitEvent, error) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	date, err := time.ParseInLocation("2006-01-02", day, loc)
	if err != nil {
		return nil, err
	}
	var out []model.LimitEvent
	seen := map[string]bool{}
	for offset := 0; offset < 20000; offset += 2000 {
		result, err := repo.LimitEvents(ctx, LimitQuery{TradeDate: day, Limit: 2000, Offset: offset})
		if err != nil {
			return nil, err
		}
		if len(result.Rows) > 2000 || (result.HasMore && len(result.Rows) != 2000) {
			return nil, fmt.Errorf("incomplete current-event page")
		}
		for _, r := range result.Rows {
			key := r.Market + ":" + r.Symbol + ":" + r.EventType
			if r.TradeDate != day || seen[key] {
				return nil, fmt.Errorf("invalid or repeated current-event page")
			}
			seen[key] = true
			out = append(out, model.LimitEvent{TradeDate: date, Market: r.Market, Symbol: r.Symbol, EventType: r.EventType, CloseStatus: r.CloseStatus, BoardCount: r.BoardCount, ReasonText: r.ReasonText, ThemePrimary: r.ThemePrimary, ThemeTags: r.ThemeTags, FirstLimitMinute: r.FirstLimitMinute, LastLimitMinute: r.LastLimitMinute, OpenCount: r.OpenCount, SealOrderAmount: r.SealOrderAmount, Amount: r.Amount, TurnoverRate: r.TurnoverRate, MarketValue: r.MarketValue})
		}
		if !result.HasMore {
			return out, nil
		}
	}
	return nil, fmt.Errorf("current-event read exceeded 20000 rows")
}

type LimitEvent struct {
	TradeDate        string   `json:"trade_date" ch:"trade_date"`
	Market           string   `json:"market" ch:"market"`
	Symbol           string   `json:"symbol" ch:"symbol"`
	EventType        string   `json:"event_type" ch:"event_type"`
	CloseStatus      string   `json:"close_status" ch:"close_status"`
	BoardCount       uint16   `json:"board_count" ch:"board_count"`
	ReasonText       string   `json:"reason_text" ch:"reason_text"`
	ThemePrimary     string   `json:"theme_primary" ch:"theme_primary"`
	ThemeTags        []string `json:"theme_tags" ch:"theme_tags"`
	FirstLimitMinute *string  `json:"first_limit_minute" ch:"first_limit_minute"`
	LastLimitMinute  *string  `json:"last_limit_minute" ch:"last_limit_minute"`
	OpenCount        *uint16  `json:"open_count" ch:"open_count"`
	SealOrderAmount  *float64 `json:"seal_order_amount" ch:"seal_order_amount"`
	Amount           *float64 `json:"amount" ch:"amount"`
	TurnoverRate     *float64 `json:"turnover_rate" ch:"turnover_rate"`
	MarketValue      *float64 `json:"market_value" ch:"market_value"`
}

type LimitDailySummary struct {
	TradeDate                string   `json:"trade_date" ch:"trade_date"`
	PrevTradeDate            *string  `json:"prev_trade_date" ch:"prev_trade_date"`
	LimitUpCount             uint32   `json:"limit_up_count" ch:"limit_up_count"`
	LimitDownCount           uint32   `json:"limit_down_count" ch:"limit_down_count"`
	OpenLimitCount           uint32   `json:"open_limit_count" ch:"open_limit_count"`
	SealSuccessRate          *float64 `json:"seal_success_rate" ch:"seal_success_rate"`
	MaxBoardHeight           uint16   `json:"max_board_height" ch:"max_board_height"`
	FirstBoardCount          uint32   `json:"first_board_count" ch:"first_board_count"`
	ContinuousBoardCount     uint32   `json:"continuous_board_count" ch:"continuous_board_count"`
	PrevLimitUpPromotionRate *float64 `json:"prev_limit_up_promotion_rate" ch:"prev_limit_up_promotion_rate"`
	PrevLadderPromotionRate  *float64 `json:"prev_ladder_promotion_rate" ch:"prev_ladder_promotion_rate"`
	BigNoodleCount           *uint32  `json:"big_noodle_count" ch:"big_noodle_count"`
	HighLevelBreakCount      *uint32  `json:"high_level_break_count" ch:"high_level_break_count"`
	StrongThemeCount         *uint32  `json:"strong_theme_count" ch:"strong_theme_count"`
	TwoBoardCount            uint32   `json:"two_board_count" ch:"two_board_count"`
	ThreeBoardCount          uint32   `json:"three_board_count" ch:"three_board_count"`
	FourBoardCount           uint32   `json:"four_board_count" ch:"four_board_count"`
	FivePlusBoardCount       uint32   `json:"five_plus_board_count" ch:"five_plus_board_count"`
}

type LimitRelayEvent struct {
	TradeDate            string   `json:"trade_date" ch:"trade_date"`
	PrevTradeDate        string   `json:"prev_trade_date" ch:"prev_trade_date"`
	Market               string   `json:"market" ch:"market"`
	Symbol               string   `json:"symbol" ch:"symbol"`
	SampleGroup          string   `json:"sample_group" ch:"sample_group"`
	PrevBoardCount       uint16   `json:"prev_board_count" ch:"prev_board_count"`
	PrevReasonText       string   `json:"prev_reason_text" ch:"prev_reason_text"`
	PrevThemePrimary     string   `json:"prev_theme_primary" ch:"prev_theme_primary"`
	PrevFirstLimitMinute *string  `json:"prev_first_limit_minute" ch:"prev_first_limit_minute"`
	TodayStatus          string   `json:"today_status" ch:"today_status"`
	TodayBoardCount      uint16   `json:"today_board_count" ch:"today_board_count"`
	TodayPctChg          *float64 `json:"today_pct_chg" ch:"today_pct_chg"`
}

type LimitPerformanceIndexBar struct {
	IndexCode string   `json:"index_code" ch:"index_code"`
	TradeDate string   `json:"trade_date" ch:"trade_date"`
	Open      float64  `json:"open" ch:"open"`
	High      float64  `json:"high" ch:"high"`
	Low       float64  `json:"low" ch:"low"`
	Close     float64  `json:"close" ch:"close"`
	Volume    *uint64  `json:"volume" ch:"volume"`
	Amount    *float64 `json:"amount" ch:"amount"`
}

type MarketBreadthDaily struct {
	TradeDate                 string  `json:"trade_date" ch:"trade_date"`
	UpCount                   uint32  `json:"up_count" ch:"up_count"`
	DownCount                 uint32  `json:"down_count" ch:"down_count"`
	FlatCount                 *uint32 `json:"flat_count" ch:"flat_count"`
	UnchangedOrSuspendedCount *uint32 `json:"unchanged_or_suspended_count" ch:"unchanged_or_suspended_count"`
	UpGT3Count                *uint32 `json:"up_gt_3_count" ch:"up_gt_3_count"`
	UpGT5Count                *uint32 `json:"up_gt_5_count" ch:"up_gt_5_count"`
	UpGT7Count                *uint32 `json:"up_gt_7_count" ch:"up_gt_7_count"`
	DownGT3Count              *uint32 `json:"down_gt_3_count" ch:"down_gt_3_count"`
	DownGT5Count              *uint32 `json:"down_gt_5_count" ch:"down_gt_5_count"`
	DownGT7Count              *uint32 `json:"down_gt_7_count" ch:"down_gt_7_count"`
	LimitUpCount              *uint32 `json:"limit_up_count" ch:"limit_up_count"`
	LimitDownCount            *uint32 `json:"limit_down_count" ch:"limit_down_count"`
	TotalCount                uint32  `json:"total_count" ch:"total_count"`
}

type LimitThemeDaily struct {
	TradeDate        string `json:"trade_date" ch:"trade_date"`
	ThemeName        string `json:"theme_name" ch:"theme_name"`
	LimitUpCount     uint32 `json:"limit_up_count" ch:"limit_up_count"`
	LadderCount      uint32 `json:"ladder_count" ch:"ladder_count"`
	BrokenCount      uint32 `json:"broken_count" ch:"broken_count"`
	LimitDownCount   uint32 `json:"limit_down_count" ch:"limit_down_count"`
	LeaderMarket     string `json:"leader_market" ch:"leader_market"`
	LeaderSymbol     string `json:"leader_symbol" ch:"leader_symbol"`
	LeaderBoardCount uint16 `json:"leader_board_count" ch:"leader_board_count"`
	StrengthRank     uint16 `json:"strength_rank" ch:"strength_rank"`
}

type LimitQuery struct {
	TradeDate     string `json:"trade_date,omitempty"`
	PrevTradeDate string `json:"prev_trade_date,omitempty"`
	Since         string `json:"since,omitempty"`
	Until         string `json:"until,omitempty"`
	Market        string `json:"market,omitempty"`
	Symbol        string `json:"symbol,omitempty"`
	EventType     string `json:"event_type,omitempty"`
	SampleGroup   string `json:"sample_group,omitempty"`
	IndexCode     string `json:"index_code,omitempty"`
	Theme         string `json:"theme,omitempty"`
	Limit         int    `json:"limit"`
	Offset        int    `json:"offset"`
}

type LimitResult[T any] struct {
	Query   LimitQuery `json:"query"`
	Rows    []T        `json:"rows"`
	HasMore bool       `json:"has_more"`
}

type LimitReviewRepository interface {
	LimitEvents(context.Context, LimitQuery) (LimitResult[LimitEvent], error)
	LimitSummaries(context.Context, LimitQuery) (LimitResult[LimitDailySummary], error)
	LimitRelay(context.Context, LimitQuery) (LimitResult[LimitRelayEvent], error)
	LimitThemes(context.Context, LimitQuery) (LimitResult[LimitThemeDaily], error)
	LimitPerformanceIndices(context.Context, LimitQuery) (LimitResult[LimitPerformanceIndexBar], error)
	MarketBreadth(context.Context, LimitQuery) (LimitResult[MarketBreadthDaily], error)
}

func NormalizeLimitQuery(q LimitQuery, kind string) (LimitQuery, error) {
	q.Market = strings.ToLower(strings.TrimSpace(q.Market))
	q.Symbol = strings.TrimSpace(q.Symbol)
	q.Theme = strings.TrimSpace(q.Theme)
	for _, p := range []*string{&q.TradeDate, &q.PrevTradeDate, &q.Since, &q.Until} {
		*p = strings.TrimSpace(*p)
		if *p != "" {
			if _, err := time.Parse("2006-01-02", *p); err != nil {
				return q, validationError("invalid date %q, expected YYYY-MM-DD", *p)
			}
		}
	}
	if q.TradeDate != "" && (q.Since != "" || q.Until != "") {
		return q, validationError("trade_date cannot be combined with since/until")
	}
	if q.TradeDate == "" && (q.Since == "" || q.Until == "") {
		return q, validationError("trade_date or both since and until are required")
	}
	if q.Since > q.Until {
		return q, validationError("since must be <= until")
	}
	if (kind == "relay" || kind == "themes" || kind == "matrix") && q.TradeDate == "" {
		since, _ := time.Parse("2006-01-02", q.Since)
		until, _ := time.Parse("2006-01-02", q.Until)
		if until.Sub(since) >= 93*24*time.Hour {
			return q, validationError("reconstructed queries support at most 93 calendar days")
		}
	}
	if q.PrevTradeDate != "" && (kind != "relay" || q.TradeDate == "" || q.PrevTradeDate >= q.TradeDate) {
		return q, validationError("prev_trade_date requires a later trade_date on relay queries")
	}
	if q.Limit == 0 {
		q.Limit = 1000
	}
	if q.Limit < 1 || q.Limit > 20000 {
		return q, validationError("limit must be between 1 and 20000")
	}
	if q.Offset < 0 || q.Offset > 1000000 {
		return q, validationError("offset must be between 0 and 1000000")
	}
	if q.Market != "" && !marketPattern.MatchString(q.Market) {
		return q, validationError("market must be sh, sz, or bj")
	}
	if q.Symbol != "" && (!symbolPattern.MatchString(q.Symbol) || q.Market == "") {
		return q, validationError("symbol requires six digits and an explicit market")
	}
	if !limitEnum(q.EventType, "", "limit_up", "open_limit", "limit_down") {
		return q, validationError("invalid event_type")
	}
	if !limitEnum(q.SampleGroup, "", "prev_limit_up", "prev_ladder", "prev_broken", "prev_limit_down") {
		return q, validationError("invalid sample_group")
	}
	if !limitEnum(q.IndexCode, "", "prev_limit_up_perf", "prev_non_st_limit_up_perf", "prev_ladder_perf", "prev_limit_down_perf") {
		return q, validationError("invalid index_code")
	}
	if q.EventType != "" && kind != "events" && kind != "matrix" {
		return q, validationError("event_type only applies to events")
	}
	if q.SampleGroup != "" && kind != "relay" {
		return q, validationError("sample_group only applies to relay")
	}
	if q.IndexCode != "" && kind != "indices" {
		return q, validationError("index_code only applies to indices")
	}
	if (q.Market != "" || q.Symbol != "") && kind != "events" && kind != "relay" && kind != "matrix" {
		return q, validationError("market/symbol only apply to events or relay")
	}
	if q.Theme != "" && kind != "events" && kind != "themes" && kind != "relay" && kind != "matrix" {
		return q, validationError("theme is not supported for this query")
	}
	return q, nil
}

func limitEnum(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

func limitQueryFromRequest(r *http.Request, kind string) (LimitQuery, error) {
	v := r.URL.Query()
	q := LimitQuery{TradeDate: v.Get("trade_date"), PrevTradeDate: v.Get("prev_trade_date"), Since: v.Get("since"), Until: v.Get("until"), Market: v.Get("market"), Symbol: v.Get("symbol"), EventType: v.Get("event_type"), SampleGroup: v.Get("sample_group"), IndexCode: v.Get("index_code"), Theme: v.Get("theme")}
	for _, field := range []struct {
		key    string
		target *int
	}{{"limit", &q.Limit}, {"offset", &q.Offset}} {
		if v.Has(field.key) {
			n, err := strconv.Atoi(v.Get(field.key))
			if err != nil || n < 0 || (field.key == "limit" && n == 0) {
				return q, validationError("invalid %s", field.key)
			}
			*field.target = n
		}
	}
	return NormalizeLimitQuery(q, kind)
}

func registerLimitList[T any](mux *http.ServeMux, path, kind string, read func(context.Context, LimitQuery) (LimitResult[T], error)) {
	mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
		q, err := limitQueryFromRequest(r, kind)
		if err != nil {
			writeError(w, statusForError(err), err)
			return
		}
		result, err := read(r.Context(), q)
		if err != nil {
			writeError(w, statusForError(err), err)
			return
		}
		if result.Rows == nil {
			result.Rows = []T{}
		}
		writeJSON(w, http.StatusOK, result)
	})
}

func (s *Server) registerLimitReviewRoutes(mux *http.ServeMux) {
	registerLimitList(mux, "/api/v1/limit-events", "events", s.repo.LimitEvents)
	registerLimitList(mux, "/api/v1/limit-summary", "summary", s.repo.LimitSummaries)
	registerLimitList(mux, "/api/v1/limit-relay", "relay", s.repo.LimitRelay)
	registerLimitList(mux, "/api/v1/limit-themes", "themes", s.repo.LimitThemes)
	registerLimitList(mux, "/api/v1/limit-performance-indices", "indices", s.repo.LimitPerformanceIndices)
	registerLimitList(mux, "/api/v1/market-breadth", "breadth", s.repo.MarketBreadth)
	mux.HandleFunc("GET /api/v1/limit-review", s.handleLimitReview)
	mux.HandleFunc("GET /api/v1/limit-review-matrix", s.handleLimitReviewMatrix)
}

// ReadCompleteLimitRows refuses partial input to aggregates and matrices.
func ReadCompleteLimitRows[T any](ctx context.Context, q LimitQuery, read func(context.Context, LimitQuery) (LimitResult[T], error)) ([]T, error) {
	rows := []T{}
	q.Limit, q.Offset = 20000, 0
	for {
		page, err := read(ctx, q)
		if err != nil {
			return nil, err
		}
		rows = append(rows, page.Rows...)
		if len(rows) > 200000 || (page.HasMore && len(rows) == 200000) {
			return nil, validationError("review reconstruction exceeds 200000 rows; narrow the range")
		}
		if !page.HasMore {
			return rows, nil
		}
		if len(page.Rows) != q.Limit {
			return nil, fmt.Errorf("incomplete review pagination")
		}
		q.Offset += len(page.Rows)
	}
}

func (c *HTTPClient) LimitEvents(ctx context.Context, q LimitQuery) (LimitResult[LimitEvent], error) {
	var result LimitResult[LimitEvent]
	err := c.getJSON(ctx, "/api/v1/limit-events", limitQueryValues(q), &result)
	return result, err
}

func (c *HTTPClient) LimitReview(ctx context.Context, date string) (LimitReview, error) {
	var result LimitReview
	err := c.getJSON(ctx, "/api/v1/limit-review", url.Values{"trade_date": {date}}, &result)
	return result, err
}

func limitQueryValues(q LimitQuery) url.Values {
	v := url.Values{}
	for k, value := range map[string]string{"trade_date": q.TradeDate, "prev_trade_date": q.PrevTradeDate, "since": q.Since, "until": q.Until, "market": q.Market, "symbol": q.Symbol, "event_type": q.EventType, "theme": q.Theme, "sample_group": q.SampleGroup, "index_code": q.IndexCode} {
		if value != "" {
			v.Set(k, value)
		}
	}
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Offset > 0 {
		v.Set("offset", strconv.Itoa(q.Offset))
	}
	return v
}
