package querier

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/onlineadjust"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

var advancedQuotesSortNames = map[string]uint16{
	"code": tdx.QuotesSortCode, "price": tdx.QuotesSortPrice, "volume": tdx.QuotesSortVolume,
	"amount": tdx.QuotesSortAmount, "change": tdx.QuotesSortChangePct, "amplitude": tdx.QuotesSortAmplitude,
	"volratio": tdx.QuotesSortVolRatio, "turnover": tdx.QuotesSortTurnover, "speed": tdx.QuotesSortSpeed,
	"mainnet": tdx.QuotesSortMainNetAmt,
}

var advancedQuotesExcludeNames = map[string]uint16{
	"new": tdx.QuotesFilterNew, "kcb": tdx.QuotesFilterKCB, "st": tdx.QuotesFilterST,
	"cyb": tdx.QuotesFilterCYB, "bj": tdx.QuotesFilterBJ,
}

func parseUint16Query(value string, names map[string]uint16) (uint16, error) {
	value = strings.TrimSpace(value)
	if v, ok := names[strings.ToLower(value)]; ok {
		return v, nil
	}
	n, err := strconv.ParseUint(value, 0, 16)
	if err != nil {
		return 0, TDXValidationError{"invalid value " + strconv.Quote(value)}
	}
	return uint16(n), nil
}

func queryTimeout(r *http.Request) time.Duration {
	if v := strings.TrimSpace(r.URL.Query().Get("timeout")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 5 * time.Second
}

func (s *Server) handleTDXHQCompactQuotes(w http.ResponseWriter, r *http.Request) {
	symbols := splitQueryValues(r, "symbol", "symbols")
	if len(symbols) == 0 {
		writeError(w, http.StatusBadRequest, TDXValidationError{"at least one symbol is required"})
		return
	}
	if len(symbols) > tdx.MaxCompactBatchQuoteCount {
		writeError(w, http.StatusBadRequest, TDXValidationError{"compact batch quote symbol count must be <= " + strconv.Itoa(tdx.MaxCompactBatchQuoteCount)})
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
	items, err := s.tdxProvider.FetchHQCompactBatchQuotes(r.Context(), requests, hqOptionsFromRequestNoError(r))
	writeTDXResult(w, items, err)
}

func (s *Server) handleTDXHQTickChart(w http.ResponseWriter, r *http.Request) {
	start, err := queryInt(r, "start", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	count, err := queryInt(r, "count", tdx.MaxHQTickChartCount)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req, err := tdx.ParseHQTickChartRequest(r.URL.Query().Get("market"), r.URL.Query().Get("symbol"), start, count)
	if err != nil {
		writeError(w, http.StatusBadRequest, TDXValidationError{err.Error()})
		return
	}
	points, err := s.tdxProvider.FetchHQTickChart(r.Context(), req, hqOptionsFromRequestNoError(r))
	writeTDXResult(w, points, err)
}

func (s *Server) handleTDXHQQuotesList(w http.ResponseWriter, r *http.Request) {
	category, err := queryInt(r, "category", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	start, err := queryInt(r, "start", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	count, err := queryInt(r, "count", 50)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sortType := tdx.QuotesSortChangePct
	if v := strings.TrimSpace(r.URL.Query().Get("sort")); v != "" {
		sortType, err = parseUint16Query(v, advancedQuotesSortNames)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	var excludeMask uint16
	for _, part := range splitQueryValues(r, "exclude") {
		bit, err := parseUint16Query(part, advancedQuotesExcludeNames)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		excludeMask |= bit
	}
	items, err := s.tdxProvider.FetchHQQuotesList(r.Context(), tdx.HQQuotesListRequest{
		Category: uint16(category), SortType: sortType, Start: start, Count: count,
		Reverse: queryBool(r, "reverse"), Exclude: excludeMask,
	}, hqOptionsFromRequestNoError(r))
	writeTDXResult(w, items, err)
}

func (s *Server) handleTDXHQTopBoard(w http.ResponseWriter, r *http.Request) {
	category, err := queryInt(r, "category", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	size, err := queryInt(r, "size", 10)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	groups, err := s.tdxProvider.FetchHQTopBoard(r.Context(), uint16(category), size, hqOptionsFromRequestNoError(r))
	writeTDXResult(w, groups, err)
}

func (s *Server) handleTDXHQLHB(w http.ResponseWriter, r *http.Request) {
	req, err := tdx.ParseHQMinuteRequest(r.URL.Query().Get("market"), r.URL.Query().Get("symbol"))
	if err != nil {
		writeError(w, http.StatusBadRequest, TDXValidationError{err.Error()})
		return
	}
	var aliases []string
	if alias := strings.TrimSpace(r.URL.Query().Get("alias")); alias != "" {
		aliases = []string{alias}
	}
	result, err := s.tdxProvider.FetchHQLHB(r.Context(), req, aliases, hqOptionsFromRequestNoError(r))
	writeTDXResult(w, result, err)
}

func (s *Server) handleTDXHQAdjustedBarsOnline(w http.ResponseWriter, r *http.Request) {
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
	req, err := onlineadjust.NormalizeRequest(onlineadjust.HQAdjustedBarsOnlineRequest{
		Market: r.URL.Query().Get("market"), Symbol: r.URL.Query().Get("symbol"), Category: category, Start: start, Count: count, Adjust: strings.TrimSpace(r.URL.Query().Get("adjust")),
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, TDXValidationError{err.Error()})
		return
	}
	result, err := s.tdxProvider.FetchHQAdjustedBarsOnline(r.Context(), req, hqOptionsFromRequestNoError(r))
	writeTDXResult(w, result, err)
}

func (s *Server) handleTDXSPServers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.tdxProvider.SPServerCandidates())
}

func (s *Server) handleTDXSPProbe(w http.ResponseWriter, r *http.Request) {
	results := s.tdxProvider.ProbeSPServers(r.Context(), splitQueryValues(r, "server", "servers"), queryTimeout(r))
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleTDXSPBoardMembers(w http.ResponseWriter, r *http.Request) {
	server := strings.TrimSpace(r.URL.Query().Get("server"))
	if server == "" && queryBool(r, "best") {
		server = tdx.BestSPServer(s.tdxProvider.ProbeSPServers(r.Context(), nil, queryTimeout(r)))
		if server == "" {
			writeError(w, http.StatusServiceUnavailable, TDXUpstreamError{"no reachable SP server found by probe"})
			return
		}
	}
	if server == "" {
		writeError(w, http.StatusBadRequest, TDXValidationError{"server is required for SP board members unless best=true is set"})
		return
	}
	sortType, err := queryInt(r, "sort_type", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	count, err := queryInt(r, "count", 80)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	order, err := queryInt(r, "order", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	items, err := s.tdxProvider.FetchSPBoardMembers(r.Context(), server, strings.TrimSpace(r.URL.Query().Get("board")), uint16(sortType), count, uint16(order), queryTimeout(r))
	writeTDXResult(w, items, err)
}

func (s *Server) handleTDXFundServers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.tdxProvider.FundServerCandidates())
}

func (s *Server) handleTDXFundProbe(w http.ResponseWriter, r *http.Request) {
	results := s.tdxProvider.ProbeFundServers(r.Context(), splitQueryValues(r, "server", "servers"), queryTimeout(r))
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleTDXFundKline(w http.ResponseWriter, r *http.Request) {
	server := strings.TrimSpace(r.URL.Query().Get("server"))
	if server == "" && queryBool(r, "best") {
		server = tdx.BestFundServer(s.tdxProvider.ProbeFundServers(r.Context(), nil, queryTimeout(r)))
		if server == "" {
			writeError(w, http.StatusServiceUnavailable, TDXUpstreamError{"no reachable fund server found by probe"})
			return
		}
	}
	if server == "" {
		writeError(w, http.StatusBadRequest, TDXValidationError{"server is required for fund kline unless best=true is set"})
		return
	}
	count, err := queryInt(r, "count", 100)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	period := strings.TrimSpace(r.URL.Query().Get("period"))
	if period == "" {
		period = "day"
	}
	bars, err := s.tdxProvider.FetchFundKline(r.Context(), server, strings.TrimSpace(r.URL.Query().Get("code")), period, count, queryTimeout(r))
	writeTDXResult(w, bars, err)
}

func (s *Server) handleTDXFundDetail(w http.ResponseWriter, r *http.Request) {
	server := strings.TrimSpace(r.URL.Query().Get("server"))
	if server == "" && queryBool(r, "best") {
		server = tdx.BestFundServer(s.tdxProvider.ProbeFundServers(r.Context(), nil, queryTimeout(r)))
		if server == "" {
			writeError(w, http.StatusServiceUnavailable, TDXUpstreamError{"no reachable fund server found by probe"})
			return
		}
	}
	if server == "" {
		writeError(w, http.StatusBadRequest, TDXValidationError{"server is required for fund detail unless best=true is set"})
		return
	}
	mode, err := queryInt(r, "mode", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	detail, err := s.tdxProvider.FetchFundDetail(r.Context(), server, strings.TrimSpace(r.URL.Query().Get("code")), uint16(mode), queryTimeout(r))
	writeTDXResult(w, detail, err)
}
