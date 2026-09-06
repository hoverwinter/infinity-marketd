package tdx

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/marketdata"
)

func providerQuery() marketdata.BarsQuery {
	return marketdata.BarsQuery{Instrument: marketdata.Instrument{Kind: "index", Market: "sh", Symbol: "000001"}, Period: "1d", Since: "2026-09-03", Until: "2026-09-04"}
}

func providerNow() time.Time { return time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC) }

func providerBar(req HQBarsRequest, stamp string) HQBar {
	return HQBar{Market: req.Market, Symbol: req.Symbol, Category: req.Category, DateTime: stamp, Open: 10, High: 12, Low: 9, Close: 11, Volume: 100, Amount: 1000}
}

func TestMarketDataBarsPagination(t *testing.T) {
	latest := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	calls := 0
	p := NewMarketDataProvider(MarketDataOptions{Now: providerNow, FetchIndexBars: func(ctx context.Context, req HQBarsRequest, opts QuoteClientOptions) ([]HQBar, error) {
		calls++
		if req.Category != HQKLineDayAlt || req.Count != MaxHQKLineCount {
			t.Fatalf("wrong wire mapping: %+v", req)
		}
		if req.Start == 0 {
			rows := make([]HQBar, MaxHQKLineCount)
			for i := range rows {
				rows[i] = providerBar(req, latest.AddDate(0, 0, -i).Format("2006-01-02 15:04"))
			}
			return rows, nil
		}
		if req.Start != MaxHQKLineCount {
			t.Fatalf("wrong cursor %d", req.Start)
		}
		// Identical boundary duplicates are harmless; conflicting ones are not.
		return []HQBar{providerBar(req, latest.AddDate(0, 0, -799).Format("2006-01-02 15:04")), providerBar(req, latest.AddDate(0, 0, -800).Format("2006-01-02 15:04"))}, nil
	}})
	q := providerQuery()
	q.Since = latest.AddDate(0, 0, -800).Format("2006-01-02")
	r, err := p.Bars(context.Background(), q)
	if err != nil || calls != 2 || len(r.Bars) != 801 || r.Bars[0].Time != q.Since || r.VolumeUnit != "hand" || len(r.Warnings) != 0 {
		t.Fatalf("rows=%d calls=%d warnings=%v err=%v", len(r.Bars), calls, r.Warnings, err)
	}
}

func TestMarketDataIntradayLowerBoundIncludesMorning(t *testing.T) {
	calls := 0
	p := NewMarketDataProvider(MarketDataOptions{Now: providerNow, FetchSecurityBars: func(ctx context.Context, req HQBarsRequest, _ QuoteClientOptions) ([]HQBar, error) {
		calls++
		if req.Category != HQKLine1Min {
			t.Fatalf("wrong period: %+v", req)
		}
		if req.Start == 0 {
			rows := make([]HQBar, MaxHQKLineCount)
			start := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
			for i := range rows {
				rows[i] = providerBar(req, start.Add(time.Duration(i)*time.Minute).Format("2006-01-02 15:04"))
			}
			return rows, nil
		}
		return []HQBar{providerBar(req, "2026-09-04 09:31"), providerBar(req, "2026-09-03 15:00")}, nil
	}})
	q := providerQuery()
	q.Instrument.Kind, q.Period, q.Since = "security", "1m", "2026-09-04"
	r, err := p.Bars(context.Background(), q)
	if err != nil || calls != 2 || len(r.Bars) != 801 || r.Bars[0].Time != "2026-09-04T09:31:00+08:00" {
		t.Fatalf("rows=%d calls=%d err=%v", len(r.Bars), calls, err)
	}
}

func TestMarketDataFailures(t *testing.T) {
	for _, tc := range []struct {
		name  string
		fetch func(HQBarsRequest) ([]HQBar, error)
		want  error
	}{
		{"upstream", func(HQBarsRequest) ([]HQBar, error) { return nil, errors.New("offline") }, marketdata.ErrUpstream},
		{"identity", func(r HQBarsRequest) ([]HQBar, error) {
			b := providerBar(r, "2026-09-04 15:00")
			b.Symbol = "600519"
			return []HQBar{b}, nil
		}, marketdata.ErrPayload},
		{"date", func(r HQBarsRequest) ([]HQBar, error) { return []HQBar{providerBar(r, "2026-02-30 15:00")}, nil }, marketdata.ErrPayload},
		{"ohlc", func(r HQBarsRequest) ([]HQBar, error) {
			b := providerBar(r, "2026-09-04 15:00")
			b.High = 8
			return []HQBar{b}, nil
		}, marketdata.ErrPayload},
		{"duplicate-conflict", func(r HQBarsRequest) ([]HQBar, error) {
			a := providerBar(r, "2026-09-04 15:00")
			b := a
			b.Close = 10
			return []HQBar{a, b}, nil
		}, marketdata.ErrPayload},
		{"stalled-page", func(r HQBarsRequest) ([]HQBar, error) {
			rows := make([]HQBar, MaxHQKLineCount)
			for i := range rows {
				rows[i] = providerBar(r, "2026-09-04 15:00")
			}
			return rows, nil
		}, marketdata.ErrPayload},
		{"scan-budget", func(r HQBarsRequest) ([]HQBar, error) {
			rows := make([]HQBar, MaxHQKLineCount)
			date := providerNow().AddDate(0, 0, -r.Start/MaxHQKLineCount)
			for i := range rows {
				rows[i] = providerBar(r, date.Format("2006-01-02 15:04"))
			}
			return rows, nil
		}, marketdata.ErrLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := NewMarketDataProvider(MarketDataOptions{Now: providerNow, FetchIndexBars: func(_ context.Context, r HQBarsRequest, _ QuoteClientOptions) ([]HQBar, error) { return tc.fetch(r) }})
			q := providerQuery()
			q.Since = "2026-01-01"
			r, err := p.Bars(context.Background(), q)
			if !errors.Is(err, tc.want) || len(r.Bars) != 0 {
				t.Fatalf("partial/incorrect result: rows=%d err=%v", len(r.Bars), err)
			}
		})
	}
}

func TestMarketDataHistoryBoundaryAndCancellation(t *testing.T) {
	calls := 0
	p := NewMarketDataProvider(MarketDataOptions{Now: providerNow, FetchIndexBars: func(_ context.Context, r HQBarsRequest, _ QuoteClientOptions) ([]HQBar, error) {
		calls++
		return []HQBar{providerBar(r, "2026-09-04 15:00")}, nil
	}})
	r, err := p.Bars(context.Background(), providerQuery())
	if err != nil || len(r.Bars) != 1 || len(r.Warnings) != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Bars(ctx, providerQuery()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	q := providerQuery()
	q.Period = "1w"
	if _, err := p.Bars(context.Background(), q); !errors.Is(err, marketdata.ErrUnsupported) {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("cancelled/invalid request reached upstream: %d", calls)
	}
}

func TestLiveMarketDataTDX(t *testing.T) {
	server := os.Getenv("MARKETD_PROVIDER_TDX_PROBE")
	if server == "" {
		t.Skip("set MARKETD_PROVIDER_TDX_PROBE=host:port for live TDX verification")
	}
	p := NewMarketDataProvider(MarketDataOptions{ClientOptions: QuoteClientOptions{Server: server, Timeout: 5 * time.Second}})
	q := providerQuery()
	for _, kind := range []string{"index", "security"} {
		q.Instrument.Kind = kind
		if kind == "security" {
			q.Instrument.Symbol = "600519"
		}
		r, err := p.Bars(context.Background(), q)
		if err != nil || len(r.Bars) != 2 {
			t.Fatalf("rows=%d err=%v", len(r.Bars), err)
		}
		t.Logf("provider=%s kind=%s symbol=%s bars=%d close=%f", r.Provider, kind, q.Instrument.Symbol, len(r.Bars), r.Bars[len(r.Bars)-1].Close)
	}
}
