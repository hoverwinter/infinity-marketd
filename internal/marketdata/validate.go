package marketdata

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalid     = errors.New("invalid market data request")
	ErrNotFound    = errors.New("provider or instrument not found")
	ErrUnsupported = errors.New("unsupported market data capability")
	ErrUpstream    = errors.New("market data upstream unavailable")
	ErrPayload     = errors.New("invalid market data upstream payload")
	ErrLimit       = errors.New("market data scan limit exceeded")
	identifier     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,31}$`)
)

// NormalizeBarsQuery validates shared semantics; sources validate their own symbols
// and capability combinations before doing I/O. Date bounds include whole days.
func NormalizeBarsQuery(q BarsQuery, now time.Time) (BarsQuery, error) {
	q.Instrument.Kind = strings.ToLower(strings.TrimSpace(q.Instrument.Kind))
	q.Instrument.Market = strings.ToLower(strings.TrimSpace(q.Instrument.Market))
	q.Instrument.Symbol = strings.TrimSpace(q.Instrument.Symbol)
	q.Period = strings.TrimSpace(q.Period)
	q.Adjust = strings.TrimSpace(q.Adjust)
	q.Since, q.Until = strings.TrimSpace(q.Since), strings.TrimSpace(q.Until)
	if q.Instrument.Kind != "index" && q.Instrument.Kind != "security" {
		return q, fmt.Errorf("%w: kind must be index or security", ErrInvalid)
	}
	if !identifier.MatchString(q.Instrument.Market) || !identifier.MatchString(q.Instrument.Symbol) {
		return q, fmt.Errorf("%w: explicit market and symbol are required (1-32 identifier characters)", ErrInvalid)
	}
	if q.Period == "" {
		q.Period = "1d"
	}
	if q.Adjust == "" {
		q.Adjust = "none"
	}
	if q.Adjust != "none" {
		return q, fmt.Errorf("%w: online bars currently support adjust=none only", ErrUnsupported)
	}
	since, e1 := time.Parse("2006-01-02", q.Since)
	until, e2 := time.Parse("2006-01-02", q.Until)
	if e1 != nil || e2 != nil || since.After(until) {
		return q, fmt.Errorf("%w: since and until must be YYYY-MM-DD with since <= until", ErrInvalid)
	}
	loc, err := time.LoadLocation(Timezone)
	if err != nil {
		return q, err
	}
	if since.Year() < 1990 || until.After(since.AddDate(10, 0, 0)) || q.Until > now.In(loc).Format("2006-01-02") {
		return q, fmt.Errorf("%w: dates must start in 1990 or later, span at most ten years and not be in the future", ErrInvalid)
	}
	return q, nil
}

// NormalizeBars validates all fetched rows, including rows outside the requested
// range, before filtering. A partial/malformed page is never silently accepted.
func NormalizeBars(rows []Bar, q BarsQuery) ([]Bar, error) {
	seen := make(map[string]Bar, len(rows))
	loc, err := time.LoadLocation(Timezone)
	if err != nil {
		return nil, err
	}
	for _, b := range rows {
		layout := "2006-01-02"
		if q.Period != "1d" {
			layout = time.RFC3339
		}
		t, err := time.ParseInLocation(layout, b.Time, loc)
		if err != nil || t.In(loc).Format(layout) != b.Time {
			return nil, fmt.Errorf("%w: invalid bar timestamp %q", ErrPayload, b.Time)
		}
		for _, n := range []float64{b.Open, b.High, b.Low, b.Close, b.Volume, b.Amount} {
			if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
				return nil, fmt.Errorf("%w: invalid numeric value at %s", ErrPayload, b.Time)
			}
		}
		if b.High < b.Low || b.High < b.Open || b.High < b.Close || b.Low > b.Open || b.Low > b.Close {
			return nil, fmt.Errorf("%w: invalid OHLC at %s", ErrPayload, b.Time)
		}
		if prior, ok := seen[b.Time]; ok && prior != b {
			return nil, fmt.Errorf("%w: conflicting bars at %s", ErrPayload, b.Time)
		}
		seen[b.Time] = b
	}
	out := make([]Bar, 0, len(seen))
	for _, b := range seen {
		date := b.Time[:10]
		if date >= q.Since && date <= q.Until {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time < out[j].Time })
	return out, nil
}

func NewBarsResult(provider string, q BarsQuery, volumeUnit string, rows []Bar) (BarsResult, error) {
	bars, err := NormalizeBars(rows, q)
	if err != nil {
		return BarsResult{}, err
	}
	return BarsResult{Provider: provider, Query: q, Timezone: Timezone, VolumeUnit: volumeUnit, AmountUnit: "CNY", Bars: bars, Warnings: []string{}}, nil
}
