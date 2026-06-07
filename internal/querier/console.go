package querier

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

func (s *Server) registerConsoleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/console/summary", s.handleConsoleSummary)
	mux.HandleFunc("GET /api/console/watermarks", s.handleConsoleWatermarks)
	mux.HandleFunc("GET /api/console/task-runs", s.handleConsoleTaskRuns)
	mux.HandleFunc("GET /api/console/data-quality-issues", s.handleConsoleDataQualityIssues)
	mux.HandleFunc("GET /api/console/quote-service/runs", s.handleConsoleQuoteServiceRuns)
	mux.HandleFunc("GET /api/console/tdx/hq/probe", s.handleConsoleTDXHQProbe)
	mux.HandleFunc("GET /api/console/tdx/hq/quotes", s.handleConsoleTDXHQQuotes)
	mux.HandleFunc("GET /api/console/bestip", s.handleConsoleBestIP)
	mux.HandleFunc("POST /api/console/bestip/refresh", s.handleConsoleBestIPRefresh)
}

func (s *Server) handleConsoleSummary(w http.ResponseWriter, r *http.Request) {
	limit, err := consoleLimitFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	health := Health{Status: "ok", Version: Version, SchemaVersion: SchemaVersion}
	if err := s.repo.Health(r.Context()); err != nil {
		health.Status = "unavailable"
	}
	watermarks, err := s.repo.ConsoleWatermarks(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	taskRuns, err := s.repo.ConsoleTaskRuns(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	issueStats, err := s.repo.ConsoleDataQualityIssueStats(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	quoteRuns, err := s.repo.ConsoleQuoteServiceRuns(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, ConsoleSummary{
		Health:                 health,
		Watermarks:             watermarks,
		TaskRuns:               taskRuns,
		DataQualityIssueCounts: issueStats,
		QuoteServiceRuns:       quoteRuns,
	})
}

func (s *Server) handleConsoleWatermarks(w http.ResponseWriter, r *http.Request) {
	limit, err := consoleLimitFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	items, err := s.repo.ConsoleWatermarks(r.Context(), limit)
	writeConsoleResult(w, items, err)
}

func (s *Server) handleConsoleTaskRuns(w http.ResponseWriter, r *http.Request) {
	limit, err := consoleLimitFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	items, err := s.repo.ConsoleTaskRuns(r.Context(), limit)
	writeConsoleResult(w, items, err)
}

func (s *Server) handleConsoleDataQualityIssues(w http.ResponseWriter, r *http.Request) {
	limit, err := consoleLimitFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	items, err := s.repo.ConsoleDataQualityIssues(r.Context(), limit)
	writeConsoleResult(w, items, err)
}

func (s *Server) handleConsoleQuoteServiceRuns(w http.ResponseWriter, r *http.Request) {
	limit, err := consoleLimitFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	items, err := s.repo.ConsoleQuoteServiceRuns(r.Context(), limit)
	writeConsoleResult(w, items, err)
}

func (s *Server) handleConsoleTDXHQProbe(w http.ResponseWriter, r *http.Request) {
	servers := splitQueryValues(r, "server", "servers")
	if err := validateServerCandidates(servers); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	results := s.tdxProvider.ProbeHQServers(r.Context(), servers, tdx.QuoteClientOptions{})
	tdx.SortProbeResults(results)
	if best := tdx.BestHQServer(results); best != "" {
		for i := range results {
			results[i].Preferred = results[i].Server == best
		}
	}
	writeJSON(w, http.StatusOK, ConsoleProbeResult{Results: nonNilProbeResults(results)})
}

func (s *Server) handleConsoleTDXHQQuotes(w http.ResponseWriter, r *http.Request) {
	symbols := splitQueryValues(r, "symbol", "symbols")
	if len(symbols) == 0 {
		writeError(w, http.StatusBadRequest, validationError("at least one symbol is required"))
		return
	}
	if len(symbols) > maxHTTPQuoteSymbols {
		writeError(w, http.StatusBadRequest, validationError("symbol count must be <= %d", maxHTTPQuoteSymbols))
		return
	}
	requests := make([]tdx.QuoteRequest, 0, len(symbols))
	for _, symbol := range symbols {
		req, err := tdx.ParseQuoteRequest(symbol)
		if err != nil {
			writeError(w, http.StatusBadRequest, validationError("%s", err.Error()))
			return
		}
		requests = append(requests, req)
	}
	opts, err := quoteOptionsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	quotes, err := s.tdxProvider.FetchRealtimeQuotes(r.Context(), requests, opts)
	writeConsoleResult(w, ConsoleQuoteSmokeResult{Quotes: quotes}, err)
}

func (s *Server) handleConsoleBestIP(w http.ResponseWriter, r *http.Request) {
	cachePath := strings.TrimSpace(r.URL.Query().Get("cache"))
	status := consoleBestIPStatus(cachePath, s.tdxProvider.LoadHQBestIPCache)
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleConsoleBestIPRefresh(w http.ResponseWriter, r *http.Request) {
	servers := splitQueryValues(r, "server", "servers")
	if err := validateServerCandidates(servers); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	maxAge, err := bestIPMaxAgeFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cachePath := strings.TrimSpace(r.URL.Query().Get("cache"))
	cache, err := s.tdxProvider.RefreshHQBestIPCache(r.Context(), servers, tdx.QuoteClientOptions{
		BestIPCachePath: cachePath,
		BestIPMaxAge:    maxAge,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	status := bestIPStatusFromCache(cachePath, cache)
	writeJSON(w, http.StatusOK, status)
}

func consoleLimitFromRequest(r *http.Request) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return DefaultConsoleLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0, validationError("limit must be an integer")
	}
	if limit <= 0 {
		return 0, validationError("limit must be positive")
	}
	if limit > MaxConsoleLimit {
		return 0, validationError("limit must be <= %d", MaxConsoleLimit)
	}
	return limit, nil
}

func validateServerCandidates(servers []string) error {
	if len(servers) > 50 {
		return validationError("server candidate count must be <= 50")
	}
	for _, server := range servers {
		if strings.TrimSpace(server) == "" {
			return validationError("server candidate must not be empty")
		}
		if !strings.Contains(server, ":") {
			return validationError("server candidate %q must be host:port", server)
		}
	}
	return nil
}

func bestIPMaxAgeFromRequest(r *http.Request) (time.Duration, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("max-age"))
	if raw == "" {
		return tdx.DefaultHQBestIPMaxAge(), nil
	}
	maxAge, err := time.ParseDuration(raw)
	if err != nil {
		return 0, validationError("max-age must be a Go duration")
	}
	if maxAge <= 0 {
		return 0, validationError("max-age must be positive")
	}
	return maxAge, nil
}

func consoleBestIPStatus(cachePath string, load func(string) (tdx.HQBestIPCache, error)) ConsoleBestIPStatus {
	cache, err := load(cachePath)
	if err != nil {
		if cachePath == "" {
			cachePath = tdx.DefaultHQBestIPCachePath()
		}
		return ConsoleBestIPStatus{CachePath: cachePath, Results: []tdx.ServerProbeResult{}, Error: err.Error()}
	}
	return bestIPStatusFromCache(cachePath, cache)
}

func bestIPStatusFromCache(cachePath string, cache tdx.HQBestIPCache) ConsoleBestIPStatus {
	if cachePath == "" {
		cachePath = tdx.DefaultHQBestIPCachePath()
	}
	generatedAt := cache.GeneratedAt
	expiresAt := cache.ExpiresAt
	status := ConsoleBestIPStatus{
		CachePath: cachePath,
		Preferred: cache.Preferred,
		Results:   nonNilProbeResults(cache.Results),
		Usable:    cache.Preferred != "" && !cache.GeneratedAt.IsZero() && (cache.ExpiresAt.IsZero() || time.Now().Before(cache.ExpiresAt)),
	}
	if !generatedAt.IsZero() {
		status.GeneratedAt = &generatedAt
	}
	if !expiresAt.IsZero() {
		status.ExpiresAt = &expiresAt
	}
	return status
}

func nonNilProbeResults(results []tdx.ServerProbeResult) []tdx.ServerProbeResult {
	if results == nil {
		return []tdx.ServerProbeResult{}
	}
	return results
}

func writeConsoleResult(w http.ResponseWriter, value any, err error) {
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("%w", err))
		return
	}
	writeJSON(w, http.StatusOK, value)
}
