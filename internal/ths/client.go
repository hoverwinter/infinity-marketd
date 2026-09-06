// Package ths owns Tonghuashun HTTP protocols and parsing. It never writes facts.
package ths

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hoverwinter/infinity-marketd/internal/marketdata"
	"golang.org/x/text/encoding/simplifiedchinese"
)

const maxResponseBytes = 4 << 20

type Options struct {
	Client *http.Client
	// URLs are configured by the operator/composition root, never HTTP query input.
	PageURL  string
	ChartURL string
	Cookie   string
	// Zero selects 300ms spacing. Negative disables spacing for local fixtures.
	RequestInterval time.Duration
	Now             func() time.Time
}

type Client struct {
	http                      *http.Client
	pageURL, chartURL, cookie string
	interval                  time.Duration
	now                       func() time.Time
	gate                      chan struct{}
	lastRequest               time.Time // protected by gate
}

func NewClient(opts Options) *Client {
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 10 * time.Second}
	}
	if opts.PageURL == "" {
		opts.PageURL = "https://q.10jqka.com.cn"
	}
	if opts.ChartURL == "" {
		opts.ChartURL = "https://d.10jqka.com.cn"
	}
	if opts.RequestInterval == 0 {
		opts.RequestInterval = 300 * time.Millisecond
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Client{http: opts.Client, pageURL: strings.TrimRight(opts.PageURL, "/"), chartURL: strings.TrimRight(opts.ChartURL, "/"), cookie: opts.Cookie, interval: opts.RequestInterval, now: opts.Now, gate: make(chan struct{}, 1)}
}

func (*Client) ID() string { return "ths" }

func (c *Client) get(ctx context.Context, base, path string) ([]byte, error) {
	// The deadline includes queueing as well as transport; cancellation never waits
	// indefinitely for another caller's request or for a rate-limit interval.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	select {
	case c.gate <- struct{}{}:
		defer func() { <-c.gate }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if wait := c.interval - time.Since(c.lastRequest); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: THS request configuration", marketdata.ErrUpstream)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", c.pageURL+"/")
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}
	c.lastRequest = time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, context.DeadlineExceeded
		}
		if errors.Is(err, context.Canceled) {
			return nil, context.Canceled
		}
		// Do not expose transport errors containing request headers/credentials.
		return nil, fmt.Errorf("%w: THS GET %s transport failed", marketdata.ErrUpstream, path)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: THS GET %s HTTP %d", marketdata.ErrUpstream, path, resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: THS response read failed", marketdata.ErrUpstream)
	}
	if len(raw) > maxResponseBytes {
		return nil, fmt.Errorf("%w: THS response exceeds 4 MiB", marketdata.ErrPayload)
	}
	return raw, nil
}

func (c *Client) page(ctx context.Context, path string) (string, error) {
	raw, err := c.get(ctx, c.pageURL, path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(raw) {
		raw, err = simplifiedchinese.GBK.NewDecoder().Bytes(raw)
		if err != nil {
			return "", fmt.Errorf("%w: invalid THS GBK page", marketdata.ErrPayload)
		}
	}
	if strings.ContainsRune(string(raw), '\uFFFD') {
		return "", fmt.Errorf("%w: invalid THS page encoding", marketdata.ErrPayload)
	}
	return string(raw), nil
}
