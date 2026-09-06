package tdx

import (
	"context"
	"fmt"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/marketdata"
)

const maxMarketDataPages = 64

type MarketDataOptions struct {
	ClientOptions     QuoteClientOptions
	FetchSecurityBars func(context.Context, HQBarsRequest, QuoteClientOptions) ([]HQBar, error)
	FetchIndexBars    func(context.Context, HQBarsRequest, QuoteClientOptions) ([]HQBar, error)
	Now               func() time.Time
}

// MarketDataProvider adapts data products without changing the TDX wire API.
type MarketDataProvider struct{ opts MarketDataOptions }

var _ marketdata.BarsProvider = (*MarketDataProvider)(nil)

func NewMarketDataProvider(opts MarketDataOptions) *MarketDataProvider {
	if opts.FetchSecurityBars == nil {
		opts.FetchSecurityBars = FetchHQSecurityBars
	}
	if opts.FetchIndexBars == nil {
		opts.FetchIndexBars = FetchHQIndexBars
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	// Own the slice values so caller changes cannot race with serving requests.
	opts.ClientOptions.Servers = append([]string(nil), opts.ClientOptions.Servers...)
	return &MarketDataProvider{opts: opts}
}

func (*MarketDataProvider) ID() string { return "tdx" }

func (*MarketDataProvider) BarsCapabilities() []marketdata.BarsCapability {
	return []marketdata.BarsCapability{
		{Kind: "index", Markets: []string{"sh", "sz", "bj"}, Periods: []string{"1d", "1m", "5m", "15m", "30m", "60m"}},
		{Kind: "security", Markets: []string{"sh", "sz", "bj"}, Periods: []string{"1d", "1m", "5m", "15m", "30m", "60m"}},
	}
}

func (p *MarketDataProvider) Bars(ctx context.Context, query marketdata.BarsQuery) (marketdata.BarsResult, error) {
	q, err := marketdata.NormalizeBarsQuery(query, p.opts.Now())
	if err != nil {
		return marketdata.BarsResult{}, err
	}
	category, ok := map[string]int{"1d": HQKLineDayAlt, "1m": HQKLine1Min, "5m": HQKLine5Min, "15m": HQKLine15Min, "30m": HQKLine30Min, "60m": HQKLine1Hour}[q.Period]
	if !ok {
		return marketdata.BarsResult{}, fmt.Errorf("%w: TDX period %q", marketdata.ErrUnsupported, q.Period)
	}
	if q.Instrument.Market != "sh" && q.Instrument.Market != "sz" && q.Instrument.Market != "bj" {
		return marketdata.BarsResult{}, fmt.Errorf("%w: TDX market %q", marketdata.ErrUnsupported, q.Instrument.Market)
	}
	req, err := ParseHQBarsRequest(category, q.Instrument.Market, q.Instrument.Symbol, 0, MaxHQKLineCount)
	if err != nil {
		return marketdata.BarsResult{}, fmt.Errorf("%w: %s", marketdata.ErrInvalid, err)
	}
	fetch := p.opts.FetchSecurityBars
	if q.Instrument.Kind == "index" {
		fetch = p.opts.FetchIndexBars
	}
	loc, err := time.LoadLocation(marketdata.Timezone)
	if err != nil {
		return marketdata.BarsResult{}, err
	}
	since, _ := time.ParseInLocation("2006-01-02", q.Since, loc)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	rows := []marketdata.Bar{}
	var oldest time.Time
	exhausted := false
	finished := false
	for page := 0; page < maxMarketDataPages; page++ {
		if err := ctx.Err(); err != nil {
			return marketdata.BarsResult{}, err
		}
		bars, err := fetch(ctx, req, p.opts.ClientOptions)
		if err != nil {
			return marketdata.BarsResult{}, fmt.Errorf("%w: TDX bars: %w", marketdata.ErrUpstream, err)
		}
		if len(bars) > req.Count {
			return marketdata.BarsResult{}, fmt.Errorf("%w: TDX page exceeds requested count", marketdata.ErrPayload)
		}
		if len(bars) == 0 {
			exhausted, finished = true, true
			break
		}
		var pageOldest, pageNewest time.Time
		for _, b := range bars {
			if b.Market != req.Market || b.Symbol != req.Symbol || b.Category != req.Category {
				return marketdata.BarsResult{}, fmt.Errorf("%w: TDX bar identity mismatch", marketdata.ErrPayload)
			}
			t, err := time.ParseInLocation("2006-01-02 15:04", b.DateTime, loc)
			if err != nil {
				return marketdata.BarsResult{}, fmt.Errorf("%w: invalid TDX datetime", marketdata.ErrPayload)
			}
			stamp := t.Format(time.RFC3339)
			if q.Period == "1d" {
				stamp = t.Format("2006-01-02")
				t, _ = time.ParseInLocation("2006-01-02", stamp, loc)
			}
			if pageOldest.IsZero() || t.Before(pageOldest) {
				pageOldest = t
			}
			if pageNewest.IsZero() || t.After(pageNewest) {
				pageNewest = t
			}
			rows = append(rows, marketdata.Bar{Time: stamp, Open: b.Open, High: b.High, Low: b.Low, Close: b.Close, Volume: b.Volume, Amount: b.Amount})
		}
		if !oldest.IsZero() && (!pageOldest.Before(oldest) || pageNewest.After(oldest)) {
			return marketdata.BarsResult{}, fmt.Errorf("%w: TDX history page overlaps or does not progress", marketdata.ErrPayload)
		}
		oldest = pageOldest
		if !oldest.After(since) {
			finished = true
			break
		}
		if len(bars) < req.Count {
			exhausted, finished = true, true
			break
		}
		req.Start += len(bars)
	}
	if !finished {
		return marketdata.BarsResult{}, fmt.Errorf("%w: TDX history requires more than %d pages; narrow the date range or use the raw TDX API", marketdata.ErrLimit, maxMarketDataPages)
	}
	result, err := marketdata.NewBarsResult(p.ID(), q, "hand", rows)
	if err != nil {
		return marketdata.BarsResult{}, err
	}
	if exhausted && (oldest.IsZero() || oldest.After(since)) {
		boundary := "no bars"
		if !oldest.IsZero() {
			boundary = oldest.Format(time.RFC3339)
		}
		result.Warnings = append(result.Warnings, "TDX available history ended at "+boundary+"; coverage back to since is not established.")
	}
	return result, nil
}
