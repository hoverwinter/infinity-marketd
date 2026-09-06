package eastmoney

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/marketdata"
)

const klineLimit = 1000 // larger than every requested yearly daily window

var _ marketdata.BarsProvider = (*Client)(nil)

func (*Client) BarsCapabilities() []marketdata.BarsCapability {
	return []marketdata.BarsCapability{{Kind: "index", Markets: []string{"board"}, Periods: []string{"1d"}}}
}

type historyData struct {
	Code   string    `json:"code"`
	Market *int      `json:"market"`
	Klines *[]string `json:"klines"`
}

func (c *Client) Bars(ctx context.Context, query marketdata.BarsQuery) (marketdata.BarsResult, error) {
	q, err := marketdata.NormalizeBarsQuery(query, c.now())
	if err != nil {
		return marketdata.BarsResult{}, err
	}
	if q.Instrument.Kind != "index" || q.Instrument.Market != "board" || q.Period != "1d" {
		return marketdata.BarsResult{}, fmt.Errorf("%w: Eastmoney supports board index daily bars", marketdata.ErrUnsupported)
	}
	if !boardCode.MatchString(q.Instrument.Symbol) {
		return marketdata.BarsResult{}, fmt.Errorf("%w: Eastmoney requires a BK quotation symbol", marketdata.ErrInvalid)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	since, _ := time.Parse("2006-01-02", q.Since)
	until, _ := time.Parse("2006-01-02", q.Until)
	params := url.Values{"secid": {"90." + q.Instrument.Symbol}, "fields1": {"f1,f2,f3,f4,f5,f6"}, "fields2": {"f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61"}, "klt": {"101"}, "fqt": {"0"}, "ut": {"7eea3edcaed734bea9cbfc24409ed989"}, "lmt": {strconv.Itoa(klineLimit)}}
	rows := []marketdata.Bar{}
	for start := since; !start.After(until); {
		end := time.Date(start.Year(), 12, 31, 0, 0, 0, 0, time.UTC)
		if end.After(until) {
			end = until
		}
		params.Set("beg", start.Format("20060102"))
		params.Set("end", end.Format("20060102"))
		var data historyData
		if err := c.get(ctx, c.historyURL, "/api/qt/stock/kline/get", params, &data); err != nil {
			return marketdata.BarsResult{}, err
		}
		chunk, err := parseHistory(data, q.Instrument.Symbol, start, end)
		if err != nil {
			return marketdata.BarsResult{}, err
		}
		rows = append(rows, chunk...)
		start = end.AddDate(0, 0, 1)
	}
	result, err := marketdata.NewBarsResult(c.ID(), q, "provider_native", rows)
	if err != nil {
		return marketdata.BarsResult{}, err
	}
	result.Warnings = append(result.Warnings, "Eastmoney volume is returned unchanged; its unit has not been independently verified. Successful date windows do not establish complete historical coverage.")
	return result, nil
}

func parseHistory(data historyData, symbol string, since, until time.Time) ([]marketdata.Bar, error) {
	if data.Code != symbol || data.Market == nil || *data.Market != 90 || data.Klines == nil {
		return nil, fmt.Errorf("%w: Eastmoney history identity or klines array missing/mismatched", marketdata.ErrPayload)
	}
	if len(*data.Klines) >= klineLimit {
		return nil, fmt.Errorf("%w: Eastmoney daily response reached its row limit", marketdata.ErrLimit)
	}
	rows := make([]marketdata.Bar, 0, len(*data.Klines))
	for i, line := range *data.Klines {
		fields := strings.Split(line, ",")
		if len(fields) != 11 {
			return nil, fmt.Errorf("%w: Eastmoney row %d has %d fields", marketdata.ErrPayload, i+1, len(fields))
		}
		date, err := time.Parse("2006-01-02", fields[0])
		if err != nil || date.Before(since) || date.After(until) {
			return nil, fmt.Errorf("%w: Eastmoney row date outside requested daily window", marketdata.ErrPayload)
		}
		bar := marketdata.Bar{Time: fields[0]}
		// Eastmoney's order is open, CLOSE, high, low; unlike THS/TDX.
		for j, dest := range []*float64{&bar.Open, &bar.Close, &bar.High, &bar.Low, &bar.Volume, &bar.Amount} {
			*dest, err = strconv.ParseFloat(fields[j+1], 64)
			if err != nil {
				return nil, fmt.Errorf("%w: Eastmoney row %d field %d is not numeric", marketdata.ErrPayload, i+1, j+1)
			}
		}
		rows = append(rows, bar)
	}
	return rows, nil
}
