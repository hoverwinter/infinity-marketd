package securitymaster

import (
	"context"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

type TDXSecurityFetcher func(context.Context, string, tdx.QuoteClientOptions) ([]tdx.Security, error)

type TDXSource struct {
	Fetcher TDXSecurityFetcher
	Options tdx.QuoteClientOptions
}

func NewTDXSource(fetcher TDXSecurityFetcher, opts tdx.QuoteClientOptions) TDXSource {
	if fetcher == nil {
		fetcher = tdx.FetchSecurityList
	}
	return TDXSource{Fetcher: fetcher, Options: opts}
}

func (s TDXSource) Fetch(ctx context.Context, markets []string) ([]SourceRow, error) {
	var rows []SourceRow
	for _, market := range markets {
		securities, err := s.Fetcher(ctx, market, s.Options)
		if err != nil {
			return nil, err
		}
		for _, security := range securities {
			rows = append(rows, SourceRow{
				Market:         security.Market,
				Symbol:         security.Symbol,
				Name:           security.Name,
				LotSize:        int(security.VolUnit),
				PricePrecision: int(security.DecimalPoint),
				Status:         StatusListed,
				Source:         SourceTDX,
			})
		}
	}
	return rows, nil
}
