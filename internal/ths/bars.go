package ths

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/marketdata"
)

var _ marketdata.BarsProvider = (*Client)(nil)
var _ marketdata.BoardProvider = (*Client)(nil)

func (*Client) BarsCapabilities() []marketdata.BarsCapability {
	return []marketdata.BarsCapability{{Kind: "index", Markets: []string{"board"}, Periods: []string{"1d"}}}
}

func (c *Client) Bars(ctx context.Context, query marketdata.BarsQuery) (marketdata.BarsResult, error) {
	q, err := marketdata.NormalizeBarsQuery(query, c.now())
	if err != nil {
		return marketdata.BarsResult{}, err
	}
	if q.Instrument.Kind != "index" || q.Instrument.Market != "board" || q.Period != "1d" {
		return marketdata.BarsResult{}, fmt.Errorf("%w: THS supports board index daily bars only", marketdata.ErrUnsupported)
	}
	if !sixDigits.MatchString(q.Instrument.Symbol) || !strings.HasPrefix(q.Instrument.Symbol, "88") {
		return marketdata.BarsResult{}, fmt.Errorf("%w: THS requires an 88xxxx quotation symbol; resolve concept page codes first", marketdata.ErrInvalid)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	since, _ := time.Parse("2006-01-02", q.Since)
	until, _ := time.Parse("2006-01-02", q.Until)
	rows := []marketdata.Bar{}
	for year := since.Year(); year <= until.Year(); year++ {
		path := fmt.Sprintf("/v4/line/bk_%s/01/%d.js", q.Instrument.Symbol, year)
		raw, err := c.get(ctx, c.chartURL, path)
		if err != nil {
			return marketdata.BarsResult{}, err
		}
		bars, err := parseYear(raw, q.Instrument.Symbol, year)
		if err != nil {
			return marketdata.BarsResult{}, fmt.Errorf("THS year %d: %w", year, err)
		}
		rows = append(rows, bars...)
	}
	// AKShare returns the volume field unchanged without proving its unit. Keep
	// it explicit as native until a source contract establishes a conversion.
	result, err := marketdata.NewBarsResult(c.ID(), q, "provider_native", rows)
	if err != nil {
		return marketdata.BarsResult{}, err
	}
	result.Warnings = append(result.Warnings, "THS volume is returned unchanged; its unit has not been independently verified. Annual files do not prove complete historical coverage.")
	return result, nil
}

func parseYear(raw []byte, symbol string, year int) ([]marketdata.Bar, error) {
	text := strings.TrimSpace(string(raw))
	prefix := fmt.Sprintf("quotebridge_v4_line_bk_%s_01_%d(", symbol, year)
	text = strings.TrimSuffix(text, ";")
	if !strings.HasPrefix(text, prefix) || !strings.HasSuffix(text, ")") {
		return nil, fmt.Errorf("%w: unexpected THS JSONP envelope (possibly a challenge page)", marketdata.ErrPayload)
	}
	var payload struct {
		Data *string `json:"data"`
	}
	if err := json.Unmarshal([]byte(text[len(prefix):len(text)-1]), &payload); err != nil || payload.Data == nil {
		return nil, fmt.Errorf("%w: missing or malformed THS data field", marketdata.ErrPayload)
	}
	rows := []marketdata.Bar{}
	if *payload.Data == "" {
		return rows, nil
	}
	for i, row := range strings.Split(*payload.Data, ";") {
		fields := strings.Split(row, ",")
		if len(fields) != 11 && len(fields) != 12 {
			return nil, fmt.Errorf("%w: THS row %d has %d fields", marketdata.ErrPayload, i+1, len(fields))
		}
		date, err := time.Parse("20060102", fields[0])
		if err != nil || date.Year() != year {
			return nil, fmt.Errorf("%w: THS row %d has invalid year/date", marketdata.ErrPayload, i+1)
		}
		bar := marketdata.Bar{Time: date.Format("2006-01-02")}
		for j, dest := range []*float64{&bar.Open, &bar.High, &bar.Low, &bar.Close, &bar.Volume, &bar.Amount} {
			*dest, err = strconv.ParseFloat(fields[j+1], 64)
			if err != nil {
				return nil, fmt.Errorf("%w: THS row %d field %d is not numeric", marketdata.ErrPayload, i+1, j+1)
			}
		}
		rows = append(rows, bar)
	}
	return rows, nil
}
