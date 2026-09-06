package querier

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hoverwinter/infinity-marketd/internal/marketdata"
	"github.com/hoverwinter/infinity-marketd/internal/securitymaster"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

type Server struct {
	repo                   Repository
	securitiesRepo         securitymaster.Reader
	tdxProvider            *TDXProvider
	marketDataProviders    *marketdata.Registry
	consoleHQDailyImporter ConsoleHQDailyImporter
	limitCorrectionHandler http.HandlerFunc
}

func NewServer(repo Repository) *Server {
	return NewServerWithSecurities(repo, nil)
}

func NewServerWithSecurities(repo Repository, securitiesRepo securitymaster.Reader) *Server {
	return NewServerWithSecuritiesAndTDXProvider(repo, securitiesRepo, DefaultTDXProvider())
}

func NewServerWithTDXProvider(repo Repository, provider *TDXProvider) *Server {
	return NewServerWithSecuritiesAndTDXProvider(repo, nil, provider)
}

func NewServerWithSecuritiesAndTDXProvider(repo Repository, securitiesRepo securitymaster.Reader, provider *TDXProvider) *Server {
	if provider == nil {
		provider = DefaultTDXProvider()
	}
	if securitiesRepo == nil {
		securitiesRepo = securitymaster.NewUnavailableReader(fmt.Errorf("mysql is not configured"))
	}
	return &Server{repo: repo, securitiesRepo: securitiesRepo, tdxProvider: provider, marketDataProviders: defaultMarketDataProviders(provider, "")}
}

func (s *Server) WithConsoleHQDailyImporter(importer ConsoleHQDailyImporter) *Server {
	s.consoleHQDailyImporter = importer
	return s
}

func (s *Server) Handler() http.Handler {
	return s.HandlerWithConsoleDist("")
}

func (s *Server) HandlerWithConsoleDist(consoleDist string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("GET /api/v1/bars", s.handleBars)
	mux.HandleFunc("GET /api/v1/intraday-points", s.handleIntradayPoints)
	mux.HandleFunc("GET /api/v1/resolve-symbol", s.handleResolveSymbol)
	mux.HandleFunc("GET /api/v1/securities", s.handleSecurity)
	mux.HandleFunc("GET /api/v1/securities/resolve", s.handleSecurityResolve)
	s.registerTDXRoutes(mux)
	s.registerMarketDataRoutes(mux)
	s.registerLimitReviewRoutes(mux)
	s.registerConsoleRoutes(mux)
	if s.limitCorrectionHandler != nil {
		mux.HandleFunc("POST /api/console/imports/limit-review-corrections", s.limitCorrectionHandler)
	}
	if strings.TrimSpace(consoleDist) != "" {
		registerConsoleStaticRoutes(mux, consoleDist)
	}
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.Health(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	writeJSON(w, http.StatusOK, Health{Status: "ok", Version: Version, SchemaVersion: SchemaVersion})
}

func (s *Server) handleBars(w http.ResponseWriter, r *http.Request) {
	query := barQueryFromRequest(r)
	result, err := s.repo.Bars(r.Context(), query)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleIntradayPoints(w http.ResponseWriter, r *http.Request) {
	query := intradayPointQueryFromRequest(r)
	result, err := s.repo.IntradayPoints(r.Context(), query)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleResolveSymbol(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimSpace(r.URL.Query().Get("symbol"))
	if !symbolPattern.MatchString(symbol) {
		writeError(w, http.StatusBadRequest, validationError("symbol must be six digits"))
		return
	}
	writeJSON(w, http.StatusOK, SymbolResolution{Symbol: symbol, Market: tdx.InferMarketFromCode(symbol)})
}

func (s *Server) handleSecurity(w http.ResponseWriter, r *http.Request) {
	market := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("market")))
	symbol := strings.TrimSpace(r.URL.Query().Get("symbol"))
	if !marketPattern.MatchString(market) {
		writeError(w, http.StatusBadRequest, validationError("market must be sh, sz, or bj"))
		return
	}
	if !symbolPattern.MatchString(symbol) {
		writeError(w, http.StatusBadRequest, validationError("symbol must be six digits"))
		return
	}
	security, err := s.securitiesRepo.Security(r.Context(), market, symbol)
	if err != nil {
		writeError(w, securityStatusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, security)
}

func (s *Server) handleSecurityResolve(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, validationError("q is required"))
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	candidates, err := s.securitiesRepo.Resolve(r.Context(), q, limit)
	if err != nil {
		writeError(w, securityStatusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"q":          q,
		"candidates": candidates,
	})
}

func intradayPointQueryFromRequest(r *http.Request) IntradayPointQuery {
	values := r.URL.Query()
	limit, _ := strconv.Atoi(values.Get("limit"))
	return IntradayPointQuery{
		Market: strings.TrimSpace(values.Get("market")),
		Symbol: strings.TrimSpace(values.Get("symbol")),
		Date:   strings.TrimSpace(values.Get("date")),
		Since:  strings.TrimSpace(values.Get("since")),
		Until:  strings.TrimSpace(values.Get("until")),
		Limit:  limit,
	}
}

func barQueryFromRequest(r *http.Request) BarQuery {
	values := r.URL.Query()
	limit, _ := strconv.Atoi(values.Get("limit"))
	return BarQuery{
		Market: strings.TrimSpace(values.Get("market")),
		Symbol: strings.TrimSpace(values.Get("symbol")),
		Period: strings.TrimSpace(values.Get("period")),
		Adjust: strings.TrimSpace(values.Get("adjust")),
		Since:  strings.TrimSpace(values.Get("since")),
		Until:  strings.TrimSpace(values.Get("until")),
		Limit:  limit,
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func statusForError(err error) int {
	if IsValidationError(err) {
		return http.StatusBadRequest
	}
	if IsMissingAdjustmentFactorError(err) {
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}

func securityStatusForError(err error) int {
	if errors.Is(err, securitymaster.ErrNotFound) {
		return http.StatusNotFound
	}
	var unavailable securitymaster.UnavailableError
	if errors.As(err, &unavailable) {
		return http.StatusServiceUnavailable
	}
	if IsValidationError(err) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

type MissingAdjustmentFactorError struct {
	Message string
}

func (e MissingAdjustmentFactorError) Error() string {
	return e.Message
}

func IsMissingAdjustmentFactorError(err error) bool {
	var missingErr MissingAdjustmentFactorError
	return errors.As(err, &missingErr)
}

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

func IsValidationError(err error) bool {
	var validationErr ValidationError
	return errors.As(err, &validationErr)
}

func validationError(format string, args ...any) error {
	return ValidationError{Message: fmt.Sprintf(format, args...)}
}
