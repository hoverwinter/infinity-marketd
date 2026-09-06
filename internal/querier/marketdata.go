package querier

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/hoverwinter/infinity-marketd/internal/eastmoney"
	"github.com/hoverwinter/infinity-marketd/internal/marketdata"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
	"github.com/hoverwinter/infinity-marketd/internal/ths"
)

func defaultMarketDataProviders(provider *TDXProvider, thsCookie string) *marketdata.Registry {
	r, err := marketdata.NewRegistry(
		tdx.NewMarketDataProvider(tdx.MarketDataOptions{
			// Closures preserve the existing TDX dependency-injection seam.
			FetchSecurityBars: func(ctx context.Context, req tdx.HQBarsRequest, opts tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
				return provider.FetchHQSecurityBars(ctx, req, opts)
			},
			FetchIndexBars: func(ctx context.Context, req tdx.HQBarsRequest, opts tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
				return provider.FetchHQIndexBars(ctx, req, opts)
			},
		}),
		ths.NewClient(ths.Options{Cookie: thsCookie}),
		eastmoney.NewClient(eastmoney.Options{}),
	)
	if err != nil {
		panic(err)
	} // statically known non-nil providers with unique IDs
	return r
}

// Configure before Handler is used; serving never mutates the registry.
func (s *Server) WithMarketDataProviders(registry *marketdata.Registry) *Server {
	if registry != nil {
		s.marketDataProviders = registry
	}
	return s
}

func (s *Server) WithTHSCookie(cookie string) *Server {
	registry, err := s.marketDataProviders.WithProvider(ths.NewClient(ths.Options{Cookie: cookie}))
	if err != nil {
		panic(err)
	} // a statically valid THS provider
	s.marketDataProviders = registry
	return s
}

func (s *Server) registerMarketDataRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/providers", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.marketDataProviders.Providers())
	})
	mux.HandleFunc("GET /api/providers/{provider}/bars", s.handleProviderBars)
	mux.HandleFunc("GET /api/providers/{provider}/boards", s.handleProviderBoards)
	mux.HandleFunc("GET /api/providers/{provider}/boards/{kind}/{code}", s.handleProviderBoard)
}

func validateProviderParams(r *http.Request, allowed ...string) error {
	for key, values := range r.URL.Query() {
		ok := false
		for _, a := range allowed {
			if key == a {
				ok = true
				break
			}
		}
		if !ok || len(values) != 1 {
			return validationError("unknown or repeated provider query parameter %q", key)
		}
	}
	return nil
}

func (s *Server) handleProviderBars(w http.ResponseWriter, r *http.Request) {
	if err := validateProviderParams(r, "kind", "market", "symbol", "period", "adjust", "since", "until"); err != nil {
		writeError(w, 400, err)
		return
	}
	v := r.URL.Query()
	q := marketdata.BarsQuery{Instrument: marketdata.Instrument{Kind: v.Get("kind"), Market: v.Get("market"), Symbol: v.Get("symbol")}, Period: v.Get("period"), Adjust: v.Get("adjust"), Since: v.Get("since"), Until: v.Get("until")}
	result, err := s.marketDataProviders.Bars(r.Context(), r.PathValue("provider"), q)
	writeMarketDataResult(w, result, err)
}

func (s *Server) handleProviderBoards(w http.ResponseWriter, r *http.Request) {
	if err := validateProviderParams(r, "kind"); err != nil {
		writeError(w, 400, err)
		return
	}
	result, err := s.marketDataProviders.Boards(r.Context(), r.PathValue("provider"), r.URL.Query().Get("kind"))
	writeMarketDataResult(w, result, err)
}

func (s *Server) handleProviderBoard(w http.ResponseWriter, r *http.Request) {
	if err := validateProviderParams(r); err != nil {
		writeError(w, 400, err)
		return
	}
	result, err := s.marketDataProviders.ResolveBoard(r.Context(), r.PathValue("provider"), r.PathValue("kind"), r.PathValue("code"))
	writeMarketDataResult(w, result, err)
}

func writeMarketDataResult(w http.ResponseWriter, value any, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, value)
		return
	}
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, marketdata.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, marketdata.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, marketdata.ErrUnsupported), errors.Is(err, marketdata.ErrLimit):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, marketdata.ErrPayload):
		status = http.StatusBadGateway
	case errors.Is(err, context.DeadlineExceeded):
		status = http.StatusGatewayTimeout
	case errors.Is(err, marketdata.ErrUpstream), errors.Is(err, context.Canceled):
		status = http.StatusServiceUnavailable
	}
	writeError(w, status, err)
}

func (c *HTTPClient) Providers(ctx context.Context) ([]marketdata.ProviderInfo, error) {
	var result []marketdata.ProviderInfo
	err := c.getJSON(ctx, "/api/providers", nil, &result)
	return result, err
}

func providerPath(provider string) string {
	return "/api/providers/" + url.PathEscape(strings.TrimSpace(provider))
}

func (c *HTTPClient) ProviderBars(ctx context.Context, provider string, q marketdata.BarsQuery) (marketdata.BarsResult, error) {
	v := url.Values{"kind": {q.Instrument.Kind}, "market": {q.Instrument.Market}, "symbol": {q.Instrument.Symbol}, "period": {q.Period}, "adjust": {q.Adjust}, "since": {q.Since}, "until": {q.Until}}
	var result marketdata.BarsResult
	err := c.getJSON(ctx, providerPath(provider)+"/bars", v, &result)
	return result, err
}

func (c *HTTPClient) ProviderBoards(ctx context.Context, provider, kind string) (marketdata.BoardsResult, error) {
	var result marketdata.BoardsResult
	err := c.getJSON(ctx, providerPath(provider)+"/boards", url.Values{"kind": {kind}}, &result)
	return result, err
}

func (c *HTTPClient) ProviderBoard(ctx context.Context, provider, kind, code string) (marketdata.BoardResult, error) {
	var result marketdata.BoardResult
	err := c.getJSON(ctx, providerPath(provider)+"/boards/"+url.PathEscape(kind)+"/"+url.PathEscape(code), nil, &result)
	return result, err
}
