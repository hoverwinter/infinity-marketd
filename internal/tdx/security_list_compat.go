package tdx

import (
	"context"
	"fmt"
	"strings"
)

const bjQuotesListPageSize = 80

// FetchSecurityListWithNames keeps the classic security-list protocol for SH/SZ,
// but uses the MAC quote protocol for BJ names. Public HQ servers currently
// answer BJ realtime quotes and category lists, while the legacy 0x0450 BJ list
// often times out.
func FetchSecurityListWithNames(ctx context.Context, market string, opts QuoteClientOptions) ([]Security, error) {
	market = strings.ToLower(strings.TrimSpace(market))
	if market != "bj" {
		return FetchSecurityList(ctx, market, opts)
	}
	codes, preClose, err := fetchBJQuoteListCodes(ctx, opts)
	if err != nil {
		return nil, err
	}
	if len(codes) == 0 {
		return nil, fmt.Errorf("TDX BJ quotes list returned no securities")
	}
	requests := make([]MACSymbolQuoteRequest, 0, len(codes))
	for _, code := range codes {
		requests = append(requests, MACSymbolQuoteRequest{Market: "bj", Symbol: code})
	}
	quotes, err := FetchMACSymbolQuotes(ctx, requests, MACClientOptions{Servers: opts.MACServers, Timeout: opts.Timeout})
	if err != nil {
		return nil, err
	}
	bySymbol := make(map[string]MACSymbolQuote, len(quotes))
	for _, quote := range quotes {
		if quote.Market == "bj" && quote.Symbol != "" && quote.Name != "" {
			bySymbol[quote.Symbol] = quote
		}
	}
	securities := make([]Security, 0, len(codes))
	for _, code := range codes {
		quote, ok := bySymbol[code]
		if !ok {
			continue
		}
		securities = append(securities, Security{
			Market:       "bj",
			Symbol:       code,
			Name:         quote.Name,
			VolUnit:      100,
			DecimalPoint: 2,
			PreClose:     preClose[code],
		})
	}
	if len(securities) == 0 {
		return nil, fmt.Errorf("TDX BJ MAC quotes returned no named securities")
	}
	return securities, nil
}

func fetchBJQuoteListCodes(ctx context.Context, opts QuoteClientOptions) ([]string, map[string]float64, error) {
	seen := make(map[string]struct{})
	var codes []string
	preClose := make(map[string]float64)
	for start := 0; ; {
		page, err := FetchHQQuotesList(ctx, HQQuotesListRequest{
			Category: 12,
			SortType: QuotesSortCode,
			Start:    start,
			Count:    bjQuotesListPageSize,
		}, opts)
		if err != nil {
			return nil, nil, err
		}
		for _, item := range page {
			if item.Market != "bj" || len(item.Symbol) != 6 || !allDigits(item.Symbol) {
				continue
			}
			if _, ok := seen[item.Symbol]; ok {
				continue
			}
			seen[item.Symbol] = struct{}{}
			codes = append(codes, item.Symbol)
			preClose[item.Symbol] = item.PreClose
		}
		if len(page) < bjQuotesListPageSize {
			break
		}
		start += len(page)
	}
	return codes, preClose, nil
}
