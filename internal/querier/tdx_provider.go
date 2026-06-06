package querier

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

const (
	maxHTTPQuoteSymbols = 200
	maxHTTPBatchSize    = 200
)

type TDXProvider struct {
	FetchRealtimeQuotes          func(context.Context, []tdx.QuoteRequest, tdx.QuoteClientOptions) ([]tdx.Quote, error)
	ProbeHQServers               func(context.Context, []string, tdx.QuoteClientOptions) []tdx.ServerProbeResult
	FetchSecurityList            func(context.Context, string, tdx.QuoteClientOptions) ([]tdx.Security, error)
	FetchHQSecurityBars          func(context.Context, tdx.HQBarsRequest, tdx.QuoteClientOptions) ([]tdx.HQBar, error)
	FetchHQIndexBars             func(context.Context, tdx.HQBarsRequest, tdx.QuoteClientOptions) ([]tdx.HQBar, error)
	FetchHQMinuteTime            func(context.Context, tdx.HQMinuteRequest, tdx.QuoteClientOptions) ([]tdx.HQMinutePoint, error)
	FetchHQHistoryMinuteTime     func(context.Context, tdx.HQMinuteRequest, int, tdx.QuoteClientOptions) ([]tdx.HQMinutePoint, error)
	FetchHQTransactions          func(context.Context, tdx.HQMinuteRequest, int, int, tdx.QuoteClientOptions) ([]tdx.HQTransaction, error)
	FetchHQHistoryTransactions   func(context.Context, tdx.HQMinuteRequest, int, int, int, tdx.QuoteClientOptions) ([]tdx.HQTransaction, error)
	FetchHQCompanyInfoCategories func(context.Context, tdx.HQMinuteRequest, tdx.QuoteClientOptions) ([]tdx.HQCompanyInfoCategory, error)
	FetchHQCompanyInfoContent    func(context.Context, tdx.HQMinuteRequest, string, uint32, uint32, tdx.QuoteClientOptions) (tdx.HQCompanyInfoContent, error)
	FetchHQXDXRInfo              func(context.Context, tdx.HQMinuteRequest, tdx.QuoteClientOptions) ([]tdx.HQXDXRInfo, error)
	FetchHQFinanceInfo           func(context.Context, tdx.HQMinuteRequest, tdx.QuoteClientOptions) (tdx.HQFinanceInfo, error)
	FetchHQBlockMeta             func(context.Context, string, tdx.QuoteClientOptions) (tdx.HQBlockMeta, error)
	FetchHQBlockChunk            func(context.Context, string, uint32, uint32, tdx.QuoteClientOptions) (tdx.HQBlockChunk, error)
	FetchHQBlockMembers          func(context.Context, string, tdx.QuoteClientOptions) ([]tdx.HQBlockMember, error)
	FetchExMarkets               func(context.Context, tdx.ExQuoteClientOptions) ([]tdx.ExMarket, error)
	FetchExInstrumentCount       func(context.Context, tdx.ExQuoteClientOptions) (int, error)
	FetchExInstruments           func(context.Context, int, int, tdx.ExQuoteClientOptions) ([]tdx.ExInstrument, error)
	FetchExQuote                 func(context.Context, tdx.ExQuoteRequest, tdx.ExQuoteClientOptions) (tdx.ExQuote, error)
	FetchExBars                  func(context.Context, tdx.ExBarsRequest, tdx.ExQuoteClientOptions) ([]tdx.ExBar, error)
	FetchExMinuteTime            func(context.Context, tdx.ExQuoteRequest, tdx.ExQuoteClientOptions) ([]tdx.ExMinutePoint, error)
	FetchExHistoryMinuteTime     func(context.Context, tdx.ExQuoteRequest, int, tdx.ExQuoteClientOptions) ([]tdx.ExMinutePoint, error)
	FetchExTransactions          func(context.Context, tdx.ExQuoteRequest, int, int, tdx.ExQuoteClientOptions) ([]tdx.ExTransaction, error)
	FetchExHistoryTransactions   func(context.Context, tdx.ExQuoteRequest, int, int, int, tdx.ExQuoteClientOptions) ([]tdx.ExTransaction, error)
	FetchExHistoryBarsRange      func(context.Context, tdx.ExQuoteRequest, int, int, tdx.ExQuoteClientOptions) ([]tdx.ExBar, error)
}

func DefaultTDXProvider() *TDXProvider {
	return &TDXProvider{
		FetchRealtimeQuotes:          tdx.FetchRealtimeQuotes,
		ProbeHQServers:               tdx.ProbeHQServers,
		FetchSecurityList:            tdx.FetchSecurityList,
		FetchHQSecurityBars:          tdx.FetchHQSecurityBars,
		FetchHQIndexBars:             tdx.FetchHQIndexBars,
		FetchHQMinuteTime:            tdx.FetchHQMinuteTime,
		FetchHQHistoryMinuteTime:     tdx.FetchHQHistoryMinuteTime,
		FetchHQTransactions:          tdx.FetchHQTransactions,
		FetchHQHistoryTransactions:   tdx.FetchHQHistoryTransactions,
		FetchHQCompanyInfoCategories: tdx.FetchHQCompanyInfoCategories,
		FetchHQCompanyInfoContent:    tdx.FetchHQCompanyInfoContent,
		FetchHQXDXRInfo:              tdx.FetchHQXDXRInfo,
		FetchHQFinanceInfo:           tdx.FetchHQFinanceInfo,
		FetchHQBlockMeta:             tdx.FetchHQBlockMeta,
		FetchHQBlockChunk:            tdx.FetchHQBlockChunk,
		FetchHQBlockMembers:          tdx.FetchHQBlockMembers,
		FetchExMarkets:               tdx.FetchExMarkets,
		FetchExInstrumentCount:       tdx.FetchExInstrumentCount,
		FetchExInstruments:           tdx.FetchExInstruments,
		FetchExQuote:                 tdx.FetchExQuote,
		FetchExBars:                  tdx.FetchExBars,
		FetchExMinuteTime:            tdx.FetchExMinuteTime,
		FetchExHistoryMinuteTime:     tdx.FetchExHistoryMinuteTime,
		FetchExTransactions:          tdx.FetchExTransactions,
		FetchExHistoryTransactions:   tdx.FetchExHistoryTransactions,
		FetchExHistoryBarsRange:      tdx.FetchExHistoryBarsRange,
	}
}

type TDXValidationError struct {
	Message string
}

func (e TDXValidationError) Error() string {
	return e.Message
}

type TDXProtocolError struct {
	Message string
}

func (e TDXProtocolError) Error() string {
	return e.Message
}

type TDXUpstreamError struct {
	Message string
}

func (e TDXUpstreamError) Error() string {
	return e.Message
}

func (s *Server) registerTDXRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/tdx/hq/quotes", s.handleTDXHQQuotes)
	mux.HandleFunc("GET /api/tdx/hq/probe", s.handleTDXHQProbe)
	mux.HandleFunc("GET /api/tdx/hq/securities", s.handleTDXHQSecurities)
	mux.HandleFunc("GET /api/tdx/hq/bars", s.handleTDXHQBars)
	mux.HandleFunc("GET /api/tdx/hq/minute", s.handleTDXHQMinute)
	mux.HandleFunc("GET /api/tdx/hq/transactions", s.handleTDXHQTransactions)
	mux.HandleFunc("GET /api/tdx/hq/company-categories", s.handleTDXHQCompanyCategories)
	mux.HandleFunc("GET /api/tdx/hq/company-content", s.handleTDXHQCompanyContent)
	mux.HandleFunc("GET /api/tdx/hq/xdxr", s.handleTDXHQXDXR)
	mux.HandleFunc("GET /api/tdx/hq/finance", s.handleTDXHQFinance)
	mux.HandleFunc("GET /api/tdx/hq/block-meta", s.handleTDXHQBlockMeta)
	mux.HandleFunc("GET /api/tdx/hq/block-chunk", s.handleTDXHQBlockChunk)
	mux.HandleFunc("GET /api/tdx/hq/block", s.handleTDXHQBlock)
	mux.HandleFunc("GET /api/tdx/exhq/markets", s.handleTDXExHQMarkets)
	mux.HandleFunc("GET /api/tdx/exhq/count", s.handleTDXExHQCount)
	mux.HandleFunc("GET /api/tdx/exhq/instruments", s.handleTDXExHQInstruments)
	mux.HandleFunc("GET /api/tdx/exhq/quote", s.handleTDXExHQQuote)
	mux.HandleFunc("GET /api/tdx/exhq/bars", s.handleTDXExHQBars)
	mux.HandleFunc("GET /api/tdx/exhq/minute", s.handleTDXExHQMinute)
	mux.HandleFunc("GET /api/tdx/exhq/transactions", s.handleTDXExHQTransactions)
	mux.HandleFunc("GET /api/tdx/exhq/history-bars", s.handleTDXExHQHistoryBars)
}

func (s *Server) handleTDXHQQuotes(w http.ResponseWriter, r *http.Request) {
	symbols := splitQueryValues(r, "symbol", "symbols")
	if len(symbols) == 0 {
		writeError(w, http.StatusBadRequest, TDXValidationError{"at least one symbol is required"})
		return
	}
	if len(symbols) > maxHTTPQuoteSymbols {
		writeError(w, http.StatusBadRequest, TDXValidationError{fmt.Sprintf("symbol count must be <= %d", maxHTTPQuoteSymbols)})
		return
	}
	requests := make([]tdx.QuoteRequest, 0, len(symbols))
	for _, symbol := range symbols {
		req, err := tdx.ParseQuoteRequest(symbol)
		if err != nil {
			writeError(w, http.StatusBadRequest, TDXValidationError{err.Error()})
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
	writeTDXResult(w, quotes, err)
}

func (s *Server) handleTDXHQProbe(w http.ResponseWriter, r *http.Request) {
	servers := splitQueryValues(r, "server", "servers")
	results := s.tdxProvider.ProbeHQServers(r.Context(), servers, tdx.QuoteClientOptions{})
	tdx.SortProbeResults(results)
	if best := tdx.BestHQServer(results); best != "" {
		for i := range results {
			results[i].Preferred = results[i].Server == best
		}
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleTDXHQSecurities(w http.ResponseWriter, r *http.Request) {
	market := strings.TrimSpace(r.URL.Query().Get("market"))
	opts, err := hqOptionsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	items, err := s.tdxProvider.FetchSecurityList(r.Context(), market, opts)
	writeTDXResult(w, items, err)
}

func (s *Server) handleTDXHQBars(w http.ResponseWriter, r *http.Request) {
	category, err := queryInt(r, "category", tdx.HQKLineDayAlt)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	start, err := queryInt(r, "start", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	count, err := queryInt(r, "count", tdx.DefaultHQKLineCount)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req, err := tdx.ParseHQBarsRequest(category, r.URL.Query().Get("market"), r.URL.Query().Get("symbol"), start, count)
	if err != nil {
		writeError(w, http.StatusBadRequest, TDXValidationError{err.Error()})
		return
	}
	opts, err := hqOptionsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var bars []tdx.HQBar
	if queryBool(r, "index") {
		bars, err = s.tdxProvider.FetchHQIndexBars(r.Context(), req, opts)
	} else {
		bars, err = s.tdxProvider.FetchHQSecurityBars(r.Context(), req, opts)
	}
	writeTDXResult(w, bars, err)
}

func (s *Server) handleTDXHQMinute(w http.ResponseWriter, r *http.Request) {
	req, err := tdx.ParseHQMinuteRequest(r.URL.Query().Get("market"), r.URL.Query().Get("symbol"))
	if err != nil {
		writeError(w, http.StatusBadRequest, TDXValidationError{err.Error()})
		return
	}
	opts, err := hqOptionsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if dateText := strings.TrimSpace(r.URL.Query().Get("date")); dateText != "" {
		date, err := parseYYYYMMDDQuery("date", dateText)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		points, err := s.tdxProvider.FetchHQHistoryMinuteTime(r.Context(), req, date, opts)
		writeTDXResult(w, points, err)
		return
	}
	points, err := s.tdxProvider.FetchHQMinuteTime(r.Context(), req, opts)
	writeTDXResult(w, points, err)
}

func (s *Server) handleTDXHQTransactions(w http.ResponseWriter, r *http.Request) {
	start, err := queryInt(r, "start", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	count, err := queryInt(r, "count", tdx.DefaultHQTransactionCount)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req, err := tdx.ParseHQTransactionRequest(r.URL.Query().Get("market"), r.URL.Query().Get("symbol"), start, count)
	if err != nil {
		writeError(w, http.StatusBadRequest, TDXValidationError{err.Error()})
		return
	}
	opts, err := hqOptionsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if dateText := strings.TrimSpace(r.URL.Query().Get("date")); dateText != "" {
		date, err := parseYYYYMMDDQuery("date", dateText)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		rows, err := s.tdxProvider.FetchHQHistoryTransactions(r.Context(), req, date, start, count, opts)
		writeTDXResult(w, rows, err)
		return
	}
	rows, err := s.tdxProvider.FetchHQTransactions(r.Context(), req, start, count, opts)
	writeTDXResult(w, rows, err)
}

func (s *Server) handleTDXHQCompanyCategories(w http.ResponseWriter, r *http.Request) {
	req, opts, ok := s.hqMinuteRequestWithOptions(w, r)
	if !ok {
		return
	}
	rows, err := s.tdxProvider.FetchHQCompanyInfoCategories(r.Context(), req, opts)
	writeTDXResult(w, rows, err)
}

func (s *Server) handleTDXHQCompanyContent(w http.ResponseWriter, r *http.Request) {
	req, opts, ok := s.hqMinuteRequestWithOptions(w, r)
	if !ok {
		return
	}
	filename := strings.TrimSpace(r.URL.Query().Get("filename"))
	start, err := queryUint32(r, "start", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	length, err := queryUint32(r, "length", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if length == 0 {
		writeError(w, http.StatusBadRequest, TDXValidationError{"length must be positive"})
		return
	}
	content, err := s.tdxProvider.FetchHQCompanyInfoContent(r.Context(), req, filename, start, length, opts)
	writeTDXResult(w, content, err)
}

func (s *Server) handleTDXHQXDXR(w http.ResponseWriter, r *http.Request) {
	req, opts, ok := s.hqMinuteRequestWithOptions(w, r)
	if !ok {
		return
	}
	rows, err := s.tdxProvider.FetchHQXDXRInfo(r.Context(), req, opts)
	writeTDXResult(w, rows, err)
}

func (s *Server) handleTDXHQFinance(w http.ResponseWriter, r *http.Request) {
	req, opts, ok := s.hqMinuteRequestWithOptions(w, r)
	if !ok {
		return
	}
	info, err := s.tdxProvider.FetchHQFinanceInfo(r.Context(), req, opts)
	writeTDXResult(w, info, err)
}

func (s *Server) handleTDXHQBlockMeta(w http.ResponseWriter, r *http.Request) {
	file := strings.TrimSpace(r.URL.Query().Get("file"))
	if file == "" {
		writeError(w, http.StatusBadRequest, TDXValidationError{"file is required"})
		return
	}
	opts, err := hqOptionsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	meta, err := s.tdxProvider.FetchHQBlockMeta(r.Context(), file, opts)
	writeTDXResult(w, meta, err)
}

func (s *Server) handleTDXHQBlockChunk(w http.ResponseWriter, r *http.Request) {
	file := strings.TrimSpace(r.URL.Query().Get("file"))
	if file == "" {
		writeError(w, http.StatusBadRequest, TDXValidationError{"file is required"})
		return
	}
	start, err := queryUint32(r, "start", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	size, err := queryUint32(r, "size", tdx.DefaultHQBlockChunkSize)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if size == 0 {
		writeError(w, http.StatusBadRequest, TDXValidationError{"size must be positive"})
		return
	}
	opts, err := hqOptionsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	chunk, err := s.tdxProvider.FetchHQBlockChunk(r.Context(), file, start, size, opts)
	writeTDXResult(w, chunk, err)
}

func (s *Server) handleTDXHQBlock(w http.ResponseWriter, r *http.Request) {
	file := strings.TrimSpace(r.URL.Query().Get("file"))
	if file == "" {
		writeError(w, http.StatusBadRequest, TDXValidationError{"file is required"})
		return
	}
	opts, err := hqOptionsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	members, err := s.tdxProvider.FetchHQBlockMembers(r.Context(), file, opts)
	writeTDXResult(w, members, err)
}

func (s *Server) hqMinuteRequestWithOptions(w http.ResponseWriter, r *http.Request) (tdx.HQMinuteRequest, tdx.QuoteClientOptions, bool) {
	req, err := tdx.ParseHQMinuteRequest(r.URL.Query().Get("market"), r.URL.Query().Get("symbol"))
	if err != nil {
		writeError(w, http.StatusBadRequest, TDXValidationError{err.Error()})
		return tdx.HQMinuteRequest{}, tdx.QuoteClientOptions{}, false
	}
	opts, err := hqOptionsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return tdx.HQMinuteRequest{}, tdx.QuoteClientOptions{}, false
	}
	return req, opts, true
}

func (s *Server) handleTDXExHQMarkets(w http.ResponseWriter, r *http.Request) {
	markets, err := s.tdxProvider.FetchExMarkets(r.Context(), exOptionsFromRequest(r))
	writeTDXResult(w, markets, err)
}

func (s *Server) handleTDXExHQCount(w http.ResponseWriter, r *http.Request) {
	count, err := s.tdxProvider.FetchExInstrumentCount(r.Context(), exOptionsFromRequest(r))
	writeTDXResult(w, map[string]int{"count": count}, err)
}

func (s *Server) handleTDXExHQInstruments(w http.ResponseWriter, r *http.Request) {
	start, err := queryInt(r, "start", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	count, err := queryInt(r, "count", tdx.DefaultExInstrumentListCount)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if start < 0 || count <= 0 || count > tdx.MaxExInstrumentListCount {
		writeError(w, http.StatusBadRequest, TDXValidationError{fmt.Sprintf("start must be non-negative and count must be between 1 and %d", tdx.MaxExInstrumentListCount)})
		return
	}
	rows, err := s.tdxProvider.FetchExInstruments(r.Context(), start, count, exOptionsFromRequest(r))
	writeTDXResult(w, rows, err)
}

func (s *Server) handleTDXExHQQuote(w http.ResponseWriter, r *http.Request) {
	req, err := exQuoteRequestFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	quote, err := s.tdxProvider.FetchExQuote(r.Context(), req, exOptionsFromRequest(r))
	writeTDXResult(w, quote, err)
}

func (s *Server) handleTDXExHQBars(w http.ResponseWriter, r *http.Request) {
	category, err := queryInt(r, "category", tdx.ExKLineDaily)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	market, err := queryInt(r, "market", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	start, err := queryInt(r, "start", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	count, err := queryInt(r, "count", 100)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req, err := tdx.ParseExBarsRequest(category, market, r.URL.Query().Get("code"), start, count)
	if err != nil {
		writeError(w, http.StatusBadRequest, TDXValidationError{err.Error()})
		return
	}
	bars, err := s.tdxProvider.FetchExBars(r.Context(), req, exOptionsFromRequest(r))
	writeTDXResult(w, bars, err)
}

func (s *Server) handleTDXExHQMinute(w http.ResponseWriter, r *http.Request) {
	req, err := exQuoteRequestFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if dateText := strings.TrimSpace(r.URL.Query().Get("date")); dateText != "" {
		date, err := parseYYYYMMDDQuery("date", dateText)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		points, err := s.tdxProvider.FetchExHistoryMinuteTime(r.Context(), req, date, exOptionsFromRequest(r))
		writeTDXResult(w, points, err)
		return
	}
	points, err := s.tdxProvider.FetchExMinuteTime(r.Context(), req, exOptionsFromRequest(r))
	writeTDXResult(w, points, err)
}

func (s *Server) handleTDXExHQTransactions(w http.ResponseWriter, r *http.Request) {
	req, err := exQuoteRequestFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	start, err := queryInt(r, "start", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	count, err := queryInt(r, "count", tdx.MaxExTransactionCount)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if start < 0 || count <= 0 || count > tdx.MaxExTransactionCount {
		writeError(w, http.StatusBadRequest, TDXValidationError{fmt.Sprintf("start must be non-negative and count must be between 1 and %d", tdx.MaxExTransactionCount)})
		return
	}
	if dateText := strings.TrimSpace(r.URL.Query().Get("date")); dateText != "" {
		date, err := parseYYYYMMDDQuery("date", dateText)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		rows, err := s.tdxProvider.FetchExHistoryTransactions(r.Context(), req, date, start, count, exOptionsFromRequest(r))
		writeTDXResult(w, rows, err)
		return
	}
	rows, err := s.tdxProvider.FetchExTransactions(r.Context(), req, start, count, exOptionsFromRequest(r))
	writeTDXResult(w, rows, err)
}

func (s *Server) handleTDXExHQHistoryBars(w http.ResponseWriter, r *http.Request) {
	req, err := exQuoteRequestFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	startDate, err := parseYYYYMMDDQuery("start_date", firstNonEmpty(r.URL.Query().Get("start_date"), r.URL.Query().Get("start-date")))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	endDate, err := parseYYYYMMDDQuery("end_date", firstNonEmpty(r.URL.Query().Get("end_date"), r.URL.Query().Get("end-date")))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if startDate > endDate {
		writeError(w, http.StatusBadRequest, TDXValidationError{"start_date must be <= end_date"})
		return
	}
	bars, err := s.tdxProvider.FetchExHistoryBarsRange(r.Context(), req, startDate, endDate, exOptionsFromRequest(r))
	writeTDXResult(w, bars, err)
}

func exQuoteRequestFromRequest(r *http.Request) (tdx.ExQuoteRequest, error) {
	market, err := queryInt(r, "market", 0)
	if err != nil {
		return tdx.ExQuoteRequest{}, err
	}
	req, err := tdx.ParseExQuoteRequest(market, r.URL.Query().Get("code"))
	if err != nil {
		return tdx.ExQuoteRequest{}, TDXValidationError{err.Error()}
	}
	return req, nil
}

func writeTDXResult(w http.ResponseWriter, value any, err error) {
	if err != nil {
		writeError(w, statusForTDXError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func statusForTDXError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	var validationErr TDXValidationError
	if errors.As(err, &validationErr) {
		return http.StatusBadRequest
	}
	var protocolErr TDXProtocolError
	if errors.As(err, &protocolErr) {
		return http.StatusBadGateway
	}
	var upstreamErr TDXUpstreamError
	if errors.As(err, &upstreamErr) {
		return http.StatusServiceUnavailable
	}
	msg := err.Error()
	if strings.Contains(msg, "decode TDX") || strings.Contains(msg, "response too short") || strings.Contains(msg, "response truncated") {
		return http.StatusBadGateway
	}
	if strings.Contains(msg, "failed on") || strings.Contains(msg, "connect TDX") || strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") {
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}

func quoteOptionsFromRequest(r *http.Request) (tdx.QuoteClientOptions, error) {
	opts := hqOptionsFromRequestNoError(r)
	batchSize, err := queryInt(r, "batch-size", 0)
	if err != nil {
		return opts, err
	}
	if !hasQuery(r, "batch-size") {
		batchSize, err = queryInt(r, "batch_size", 0)
		if err != nil {
			return opts, err
		}
	}
	if batchSize < 0 || batchSize > maxHTTPBatchSize {
		return opts, TDXValidationError{fmt.Sprintf("batch_size must be between 0 and %d", maxHTTPBatchSize)}
	}
	opts.BatchSize = batchSize
	if tradeDateText := strings.TrimSpace(r.URL.Query().Get("trade_date")); tradeDateText != "" {
		loc, err := time.LoadLocation("Asia/Shanghai")
		if err != nil {
			return opts, err
		}
		tradeDate, err := time.ParseInLocation("2006-01-02", tradeDateText, loc)
		if err != nil {
			return opts, TDXValidationError{"trade_date must be YYYY-MM-DD"}
		}
		opts.TradeDate = tradeDate
		opts.Location = loc
	}
	return opts, nil
}

func hqOptionsFromRequest(r *http.Request) (tdx.QuoteClientOptions, error) {
	return hqOptionsFromRequestNoError(r), nil
}

func hqOptionsFromRequestNoError(r *http.Request) tdx.QuoteClientOptions {
	return tdx.QuoteClientOptions{Servers: splitQueryValues(r, "server", "servers")}
}

func exOptionsFromRequest(r *http.Request) tdx.ExQuoteClientOptions {
	return tdx.ExQuoteClientOptions{Servers: splitQueryValues(r, "server", "servers")}
}

func splitQueryValues(r *http.Request, names ...string) []string {
	var out []string
	for _, name := range names {
		for _, value := range r.URL.Query()[name] {
			for _, part := range strings.Split(value, ",") {
				part = strings.TrimSpace(part)
				if part != "" {
					out = append(out, part)
				}
			}
		}
	}
	return out
}

func queryInt(r *http.Request, name string, fallback int) (int, error) {
	value := strings.TrimSpace(r.URL.Query().Get(name))
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, TDXValidationError{fmt.Sprintf("%s must be an integer", name)}
	}
	return n, nil
}

func queryBool(r *http.Request, name string) bool {
	value := strings.ToLower(strings.TrimSpace(r.URL.Query().Get(name)))
	return value == "1" || value == "true" || value == "yes"
}

func queryUint32(r *http.Request, name string, fallback uint32) (uint32, error) {
	value := strings.TrimSpace(r.URL.Query().Get(name))
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, TDXValidationError{fmt.Sprintf("%s must fit uint32", name)}
	}
	return uint32(n), nil
}

func parseYYYYMMDDQuery(name, value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, TDXValidationError{fmt.Sprintf("%s is required", name)}
	}
	if len(value) != 8 {
		return 0, TDXValidationError{fmt.Sprintf("%s must be YYYYMMDD", name)}
	}
	if _, err := time.Parse("20060102", value); err != nil {
		return 0, TDXValidationError{fmt.Sprintf("%s must be YYYYMMDD", name)}
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, TDXValidationError{fmt.Sprintf("%s must be YYYYMMDD", name)}
	}
	return n, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func hasQuery(r *http.Request, name string) bool {
	_, ok := r.URL.Query()[name]
	return ok
}
