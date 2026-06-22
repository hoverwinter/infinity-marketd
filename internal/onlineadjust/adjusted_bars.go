package onlineadjust

import (
	"context"
	"fmt"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/adjust"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

type HQAdjustedBarsOnlineRequest struct {
	Category int    `json:"category"`
	Market   string `json:"market"`
	Symbol   string `json:"symbol"`
	Start    int    `json:"start"`
	Count    int    `json:"count"`
	Adjust   string `json:"adjust"`
}

type HQAdjustedBarsOnlineResult struct {
	Query  HQAdjustedBarsOnlineRequest `json:"query"`
	Source string                      `json:"source"`
	Bars   []HQAdjustedBar             `json:"bars"`
}

type HQAdjustedBar struct {
	Market   string  `json:"market"`
	Symbol   string  `json:"symbol"`
	Category int     `json:"category"`
	DateTime string  `json:"datetime"`
	Open     float64 `json:"open"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	Close    float64 `json:"close"`
	Volume   float64 `json:"volume"`
	Amount   float64 `json:"amount"`
	Adjust   string  `json:"adjust"`
	Factor   float64 `json:"factor"`
}

var fetchHQSecurityBars = tdx.FetchHQSecurityBars
var fetchHQXDXRInfo = tdx.FetchHQXDXRInfo

const maxDailyFactorPages = 64

func FetchHQAdjustedBarsOnline(ctx context.Context, req HQAdjustedBarsOnlineRequest, opts tdx.QuoteClientOptions) (HQAdjustedBarsOnlineResult, error) {
	normalized, err := NormalizeRequest(req)
	if err != nil {
		return HQAdjustedBarsOnlineResult{}, err
	}
	hqReq, err := tdx.ParseHQBarsRequest(normalized.Category, normalized.Market, normalized.Symbol, normalized.Start, normalized.Count)
	if err != nil {
		return HQAdjustedBarsOnlineResult{}, err
	}
	rawBars, err := fetchHQSecurityBars(ctx, hqReq, opts)
	if err != nil {
		return HQAdjustedBarsOnlineResult{}, err
	}
	result := HQAdjustedBarsOnlineResult{Query: normalized, Source: "tdx-live-provider", Bars: make([]HQAdjustedBar, 0, len(rawBars))}
	if len(rawBars) == 0 {
		return result, nil
	}
	if normalized.Adjust == "none" {
		for _, bar := range rawBars {
			result.Bars = append(result.Bars, adjustedBarFromHQ(bar, normalized.Adjust, 1))
		}
		return result, nil
	}
	dailyBars, err := fetchDailyBarsForFactors(ctx, normalized, rawBars, opts)
	if err != nil {
		return HQAdjustedBarsOnlineResult{}, err
	}
	xdxrReq, err := tdx.ParseHQMinuteRequest(normalized.Market, normalized.Symbol)
	if err != nil {
		return HQAdjustedBarsOnlineResult{}, err
	}
	xdxrRows, err := fetchHQXDXRInfo(ctx, xdxrReq, opts)
	if err != nil {
		return HQAdjustedBarsOnlineResult{}, err
	}
	events, normalizeIssues := adjust.NormalizeXDXR(xdxrRows)
	if len(normalizeIssues) > 0 {
		return HQAdjustedBarsOnlineResult{}, fmt.Errorf("online adjusted bars xdxr normalization issue: %s", normalizeIssues[0].Message)
	}
	factors, factorIssues := adjust.GenerateFactors(dailyBars, events, time.Now())
	for _, issue := range factorIssues {
		switch issue.Type {
		case "missing_previous_close", "missing_xdxr_fields", "invalid_xdxr_denominator", "invalid_adjust_ratio", "zero_daily_bars":
			return HQAdjustedBarsOnlineResult{}, fmt.Errorf("online adjusted bars missing factor input for %s:%s %s: %s", normalized.Market, normalized.Symbol, issue.TradeDate.Format("2006-01-02"), issue.Message)
		}
	}
	factorByDate := make(map[string]float64, len(factors))
	for _, factor := range factors {
		var value *float64
		if normalized.Adjust == "hfq" {
			value = factor.HFQFactor
		} else {
			value = factor.QFQFactor
		}
		if value != nil {
			factorByDate[factor.TradeDate.Format("2006-01-02")] = *value
		}
	}
	for _, bar := range rawBars {
		date, err := hqBarDate(bar)
		if err != nil {
			return HQAdjustedBarsOnlineResult{}, err
		}
		factor, ok := factorByDate[date.Format("2006-01-02")]
		if !ok {
			return HQAdjustedBarsOnlineResult{}, fmt.Errorf("missing %s adjustment factor for %s:%s on %s", normalized.Adjust, normalized.Market, normalized.Symbol, date.Format("2006-01-02"))
		}
		result.Bars = append(result.Bars, adjustedBarFromHQ(bar, normalized.Adjust, factor))
	}
	return result, nil
}

func NormalizeRequest(req HQAdjustedBarsOnlineRequest) (HQAdjustedBarsOnlineRequest, error) {
	if req.Count == 0 {
		req.Count = tdx.DefaultHQKLineCount
	}
	if req.Adjust == "" {
		req.Adjust = "none"
	}
	switch req.Adjust {
	case "none", "qfq", "hfq":
	default:
		return HQAdjustedBarsOnlineRequest{}, fmt.Errorf("adjust must be none, qfq, or hfq")
	}
	if _, err := tdx.ParseHQBarsRequest(req.Category, req.Market, req.Symbol, req.Start, req.Count); err != nil {
		return HQAdjustedBarsOnlineRequest{}, err
	}
	return req, nil
}

func fetchDailyBarsForFactors(ctx context.Context, req HQAdjustedBarsOnlineRequest, rawBars []tdx.HQBar, opts tdx.QuoteClientOptions) ([]model.DailyBar, error) {
	minRequestedDate, err := minHQBarDate(rawBars)
	if err != nil {
		return nil, err
	}
	bars := make([]model.DailyBar, 0, tdx.MaxHQKLineCount)
	seen := make(map[string]struct{})
	var earliest time.Time
	for page := 0; page < maxDailyFactorPages; page++ {
		start := page * tdx.MaxHQKLineCount
		dailyReq, err := tdx.ParseHQBarsRequest(tdx.HQKLineDayAlt, req.Market, req.Symbol, start, tdx.MaxHQKLineCount)
		if err != nil {
			return nil, err
		}
		rows, err := fetchHQSecurityBars(ctx, dailyReq, opts)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			date, err := hqBarDate(row)
			if err != nil {
				return nil, err
			}
			if earliest.IsZero() || date.Before(earliest) {
				earliest = date
			}
			key := date.Format("2006-01-02")
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			bars = append(bars, dailyBarFromHQ(row, date))
		}
		if !earliest.After(minRequestedDate) {
			break
		}
		if len(rows) < tdx.MaxHQKLineCount {
			break
		}
	}
	if len(bars) == 0 {
		return nil, fmt.Errorf("online adjusted bars missing daily history for %s:%s", req.Market, req.Symbol)
	}
	if earliest.After(minRequestedDate) {
		return nil, fmt.Errorf("online adjusted bars insufficient daily history for %s:%s: earliest daily bar %s is after requested %s", req.Market, req.Symbol, earliest.Format("2006-01-02"), minRequestedDate.Format("2006-01-02"))
	}
	return bars, nil
}

func minHQBarDate(rows []tdx.HQBar) (time.Time, error) {
	var minDate time.Time
	for _, row := range rows {
		date, err := hqBarDate(row)
		if err != nil {
			return time.Time{}, err
		}
		if minDate.IsZero() || date.Before(minDate) {
			minDate = date
		}
	}
	if minDate.IsZero() {
		return time.Time{}, fmt.Errorf("online adjusted bars missing requested raw bar dates")
	}
	return minDate, nil
}

func dailyBarFromHQ(row tdx.HQBar, date time.Time) model.DailyBar {
	return model.DailyBar{
		Market:    row.Market,
		Symbol:    row.Symbol,
		TradeDate: date,
		Open:      row.Open,
		High:      row.High,
		Low:       row.Low,
		Close:     row.Close,
		Volume:    uint64(row.Volume),
		Amount:    row.Amount,
	}
}

func hqBarDate(bar tdx.HQBar) (time.Time, error) {
	loc := shanghaiLoc()
	if bar.Year > 0 && bar.Month > 0 && bar.Day > 0 {
		return time.Date(bar.Year, time.Month(bar.Month), bar.Day, 0, 0, 0, 0, loc), nil
	}
	if len(bar.DateTime) >= 10 {
		return time.ParseInLocation("2006-01-02", bar.DateTime[:10], loc)
	}
	return time.Time{}, fmt.Errorf("HQ bar missing date: %+v", bar)
}

func adjustedBarFromHQ(bar tdx.HQBar, mode string, factor float64) HQAdjustedBar {
	return HQAdjustedBar{
		Market:   bar.Market,
		Symbol:   bar.Symbol,
		Category: bar.Category,
		DateTime: bar.DateTime,
		Open:     bar.Open * factor,
		High:     bar.High * factor,
		Low:      bar.Low * factor,
		Close:    bar.Close * factor,
		Volume:   bar.Volume,
		Amount:   bar.Amount,
		Adjust:   mode,
		Factor:   factor,
	}
}

func shanghaiLoc() *time.Location {
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return loc
	}
	return time.FixedZone("CST", 8*3600)
}
