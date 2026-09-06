package marketdata

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

func testQuery() BarsQuery {
	return BarsQuery{Instrument: Instrument{Kind: "index", Market: "board", Symbol: "881270"}, Period: "1d", Since: "2026-01-01", Until: "2026-01-06"}
}

func TestQueryValidation(t *testing.T) {
	now := time.Date(2026, 1, 6, 17, 0, 0, 0, time.UTC) // Jan 7 in Shanghai
	for _, tc := range []struct {
		name   string
		change func(*BarsQuery)
		want   error
	}{
		{"defaults", func(q *BarsQuery) { q.Period = ""; q.Until = "2026-01-07" }, nil},
		{"future", func(q *BarsQuery) { q.Until = "2026-01-08" }, ErrInvalid},
		{"missing-market", func(q *BarsQuery) { q.Instrument.Market = "" }, ErrInvalid},
		{"source-symbol", func(q *BarsQuery) { q.Instrument.Symbol = "BK1234" }, nil},
		{"unsafe-symbol", func(q *BarsQuery) { q.Instrument.Symbol = "../x" }, ErrInvalid},
		{"reversed", func(q *BarsQuery) { q.Since = "2026-01-07" }, ErrInvalid},
		{"invalid-date", func(q *BarsQuery) { q.Since = "2026-02-30" }, ErrInvalid},
		{"over-ten-years", func(q *BarsQuery) { q.Since = "2015-01-01" }, ErrInvalid},
		{"adjustment", func(q *BarsQuery) { q.Adjust = "qfq" }, ErrUnsupported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := testQuery()
			tc.change(&q)
			_, err := NormalizeBarsQuery(q, now)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestRowsValidationAndRange(t *testing.T) {
	b := Bar{Time: "2026-01-05", Open: 10, High: 12, Low: 9, Close: 11, Volume: 100, Amount: 1000}
	q := testQuery()
	later := b
	later.Time = "2026-01-06"
	outside := b
	outside.Time = "2025-12-31"
	rows, err := NormalizeBars([]Bar{later, b, b, outside}, q)
	if err != nil || len(rows) != 2 || rows[0] != b || rows[1] != later {
		t.Fatalf("%+v %v", rows, err)
	}
	for _, mutate := range []func(*Bar){
		func(b *Bar) { b.Time = "2026-02-30" },
		func(b *Bar) { b.Volume = math.NaN() },
		func(b *Bar) { b.Amount = math.Inf(1) },
		func(b *Bar) { b.Low = -1 },
		func(b *Bar) { b.High = 8 },
		func(b *Bar) { b.Close = 10.5 }, // duplicate timestamp with conflicting values
	} {
		bad := b
		mutate(&bad)
		if _, err := NormalizeBars([]Bar{b, bad}, q); !errors.Is(err, ErrPayload) {
			t.Fatalf("accepted %+v: %v", bad, err)
		}
	}
	q.Period = "1m"
	b.Time = "2026-01-05T09:31:00+08:00"
	if _, err := NormalizeBars([]Bar{b}, q); err != nil {
		t.Fatal(err)
	}
	b.Time = "2026-01-05T01:31:00Z"
	if _, err := NormalizeBars([]Bar{b}, q); !errors.Is(err, ErrPayload) {
		t.Fatalf("accepted non-local timestamp: %v", err)
	}
}

type catalogOnly struct{}

func (*catalogOnly) ID() string                                           { return "catalog-only" }
func (*catalogOnly) BoardKinds() []string                                 { return []string{"concept"} }
func (*catalogOnly) Boards(context.Context, string) (BoardsResult, error) { return BoardsResult{}, nil }
func (*catalogOnly) ResolveBoard(context.Context, string, string) (BoardResult, error) {
	return BoardResult{}, nil
}

func TestRegistryOptionalCapabilitiesAndIdentity(t *testing.T) {
	r, err := NewRegistry(&catalogOnly{})
	if err != nil {
		t.Fatal(err)
	}
	if infos := r.Providers(); len(infos) != 1 || len(infos[0].Bars) != 0 || len(infos[0].BoardKinds) != 1 {
		t.Fatalf("%+v", infos)
	}
	if _, err := r.Bars(context.Background(), "catalog-only", testQuery()); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := r.Boards(context.Background(), "eastmoney", "concept"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := NewRegistry(&catalogOnly{}, &catalogOnly{}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	var nilProvider *catalogOnly
	if _, err := NewRegistry(nilProvider); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}
