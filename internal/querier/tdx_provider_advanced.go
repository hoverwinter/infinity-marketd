package querier

import (
	"net/http"
	"strconv"
	"strings"
	"time"

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

func (s *Server) handleTDXSPBoardMembers(w http.ResponseWriter, r *http.Request) {
	server := strings.TrimSpace(r.URL.Query().Get("server"))
	if server == "" {
		writeError(w, http.StatusBadRequest, TDXValidationError{"server is required for SP board members"})
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

func (s *Server) handleTDXFundKline(w http.ResponseWriter, r *http.Request) {
	server := strings.TrimSpace(r.URL.Query().Get("server"))
	if server == "" {
		writeError(w, http.StatusBadRequest, TDXValidationError{"server is required for fund kline"})
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
	if server == "" {
		writeError(w, http.StatusBadRequest, TDXValidationError{"server is required for fund detail"})
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
