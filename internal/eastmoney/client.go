// Package eastmoney adapts Eastmoney online data to the common marketdata products.
package eastmoney

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/marketdata"
)

const maxResponseBytes = 4 << 20

type Options struct {
	Client *http.Client
	// Operator/test configuration only; never taken from public query parameters.
	QuoteURL   string
	HistoryURL string
	// Zero uses 300ms; a negative value disables spacing for local fixtures.
	RequestInterval time.Duration
	Now             func() time.Time
}

type Client struct {
	http                 *http.Client
	quoteURL, historyURL string
	interval             time.Duration
	now                  func() time.Time
	gate                 chan struct{}
	lastRequest          time.Time // protected by gate
}

func NewClient(opts Options) *Client {
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 10 * time.Second}
	}
	if opts.QuoteURL == "" {
		opts.QuoteURL = "https://17.push2.eastmoney.com"
	}
	if opts.HistoryURL == "" {
		opts.HistoryURL = "https://push2his.eastmoney.com"
	}
	if opts.RequestInterval == 0 {
		opts.RequestInterval = 300 * time.Millisecond
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Client{http: opts.Client, quoteURL: strings.TrimRight(opts.QuoteURL, "/"), historyURL: strings.TrimRight(opts.HistoryURL, "/"), interval: opts.RequestInterval, now: opts.Now, gate: make(chan struct{}, 1)}
}

func (*Client) ID() string { return "eastmoney" }

func (c *Client) get(ctx context.Context, base, path string, params url.Values, out any) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	select {
	case c.gate <- struct{}{}:
		defer func() { <-c.gate }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if wait := c.interval - time.Since(c.lastRequest); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("%w: Eastmoney request configuration", marketdata.ErrUpstream)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")
	c.lastRequest = time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return context.DeadlineExceeded
		}
		if errors.Is(err, context.Canceled) {
			return context.Canceled
		}
		return fmt.Errorf("%w: Eastmoney GET %s transport failed", marketdata.ErrUpstream, path)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: Eastmoney GET %s HTTP %d", marketdata.ErrUpstream, path, resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: Eastmoney response read failed", marketdata.ErrUpstream)
	}
	if len(raw) > maxResponseBytes {
		return fmt.Errorf("%w: Eastmoney response exceeds 4 MiB", marketdata.ErrPayload)
	}
	var envelope struct {
		RC   *int            `json:"rc"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.RC == nil {
		return fmt.Errorf("%w: invalid Eastmoney JSON envelope", marketdata.ErrPayload)
	}
	if *envelope.RC != 0 {
		return fmt.Errorf("%w: Eastmoney rc=%d", marketdata.ErrUpstream, *envelope.RC)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("%w: Eastmoney returned no data; empty history is not established", marketdata.ErrUpstream)
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("%w: invalid Eastmoney data structure", marketdata.ErrPayload)
	}
	return nil
}
