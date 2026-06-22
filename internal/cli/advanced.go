package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/onlineadjust"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

var fetchHQCompactBatchQuotes = tdx.FetchHQCompactBatchQuotes
var fetchHQTickChart = tdx.FetchHQTickChart
var fetchHQQuotesList = tdx.FetchHQQuotesList
var fetchHQTopBoard = tdx.FetchHQTopBoard
var fetchHQLHB = tdx.FetchHQLHB
var fetchSPBoardMembers = tdx.FetchSPBoardMembers
var fetchFundKline = tdx.FetchFundKline
var fetchFundDetail = tdx.FetchFundDetail
var probeSPServers = tdx.ProbeSPServers
var probeFundServers = tdx.ProbeFundServers
var fetchHQAdjustedBarsOnline = onlineadjust.FetchHQAdjustedBarsOnline

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

func runHQCompactQuotes(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var symbols symbolFlags
	fs := newFlagSet("hq-compact-quotes", stderr)
	fs.Var(&symbols, "symbol", "symbol; repeat or comma-separate, supports market:symbol")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	requests := make([]tdx.QuoteRequest, 0, len(symbols))
	for _, symbol := range symbols {
		req, err := tdx.ParseQuoteRequest(symbol)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		requests = append(requests, req)
	}
	if len(requests) == 0 {
		fmt.Fprintln(stderr, "at least one symbol is required")
		return 2
	}
	if len(requests) > tdx.MaxCompactBatchQuoteCount {
		fmt.Fprintf(stderr, "compact batch quote symbol count must be <= %d\n", tdx.MaxCompactBatchQuoteCount)
		return 2
	}
	items, err := fetchHQCompactBatchQuotes(ctx, requests, hqClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, items)
}

func runHQTickChart(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market, symbol string
	var start, count int
	fs := newFlagSet("hq-tick-chart", stderr)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.IntVar(&start, "start", 0, "pagination start offset")
	fs.IntVar(&count, "count", tdx.MaxHQTickChartCount, "number of points")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseHQTickChartRequest(market, symbol, start, count)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	points, err := fetchHQTickChart(ctx, req, hqClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, points)
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
	var best bool
	fs := newFlagSet("sp-board-members", stderr)
	fs.StringVar(&server, "server", "", "SP server host:port (required; no public defaults)")
	fs.BoolVar(&best, "best", false, "probe built-in SP servers and use the fastest reachable server")
	fs.StringVar(&board, "board", "", "board id/symbol")
	fs.IntVar(&sortType, "sort-type", 0, "sort type")
	fs.IntVar(&count, "count", 80, "number of members")
	fs.IntVar(&order, "order", 0, "sort order")
	fs.DurationVar(&timeout, "timeout", 5*time.Second, "request timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(server) == "" && best {
		results := probeSPServers(ctx, nil, timeout)
		server = tdx.BestSPServer(results)
		if strings.TrimSpace(server) == "" {
			fmt.Fprintln(stderr, "no reachable SP server found by --best probe")
			return 1
		}
	}
	if strings.TrimSpace(server) == "" {
		fmt.Fprintln(stderr, "--server is required for SP board members unless --best is set")
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
	var best bool
	fs := newFlagSet("fund-kline", stderr)
	fs.StringVar(&server, "server", "", "fund 7727 server host:port (required)")
	fs.BoolVar(&best, "best", false, "probe built-in fund servers and use the fastest reachable server")
	fs.StringVar(&code, "code", "", "six-digit fund code")
	fs.StringVar(&period, "period", "day", "1m,5m,15m,30m,60m,day,week,month")
	fs.IntVar(&count, "count", 100, "number of bars (1-800)")
	fs.DurationVar(&timeout, "timeout", 5*time.Second, "request timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(server) == "" && best {
		results := probeFundServers(ctx, nil, timeout)
		server = tdx.BestFundServer(results)
		if strings.TrimSpace(server) == "" {
			fmt.Fprintln(stderr, "no reachable fund server found by --best probe")
			return 1
		}
	}
	if strings.TrimSpace(server) == "" {
		fmt.Fprintln(stderr, "--server is required for fund-kline unless --best is set")
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
	var best bool
	fs := newFlagSet("fund-detail", stderr)
	fs.StringVar(&server, "server", "", "fund 7727 server host:port (required)")
	fs.BoolVar(&best, "best", false, "probe built-in fund servers and use the fastest reachable server")
	fs.StringVar(&code, "code", "", "six-digit fund code")
	fs.IntVar(&mode, "mode", 0, "fund detail mode (default 50)")
	fs.DurationVar(&timeout, "timeout", 5*time.Second, "request timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(server) == "" && best {
		results := probeFundServers(ctx, nil, timeout)
		server = tdx.BestFundServer(results)
		if strings.TrimSpace(server) == "" {
			fmt.Fprintln(stderr, "no reachable fund server found by --best probe")
			return 1
		}
	}
	if strings.TrimSpace(server) == "" {
		fmt.Fprintln(stderr, "--server is required for fund-detail unless --best is set")
		return 2
	}
	detail, err := fetchFundDetail(ctx, server, code, uint16(mode), timeout)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, detail)
}

func runSPServers(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("sp-servers", stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	return writeJSON(stdout, stderr, tdx.SPServerCandidates())
}

func runFundServers(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("fund-servers", stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	return writeJSON(stdout, stderr, tdx.FundServerCandidates())
}

func runSPProbe(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var timeout time.Duration
	fs := newFlagSet("sp-probe", stderr)
	fs.Var(&servers, "server", "SP server host:port; repeat or comma-separate")
	fs.DurationVar(&timeout, "timeout", 5*time.Second, "request timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	return writeJSON(stdout, stderr, probeSPServers(ctx, []string(servers), timeout))
}

func runFundProbe(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var timeout time.Duration
	fs := newFlagSet("fund-probe", stderr)
	fs.Var(&servers, "server", "fund server host:port; repeat or comma-separate")
	fs.DurationVar(&timeout, "timeout", 5*time.Second, "request timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	return writeJSON(stdout, stderr, probeFundServers(ctx, []string(servers), timeout))
}

func runHQAdjustedBarsOnline(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market, symbol, adjustMode string
	var category, start, count int
	fs := newFlagSet("hq-adjusted-bars-online", stderr)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.IntVar(&category, "category", tdx.HQKLineDayAlt, "K-line category")
	fs.IntVar(&start, "start", 0, "pagination start offset")
	fs.IntVar(&count, "count", tdx.DefaultHQKLineCount, "number of bars")
	fs.StringVar(&adjustMode, "adjust", "none", "adjustment mode: none, qfq, or hfq")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := onlineadjust.NormalizeRequest(onlineadjust.HQAdjustedBarsOnlineRequest{
		Market: market, Symbol: symbol, Category: category, Start: start, Count: count, Adjust: adjustMode,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	result, err := fetchHQAdjustedBarsOnline(ctx, req, hqClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, result)
}
