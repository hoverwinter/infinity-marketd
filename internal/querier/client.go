package querier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type HTTPClient struct {
	baseURL string
	client  *http.Client
}

func NewHTTPClient(baseURL string, client *http.Client) *HTTPClient {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPClient{baseURL: strings.TrimRight(baseURL, "/"), client: client}
}

func (c *HTTPClient) Health(ctx context.Context) (Health, error) {
	var health Health
	if err := c.getJSON(ctx, "/api/v1/health", nil, &health); err != nil {
		return health, err
	}
	return health, nil
}

func (c *HTTPClient) Bars(ctx context.Context, query BarQuery) (BarResult, error) {
	values := url.Values{}
	values.Set("market", query.Market)
	values.Set("symbol", query.Symbol)
	values.Set("period", query.Period)
	if query.Adjust != "" {
		values.Set("adjust", query.Adjust)
	}
	if query.Since != "" {
		values.Set("since", query.Since)
	}
	if query.Until != "" {
		values.Set("until", query.Until)
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	var result BarResult
	if err := c.getJSON(ctx, "/api/v1/bars", values, &result); err != nil {
		return result, err
	}
	return result, nil
}

func (c *HTTPClient) IntradayPoints(ctx context.Context, query IntradayPointQuery) (IntradayPointResult, error) {
	values := url.Values{}
	values.Set("market", query.Market)
	values.Set("symbol", query.Symbol)
	if query.Date != "" {
		values.Set("date", query.Date)
	}
	if query.Since != "" {
		values.Set("since", query.Since)
	}
	if query.Until != "" {
		values.Set("until", query.Until)
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	var result IntradayPointResult
	if err := c.getJSON(ctx, "/api/v1/intraday-points", values, &result); err != nil {
		return result, err
	}
	return result, nil
}

func (c *HTTPClient) ResolveSymbol(ctx context.Context, symbol string) (SymbolResolution, error) {
	values := url.Values{}
	values.Set("symbol", symbol)
	var result SymbolResolution
	if err := c.getJSON(ctx, "/api/v1/resolve-symbol", values, &result); err != nil {
		return result, err
	}
	return result, nil
}

func (c *HTTPClient) getJSON(ctx context.Context, path string, values url.Values, target any) error {
	endpoint := c.baseURL + path
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var payload struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&payload)
		if payload.Error == "" {
			payload.Error = resp.Status
		}
		return fmt.Errorf("%s", payload.Error)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}
