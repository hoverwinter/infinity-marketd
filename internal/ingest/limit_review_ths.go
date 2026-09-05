package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

const thsReviewBaseURL = "https://data.10jqka.com.cn/dataapi/limit_up"

type THSReviewOptions struct {
	LoadEvents      func(context.Context, string) ([]model.LimitEvent, error)
	Date            string
	DryRun          bool
	Store           LimitReviewWriter
	Now             func() time.Time
	Client          *http.Client
	BaseURL         string
	RequestInterval time.Duration
}

type thsPoolStock struct {
	Code          string   `json:"code"`
	HighDays      string   `json:"high_days"`
	Reason        string   `json:"reason_type"`
	First         *string  `json:"first_limit_up_time"`
	Last          *string  `json:"last_limit_up_time"`
	FirstDown     *string  `json:"first_limit_down_time"`
	LastDown      *string  `json:"last_limit_down_time"`
	OpenNum       *uint16  `json:"open_num"`
	OrderAmount   *float64 `json:"order_amount"`
	Turnover      *float64 `json:"turnover"`
	TurnoverRate  *float64 `json:"turnover_rate"`
	CurrencyValue *float64 `json:"currency_value"`
}

type thsPoolPage struct {
	Date string                                   `json:"date"`
	Page *struct{ Limit, Total, Count, Page int } `json:"page"`
	Info []thsPoolStock                           `json:"info"`
}

type thsReviewClient struct {
	client   *http.Client
	base     string
	interval time.Duration
	last     time.Time
	pools    map[string][]thsPoolStock
	previous map[string]string
}

func (c *thsReviewClient) get(ctx context.Context, path string, values url.Values, out any) error {
	if wait := c.interval - time.Since(c.last); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/"+path+"?"+values.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	c.last = time.Now()
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("THS %s: HTTP %d", path, resp.StatusCode)
	}
	var envelope struct {
		Status  *int            `json:"status_code"`
		Message string          `json:"status_msg"`
		Data    json.RawMessage `json:"data"`
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20+1))
	if err != nil {
		return err
	}
	if len(raw) > 16<<20 {
		return fmt.Errorf("THS response too large")
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("THS %s: %w", path, err)
	}
	if envelope.Status == nil || *envelope.Status != 0 || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("THS %s rejected response: %s", path, envelope.Message)
	}
	return json.Unmarshal(envelope.Data, out)
}

func (c *thsReviewClient) prev(ctx context.Context, date string) (string, error) {
	if prev, ok := c.previous[date]; ok {
		return prev, nil
	}
	var day struct {
		Code      *int     `json:"code"`
		TradeDay  bool     `json:"trade_day"`
		PrevDates []string `json:"prev_dates"`
	}
	if err := c.get(ctx, "trade_day", url.Values{"date": {date}, "stock": {"stock"}, "prev": {"1"}, "next": {"1"}}, &day); err != nil {
		return "", err
	}
	if day.Code == nil || *day.Code != 0 || !day.TradeDay || len(day.PrevDates) != 1 {
		return "", fmt.Errorf("THS %s: trading day/previous date unavailable", date)
	}
	prev := day.PrevDates[0]
	if _, err := time.Parse("20060102", prev); err != nil || prev >= date {
		return "", fmt.Errorf("THS %s: invalid previous date %q", date, prev)
	}
	c.previous[date] = prev
	return prev, nil
}

func (c *thsReviewClient) pool(ctx context.Context, path, date string) ([]thsPoolStock, error) {
	key := path + date
	if rows, ok := c.pools[key]; ok {
		return rows, nil
	}
	fields := map[string]string{
		"limit_up_pool":    "199112,10,9001,330323,330324,330325,9002,330329,133971,133970,1968584,3475914",
		"open_limit_pool":  "199112,9002,48,1968584,19,3475914,10",
		"lower_limit_pool": "199112,10,330333,330334,1968584,3475914",
	}
	rows := []thsPoolStock{}
	seen := map[string]bool{}
	total := -1
	for page := 1; page <= 100; page++ {
		var p thsPoolPage
		v := url.Values{"date": {date}, "field": {fields[path]}, "filter": {"HS,GEM2STAR,ST"}, "order_field": {"199112"}, "order_type": {"0"}, "page": {strconv.Itoa(page)}, "limit": {"200"}}
		if err := c.get(ctx, path, v, &p); err != nil {
			return nil, err
		}
		if p.Date != date || p.Page == nil || p.Page.Page != page || p.Page.Total < 0 || p.Page.Total > 20000 || p.Page.Limit < 1 {
			return nil, fmt.Errorf("THS %s: invalid date/pagination on page %d", path, page)
		}
		if total < 0 {
			total = p.Page.Total
		} else if total != p.Page.Total {
			return nil, fmt.Errorf("THS %s changed total during pagination", path)
		}
		for _, row := range p.Info {
			if !validReviewSymbol(row.Code) || seen[row.Code] {
				return nil, fmt.Errorf("THS %s repeated/invalid symbol %q", path, row.Code)
			}
			seen[row.Code] = true
			rows = append(rows, row)
		}
		if len(rows) > total {
			return nil, fmt.Errorf("THS %s exceeds declared total", path)
		}
		if len(rows) == total {
			c.pools[key] = rows
			return rows, nil
		}
		if len(p.Info) == 0 || page >= p.Page.Count {
			return nil, fmt.Errorf("THS %s incomplete pagination: %d/%d", path, len(rows), total)
		}
	}
	return nil, fmt.Errorf("THS %s pagination limit exceeded", path)
}

var thsBoardLabel = regexp.MustCompile(`^(\d+)天(\d+)板$`)

func thsKnownStreak(label string) (uint16, bool, error) {
	label = strings.TrimSpace(label)
	if label == "首板" {
		return 1, true, nil
	}
	m := thsBoardLabel.FindStringSubmatch(label)
	if m == nil {
		return 0, false, fmt.Errorf("unsupported THS board label %q", label)
	}
	days, e1 := strconv.ParseUint(m[1], 10, 16)
	boards, e2 := strconv.ParseUint(m[2], 10, 16)
	if e1 != nil || e2 != nil || boards == 0 || days < boards {
		return 0, false, fmt.Errorf("invalid THS board label %q", label)
	}
	return uint16(boards), days == boards, nil
}

// Multi-day multi-board labels do not describe the consecutive suffix.
func (c *thsReviewClient) streak(ctx context.Context, date string, row thsPoolStock) (uint16, error) {
	count := uint16(0)
	for depth := 0; depth < 60; depth++ {
		n, known, err := thsKnownStreak(row.HighDays)
		if err != nil {
			return 0, err
		}
		if known {
			if uint32(count)+uint32(n) > 65535 {
				return 0, fmt.Errorf("board count overflow")
			}
			return count + n, nil
		}
		count++
		date, err = c.prev(ctx, date)
		if err != nil {
			return 0, err
		}
		prior, err := c.pool(ctx, "limit_up_pool", date)
		if err != nil {
			return 0, err
		}
		found := false
		for _, candidate := range prior {
			if candidate.Code == row.Code {
				row = candidate
				found = true
				break
			}
		}
		if !found {
			return count, nil
		}
	}
	return 0, fmt.Errorf("THS consecutive streak exceeds lookback limit")
}

func thsMinute(raw *string, date time.Time) (*string, error) {
	if raw == nil || *raw == "" || *raw == "-" {
		return nil, nil
	}
	seconds, err := strconv.ParseInt(*raw, 10, 64)
	if err != nil || seconds <= 0 {
		return nil, fmt.Errorf("invalid THS time %q", *raw)
	}
	t := time.Unix(seconds, 0).In(date.Location())
	if t.Format("2006-01-02") != date.Format("2006-01-02") {
		return nil, fmt.Errorf("THS time outside requested date")
	}
	minute := t.Format("15:04")
	return &minute, nil
}

func RefreshTHSLimitReview(ctx context.Context, opts THSReviewOptions) (LimitReviewImportSummary, error) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	date, err := parseReviewDate(opts.Date, loc)
	if err != nil {
		return LimitReviewImportSummary{}, err
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	now := opts.Now().In(loc)
	if date.After(now) || (date.Format("2006-01-02") == now.Format("2006-01-02") && now.Before(date.Add(15*time.Hour+5*time.Minute))) {
		return LimitReviewImportSummary{}, fmt.Errorf("THS review requires a closed trading day")
	}
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 20 * time.Second}
	}
	if opts.BaseURL == "" {
		opts.BaseURL = thsReviewBaseURL
	}
	if opts.RequestInterval == 0 {
		opts.RequestInterval = time.Second
	}
	c := &thsReviewClient{client: opts.Client, base: strings.TrimRight(opts.BaseURL, "/"), interval: opts.RequestInterval, pools: map[string][]thsPoolStock{}, previous: map[string]string{}}
	params, _ := json.Marshal(map[string]any{"date": opts.Date, "provider": "ths", "filter": "HS,GEM2STAR,ST", "board_count": "consecutive", "operation": "provider_refresh_preserve_enrichment", "validated_pools": []string{"limit_up_pool", "open_limit_pool", "lower_limit_pool"}, "coverage": "provider_filtered_pools_only", "target_tables": []string{"a_share_limit_events", "a_share_limit_daily_summary"}})
	var bundle limitReviewBundle
	writeStarted := false
	result, err := RunOnlineJob(ctx, OnlineJob[limitReviewBundle]{Dataset: limitReviewDataset, TargetTable: limitReviewTarget, TaskType: "ths_limit_review_refresh", InputFormat: "ths.limit_up.pools", Asset: "all", Params: string(params), DryRun: opts.DryRun, Ops: opts.Store, Now: opts.Now,
		Produce: func(ctx context.Context, runID string) ([]limitReviewBundle, uint64, []model.QualityIssue, error) {
			prev, err := c.prev(ctx, date.Format("20060102"))
			if err != nil {
				return nil, 0, nil, err
			}
			prevDate, _ := time.ParseInLocation("20060102", prev, loc)
			for _, pool := range []struct{ path, event string }{{"limit_up_pool", "limit_up"}, {"open_limit_pool", "open_limit"}, {"lower_limit_pool", "limit_down"}} {
				rows, err := c.pool(ctx, pool.path, date.Format("20060102"))
				if err != nil {
					return nil, 0, nil, err
				}
				for _, r := range rows {
					var boards uint16
					if pool.event == "limit_up" {
						boards, err = c.streak(ctx, date.Format("20060102"), r)
						if err != nil {
							return nil, 0, nil, fmt.Errorf("%s: %w", r.Code, err)
						}
					}
					first, last := r.First, r.Last
					if pool.event == "limit_down" {
						first, last = r.FirstDown, r.LastDown
					}
					first, err = thsMinute(first, date)
					if err != nil {
						return nil, 0, nil, err
					}
					last, err = thsMinute(last, date)
					if err != nil {
						return nil, 0, nil, err
					}
					event, err := normalizeLimitStock(date, pool.event, rawLimitStock{Code: r.Code, BoardCount: boards, ReasonType: r.Reason, FirstLimitUpTime: first, LastLimitUpTime: last, OrderAmount: r.OrderAmount, Amount: r.Turnover, TurnoverRate: r.TurnoverRate, OpenNum: r.OpenNum, MarketValue: r.CurrencyValue})
					if err != nil {
						return nil, 0, nil, err
					}
					bundle.Events = append(bundle.Events, event)
				}
			}
			summary := model.LimitDailySummary{TradeDate: date, PrevTradeDate: &prevDate}
			closing := map[string]string{}
			for _, e := range bundle.Events {
				if e.EventType == "limit_up" || e.EventType == "limit_down" {
					if old, ok := closing[e.Symbol]; ok && old != e.EventType {
						return nil, 0, nil, fmt.Errorf("THS conflicting close pools for %s", e.Symbol)
					}
					closing[e.Symbol] = e.EventType
				}
			}
			var unsealed uint32
			for i := range bundle.Events {
				e := &bundle.Events[i]
				if e.EventType != "open_limit" {
					continue
				}
				switch closing[e.Symbol] {
				case "limit_up":
					e.CloseStatus = "broken_reseal"
				case "limit_down":
					e.CloseStatus = "limit_down"
				}
				if e.CloseStatus != "broken_reseal" {
					unsealed++
				}
			}
			for _, e := range bundle.Events {
				switch e.EventType {
				case "limit_up":
					summary.LimitUpCount++
					if e.BoardCount == 1 {
						summary.FirstBoardCount++
					} else {
						summary.ContinuousBoardCount++
					}
					if e.BoardCount > summary.MaxBoardHeight {
						summary.MaxBoardHeight = e.BoardCount
					}
				case "open_limit":
					summary.OpenLimitCount++
				case "limit_down":
					summary.LimitDownCount++
				}
			}
			heights := countBoardHeights(bundle.Events)
			summary.TwoBoardCount, summary.ThreeBoardCount, summary.FourBoardCount, summary.FivePlusBoardCount = heights[2], heights[3], heights[4], heights[5]
			if total := summary.LimitUpCount + unsealed; total > 0 {
				rate := float64(summary.LimitUpCount) / float64(total)
				summary.SealSuccessRate = &rate
			}
			bundle.Summaries = []model.LimitDailySummary{summary}
			return []limitReviewBundle{bundle}, 0, nil, nil
		},
		Write: func(ctx context.Context, rows []limitReviewBundle) error {
			if opts.LoadEvents == nil {
				return fmt.Errorf("current-event reader required for provider refresh")
			}
			current, err := opts.LoadEvents(ctx, opts.Date)
			if err != nil {
				return err
			}
			byKey := map[string]model.LimitEvent{}
			for _, row := range current {
				byKey[row.Market+":"+row.Symbol+":"+row.EventType] = row
			}
			for i, row := range rows[0].Events {
				if old, ok := byKey[row.Market+":"+row.Symbol+":"+row.EventType]; ok {
					rows[0].Events[i] = preserveLimitEnrichment(old, row)
				}
			}
			writeStarted = true
			_, err = writeLimitReviewBundle(ctx, opts.Store, rows[0])
			return err
		},
		CountRows: func(rows []limitReviewBundle) uint64 {
			if len(rows) == 0 {
				return 0
			}
			return uint64(len(rows[0].Events) + len(rows[0].Summaries))
		},
		Bounds: func(rows []limitReviewBundle) (*time.Time, *time.Time) { return &date, &date },
	})
	if err != nil && !writeStarted {
		result.RowsWritten = 0
	}
	return LimitReviewImportSummary{RunID: result.RunID, Dataset: result.Dataset, TargetTable: result.TargetTable, Events: uint64(len(bundle.Events)), DailySummaries: uint64(len(bundle.Summaries)), RowsWritten: result.RowsWritten, RowsSkipped: result.RowsSkipped, Issues: result.Issues, DryRun: result.DryRun}, err
}

func preserveLimitEnrichment(old, next model.LimitEvent) model.LimitEvent {
	if old.ReasonText != "" {
		next.ReasonText = old.ReasonText
	}
	if old.ThemePrimary != "" && old.ThemePrimary != "未分类" {
		next.ThemePrimary = old.ThemePrimary
	}
	if len(old.ThemeTags) != 0 {
		next.ThemeTags = old.ThemeTags
	}
	if next.FirstLimitMinute == nil {
		next.FirstLimitMinute = old.FirstLimitMinute
	}
	if next.LastLimitMinute == nil {
		next.LastLimitMinute = old.LastLimitMinute
	}
	if next.OpenCount == nil {
		next.OpenCount = old.OpenCount
	}
	if next.SealOrderAmount == nil {
		next.SealOrderAmount = old.SealOrderAmount
	}
	if next.Amount == nil {
		next.Amount = old.Amount
	}
	if next.TurnoverRate == nil {
		next.TurnoverRate = old.TurnoverRate
	}
	if next.MarketValue == nil {
		next.MarketValue = old.MarketValue
	}
	return next
}
