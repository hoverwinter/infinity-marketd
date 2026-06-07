package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

var fetchHQQuotesList = tdx.FetchHQQuotesList
var fetchHQTopBoard = tdx.FetchHQTopBoard
var fetchHQLHB = tdx.FetchHQLHB
var fetchSPBoardMembers = tdx.FetchSPBoardMembers
var fetchFundKline = tdx.FetchFundKline
var fetchFundDetail = tdx.FetchFundDetail

var quotesSortNames = map[string]uint16{
	"code": tdx.QuotesSortCode, "price": tdx.QuotesSortPrice, "volume": tdx.QuotesSortVolume,
	"amount": tdx.QuotesSortAmount, "change": tdx.QuotesSortChangePct, "amplitude": tdx.QuotesSortAmplitude,
	"volratio": tdx.QuotesSortVolRatio, "turnover": tdx.QuotesSortTurnover, "speed": tdx.QuotesSortSpeed,
	"mainnet": tdx.QuotesSortMainNetAmt,
}

var quotesExcludeNames = map[string]uint16{
	"new": tdx.QuotesFilterNew, "kcb": tdx.QuotesFilterKCB, "st": tdx.QuotesFilterST,
	"cyb": tdx.QuotesFilterCYB, "bj": tdx.QuotesFilterBJ,
}

// parseUint16 accepts a named key (via names) or a raw numeric (decimal/0x hex).
func parseUint16(value string, names map[string]uint16) (uint16, error) {
	value = strings.TrimSpace(value)
	if v, ok := names[strings.ToLower(value)]; ok {
		return v, nil
	}
	n, err := strconv.ParseUint(value, 0, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid value %q", value)
	}
	return uint16(n), nil
}

func runHQQuotesList(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var category, start, count int
	var sort, exclude string
	var reverse bool
	fs := newFlagSet("hq-quotes-list", stderr)
	fs.IntVar(&category, "category", 0, "market category id (0=sh,2=sz,6=A,8=kcb,12=bj,14=cyb)")
	fs.StringVar(&sort, "sort", "change", "sort key name or raw id")
	fs.IntVar(&start, "start", 0, "pagination start offset")
	fs.IntVar(&count, "count", 50, "number of rows")
	fs.BoolVar(&reverse, "reverse", false, "ascending order")
	fs.StringVar(&exclude, "exclude", "", "comma-separated exclude filters: new,kcb,st,cyb,bj or raw")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	sortType, err := parseUint16(sort, quotesSortNames)
	if err != nil {
		fmt.Fprintln(stderr, "invalid --sort:", err)
		return 2
	}
	var excludeMask uint16
	for _, part := range strings.Split(exclude, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		bit, err := parseUint16(part, quotesExcludeNames)
		if err != nil {
			fmt.Fprintln(stderr, "invalid --exclude:", err)
			return 2
		}
		excludeMask |= bit
	}
	items, err := fetchHQQuotesList(ctx, tdx.HQQuotesListRequest{
		Category: uint16(category), SortType: sortType, Start: start, Count: count, Reverse: reverse, Exclude: excludeMask,
	}, hqClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, items)
}

func runHQTopBoard(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var category, size int
	fs := newFlagSet("hq-top-board", stderr)
	fs.IntVar(&category, "category", 0, "market category id")
	fs.IntVar(&size, "size", 10, "entries per ranking group (1-100)")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	groups, err := fetchHQTopBoard(ctx, uint16(category), size, hqClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, groups)
}

func runHQLHB(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market, symbol, alias string
	fs := newFlagSet("hq-lhb", stderr)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.StringVar(&alias, "alias", "", "F10 section alias (defaults to 资金动向)")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseHQMinuteRequest(market, symbol)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var aliases []string
	if strings.TrimSpace(alias) != "" {
		aliases = []string{alias}
	}
	result, err := fetchHQLHB(ctx, req, aliases, hqClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, result)
}

func runSPBoardMembers(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var server, board string
	var sortType, count, order int
	var timeout time.Duration
	fs := newFlagSet("sp-board-members", stderr)
	fs.StringVar(&server, "server", "", "SP server host:port (required; no public defaults)")
	fs.StringVar(&board, "board", "", "board id/symbol")
	fs.IntVar(&sortType, "sort-type", 0, "sort type")
	fs.IntVar(&count, "count", 80, "number of members")
	fs.IntVar(&order, "order", 0, "sort order")
	fs.DurationVar(&timeout, "timeout", 5*time.Second, "request timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(server) == "" {
		fmt.Fprintln(stderr, "--server is required for SP board members")
		return 2
	}
	items, err := fetchSPBoardMembers(ctx, server, board, uint16(sortType), count, uint16(order), timeout)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, items)
}

func runFundKline(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var server, code, period string
	var count int
	var timeout time.Duration
	fs := newFlagSet("fund-kline", stderr)
	fs.StringVar(&server, "server", "", "fund 7727 server host:port (required)")
	fs.StringVar(&code, "code", "", "six-digit fund code")
	fs.StringVar(&period, "period", "day", "1m,5m,15m,30m,60m,day,week,month")
	fs.IntVar(&count, "count", 100, "number of bars (1-800)")
	fs.DurationVar(&timeout, "timeout", 5*time.Second, "request timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(server) == "" {
		fmt.Fprintln(stderr, "--server is required for fund-kline")
		return 2
	}
	bars, err := fetchFundKline(ctx, server, code, period, count, timeout)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, bars)
}

func runFundDetail(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var server, code string
	var mode int
	var timeout time.Duration
	fs := newFlagSet("fund-detail", stderr)
	fs.StringVar(&server, "server", "", "fund 7727 server host:port (required)")
	fs.StringVar(&code, "code", "", "six-digit fund code")
	fs.IntVar(&mode, "mode", 0, "fund detail mode (default 50)")
	fs.DurationVar(&timeout, "timeout", 5*time.Second, "request timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(server) == "" {
		fmt.Fprintln(stderr, "--server is required for fund-detail")
		return 2
	}
	detail, err := fetchFundDetail(ctx, server, code, uint16(mode), timeout)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, detail)
}
