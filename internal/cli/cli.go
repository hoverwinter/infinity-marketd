package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	chstore "github.com/hoverwinter/infinity-marketd/internal/clickhouse"
	"github.com/hoverwinter/infinity-marketd/internal/config"
	"github.com/hoverwinter/infinity-marketd/internal/ingest"
	"github.com/hoverwinter/infinity-marketd/internal/logging"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

var fetchRealtimeQuotes = tdx.FetchRealtimeQuotes
var probeHQServers = tdx.ProbeHQServers
var refreshHQBestIPCache = tdx.RefreshHQBestIPCache
var fetchQuoteSweep = tdx.QuoteSweep
var fetchHQSecurityBars = tdx.FetchHQSecurityBars
var fetchHQIndexBars = tdx.FetchHQIndexBars
var fetchHQMinuteTime = tdx.FetchHQMinuteTime
var fetchHQHistoryMinuteTime = tdx.FetchHQHistoryMinuteTime
var fetchHQTransactions = tdx.FetchHQTransactions
var fetchHQHistoryTransactions = tdx.FetchHQHistoryTransactions
var fetchHQCompanyInfoCategories = tdx.FetchHQCompanyInfoCategories
var fetchHQCompanyInfoContent = tdx.FetchHQCompanyInfoContent
var fetchHQXDXRInfo = tdx.FetchHQXDXRInfo
var fetchHQFinanceInfo = tdx.FetchHQFinanceInfo
var fetchHQBlockMeta = tdx.FetchHQBlockMeta
var fetchHQBlockMembers = tdx.FetchHQBlockMembers
var fetchExMarkets = tdx.FetchExMarkets
var fetchExQuote = tdx.FetchExQuote
var fetchExInstrumentCount = tdx.FetchExInstrumentCount
var fetchExInstruments = tdx.FetchExInstruments
var fetchExBars = tdx.FetchExBars
var fetchExMinuteTime = tdx.FetchExMinuteTime
var fetchExHistoryMinuteTime = tdx.FetchExHistoryMinuteTime
var fetchExTransactions = tdx.FetchExTransactions
var fetchExHistoryTransactions = tdx.FetchExHistoryTransactions
var fetchExHistoryBarsRange = tdx.FetchExHistoryBarsRange

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "bootstrap":
		return runBootstrap(ctx, args[1:], stdout, stderr)
	case "status":
		return runStatus(ctx, args[1:], stdout, stderr)
	case "import-tdx-day":
		return runImport(ctx, args[1:], stdout, stderr, tdx.PeriodDay)
	case "import-tdx-1m":
		return runImport(ctx, args[1:], stdout, stderr, tdx.Period1m)
	case "import-tdx-5m":
		return runImport(ctx, args[1:], stdout, stderr, tdx.Period5m)
	case "import-tdx-vipdoc-zip":
		return runImportVIPDocZip(ctx, args[1:], stdout, stderr)
	case "import-tdx-fin":
		return runImportTDXFinancial(ctx, args[1:], stdout, stderr)
	case "import-tdx-gp":
		return runImportTDXGP(ctx, args[1:], stdout, stderr)
	case "quote":
		return runQuote(ctx, args[1:], stdout, stderr)
	case "quote-probe":
		return runQuoteProbe(ctx, args[1:], stdout, stderr)
	case "quote-bestip":
		return runQuoteBestIP(ctx, args[1:], stdout, stderr)
	case "quote-sweep":
		return runQuoteSweep(ctx, args[1:], stdout, stderr)
	case "hq-bars":
		return runHQBars(ctx, args[1:], stdout, stderr, false)
	case "hq-index-bars":
		return runHQBars(ctx, args[1:], stdout, stderr, true)
	case "hq-minute":
		return runHQMinute(ctx, args[1:], stdout, stderr)
	case "hq-history-minute":
		return runHQHistoryMinute(ctx, args[1:], stdout, stderr)
	case "hq-transactions":
		return runHQTransactions(ctx, args[1:], stdout, stderr)
	case "hq-history-transactions":
		return runHQHistoryTransactions(ctx, args[1:], stdout, stderr)
	case "hq-company-categories":
		return runHQCompanyCategories(ctx, args[1:], stdout, stderr)
	case "hq-company-content":
		return runHQCompanyContent(ctx, args[1:], stdout, stderr)
	case "hq-xdxr":
		return runHQXDXR(ctx, args[1:], stdout, stderr)
	case "hq-finance":
		return runHQFinance(ctx, args[1:], stdout, stderr)
	case "hq-block-meta":
		return runHQBlockMeta(ctx, args[1:], stdout, stderr)
	case "hq-block":
		return runHQBlock(ctx, args[1:], stdout, stderr)
	case "quote-serve":
		return runQuoteServe(ctx, args[1:], stdout, stderr)
	case "quote-status":
		return runQuoteStatus(ctx, args[1:], stdout, stderr)
	case "exquote-markets":
		return runExQuoteMarkets(ctx, args[1:], stdout, stderr)
	case "exquote-count":
		return runExQuoteCount(ctx, args[1:], stdout, stderr)
	case "exquote-instruments":
		return runExQuoteInstruments(ctx, args[1:], stdout, stderr)
	case "exquote":
		return runExQuote(ctx, args[1:], stdout, stderr)
	case "exquote-bars":
		return runExQuoteBars(ctx, args[1:], stdout, stderr)
	case "exquote-minute":
		return runExQuoteMinute(ctx, args[1:], stdout, stderr)
	case "exquote-history-minute":
		return runExQuoteHistoryMinute(ctx, args[1:], stdout, stderr)
	case "exquote-transactions":
		return runExQuoteTransactions(ctx, args[1:], stdout, stderr)
	case "exquote-history-transactions":
		return runExQuoteHistoryTransactions(ctx, args[1:], stdout, stderr)
	case "exquote-history-bars":
		return runExQuoteHistoryBars(ctx, args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

type symbolFlags []string

func (s *symbolFlags) String() string {
	return strings.Join(*s, ",")
}

func (s *symbolFlags) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			*s = append(*s, part)
		}
	}
	return nil
}

type listFlags []string

func (s *listFlags) String() string {
	return strings.Join(*s, ",")
}

func (s *listFlags) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			*s = append(*s, part)
		}
	}
	return nil
}

type bestIPFlags struct {
	Enabled   bool
	CachePath string
	MaxAge    time.Duration
	Refresh   bool
}

func registerBestIPFlags(fs *flag.FlagSet, flags *bestIPFlags) {
	fs.BoolVar(&flags.Enabled, "bestip", false, "use cached best TDX HQ servers when --server is omitted")
	fs.StringVar(&flags.CachePath, "bestip-cache", tdx.DefaultHQBestIPCachePath(), "bestip cache file path")
	fs.DurationVar(&flags.MaxAge, "bestip-max-age", tdx.DefaultHQBestIPMaxAge(), "maximum bestip cache age before refresh")
	fs.BoolVar(&flags.Refresh, "bestip-refresh", true, "refresh bestip cache when missing or stale")
}

func applyBestIPFlags(opts *tdx.QuoteClientOptions, flags bestIPFlags) {
	if !flags.Enabled {
		return
	}
	opts.BestIP = true
	opts.BestIPCachePath = flags.CachePath
	opts.BestIPMaxAge = flags.MaxAge
	opts.BestIPRefresh = flags.Refresh
}

func runQuote(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var symbols symbolFlags
	var servers listFlags
	var bestIP bestIPFlags
	var batchSize int
	var tradeDateText string
	fs := newFlagSet("quote", stderr)
	fs.Var(&symbols, "symbol", "symbol to quote; repeat or comma-separate, accepts code or market:code")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	registerBestIPFlags(fs, &bestIP)
	fs.IntVar(&batchSize, "batch-size", 0, "quote request batch size")
	fs.StringVar(&tradeDateText, "trade-date", "", "explicit trade date YYYY-MM-DD for quote_time")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(symbols) == 0 {
		fmt.Fprintln(stderr, "at least one --symbol is required")
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
	clientOpts, err := quoteClientOptions([]string(servers), batchSize, tradeDateText, bestIP)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	quotes, err := fetchRealtimeQuotes(ctx, requests, clientOpts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	encoded, err := json.MarshalIndent(quotes, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, string(encoded))
	return 0
}

func runQuoteProbe(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	fs := newFlagSet("quote-probe", stderr)
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	results := probeHQServers(ctx, []string(servers), tdx.QuoteClientOptions{})
	tdx.SortProbeResults(results)
	encoded, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, string(encoded))
	return 0
}

func runQuoteBestIP(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var cachePath string
	var maxAge time.Duration
	var watch bool
	var interval time.Duration
	fs := newFlagSet("quote-bestip", stderr)
	fs.Var(&servers, "server", "TDX HQ server host:port candidate; repeat or comma-separate")
	fs.StringVar(&cachePath, "cache", tdx.DefaultHQBestIPCachePath(), "bestip cache file path")
	fs.DurationVar(&maxAge, "max-age", tdx.DefaultHQBestIPMaxAge(), "cache validity duration")
	fs.BoolVar(&watch, "watch", false, "keep refreshing the bestip cache until interrupted")
	fs.DurationVar(&interval, "interval", 30*time.Minute, "refresh interval for --watch")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if maxAge <= 0 {
		fmt.Fprintln(stderr, "--max-age must be positive")
		return 2
	}
	if watch && interval <= 0 {
		fmt.Fprintln(stderr, "--interval must be positive")
		return 2
	}
	probeOnce := func() int {
		cache, err := refreshHQBestIPCache(ctx, []string(servers), tdx.QuoteClientOptions{
			BestIPCachePath: cachePath,
			BestIPMaxAge:    maxAge,
		})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return writeJSON(stdout, stderr, cache)
	}
	if !watch {
		return probeOnce()
	}
	for {
		if code := probeOnce(); code != 0 {
			return code
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return 0
		case <-timer.C:
		}
	}
}

func runQuoteSweep(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var symbols symbolFlags
	var servers listFlags
	var bestIP bestIPFlags
	var markets listFlags
	var batchSize int
	var limit int
	var tradeDateText string
	fs := newFlagSet("quote-sweep", stderr)
	fs.Var(&symbols, "symbol", "symbol to quote; repeat or comma-separate, accepts code or market:code")
	fs.Var(&markets, "market", "market to discover when no symbols are provided; repeat or comma-separate")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	registerBestIPFlags(fs, &bestIP)
	fs.IntVar(&batchSize, "batch-size", 0, "quote request batch size")
	fs.IntVar(&limit, "limit", 0, "maximum symbols to quote")
	fs.StringVar(&tradeDateText, "trade-date", "", "explicit trade date YYYY-MM-DD for quote_time")
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
	clientOpts, err := quoteClientOptions([]string(servers), batchSize, tradeDateText, bestIP)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	quotes, err := fetchQuoteSweep(ctx, tdx.QuoteSweepOptions{
		Requests: requests,
		Markets:  []string(markets),
		Limit:    limit,
		Client:   clientOpts,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	encoded, err := json.MarshalIndent(quotes, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, string(encoded))
	return 0
}

func runHQBars(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer, index bool) int {
	var servers listFlags
	var market string
	var symbol string
	var category int
	var start int
	var count int
	name := "hq-bars"
	if index {
		name = "hq-index-bars"
	}
	fs := newFlagSet(name, stderr)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.IntVar(&category, "category", tdx.HQKLineDayAlt, "K-line category: 0=5m, 1=15m, 2=30m, 3=1h, 4=day, 5=week, 6=month, 7=1m, 8=1m, 9=day, 10=quarter, 11=year")
	fs.IntVar(&start, "start", 0, "K-line start offset")
	fs.IntVar(&count, "count", tdx.DefaultHQKLineCount, "K-line count, max 800")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseHQBarsRequest(category, market, symbol, start, count)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var bars []tdx.HQBar
	if index {
		bars, err = fetchHQIndexBars(ctx, req, hqClientOptions([]string(servers)))
	} else {
		bars, err = fetchHQSecurityBars(ctx, req, hqClientOptions([]string(servers)))
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, bars)
}

func runHQMinute(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market string
	var symbol string
	fs := newFlagSet("hq-minute", stderr)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseHQMinuteRequest(market, symbol)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	points, err := fetchHQMinuteTime(ctx, req, hqClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, points)
}

func runHQHistoryMinute(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market string
	var symbol string
	var dateText string
	fs := newFlagSet("hq-history-minute", stderr)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.StringVar(&dateText, "date", "", "history date YYYYMMDD")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseHQMinuteRequest(market, symbol)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	date, err := parseYYYYMMDDFlag("date", dateText)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	points, err := fetchHQHistoryMinuteTime(ctx, req, date, hqClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, points)
}

func runHQTransactions(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market string
	var symbol string
	var start int
	var count int
	fs := newFlagSet("hq-transactions", stderr)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.IntVar(&start, "start", 0, "transaction start offset")
	fs.IntVar(&count, "count", tdx.DefaultHQTransactionCount, "transaction count, max 1800")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseHQTransactionRequest(market, symbol, start, count)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	transactions, err := fetchHQTransactions(ctx, req, start, count, hqClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, transactions)
}

func runHQHistoryTransactions(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market string
	var symbol string
	var dateText string
	var start int
	var count int
	fs := newFlagSet("hq-history-transactions", stderr)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.StringVar(&dateText, "date", "", "history date YYYYMMDD")
	fs.IntVar(&start, "start", 0, "transaction start offset")
	fs.IntVar(&count, "count", tdx.DefaultHQTransactionCount, "transaction count, max 1800")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseHQTransactionRequest(market, symbol, start, count)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	date, err := parseYYYYMMDDFlag("date", dateText)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	transactions, err := fetchHQHistoryTransactions(ctx, req, date, start, count, hqClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, transactions)
}

func runHQCompanyCategories(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market string
	var symbol string
	fs := newFlagSet("hq-company-categories", stderr)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseHQMinuteRequest(market, symbol)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	categories, err := fetchHQCompanyInfoCategories(ctx, req, hqClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, categories)
}

func runHQCompanyContent(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market string
	var symbol string
	var filename string
	var start uint
	var length uint
	fs := newFlagSet("hq-company-content", stderr)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.StringVar(&filename, "filename", "", "company info filename")
	fs.UintVar(&start, "start", 0, "content start offset")
	fs.UintVar(&length, "length", 0, "content length")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseHQMinuteRequest(market, symbol)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if length == 0 || start > uint(^uint32(0)) || length > uint(^uint32(0)) {
		fmt.Fprintln(stderr, "--length must be positive and offsets must fit uint32")
		return 2
	}
	content, err := fetchHQCompanyInfoContent(ctx, req, filename, uint32(start), uint32(length), hqClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, content)
}

func runHQXDXR(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market string
	var symbol string
	fs := newFlagSet("hq-xdxr", stderr)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseHQMinuteRequest(market, symbol)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	rows, err := fetchHQXDXRInfo(ctx, req, hqClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, rows)
}

func runHQFinance(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market string
	var symbol string
	fs := newFlagSet("hq-finance", stderr)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseHQMinuteRequest(market, symbol)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	info, err := fetchHQFinanceInfo(ctx, req, hqClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, info)
}

func runHQBlockMeta(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var file string
	fs := newFlagSet("hq-block-meta", stderr)
	fs.StringVar(&file, "file", "", "block file, such as block.dat, block_zs.dat, block_fg.dat, block_gn.dat")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(file) == "" {
		fmt.Fprintln(stderr, "--file is required")
		return 2
	}
	meta, err := fetchHQBlockMeta(ctx, file, hqClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, meta)
}

func runHQBlock(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var file string
	fs := newFlagSet("hq-block", stderr)
	fs.StringVar(&file, "file", "", "block file, such as block.dat, block_zs.dat, block_fg.dat, block_gn.dat")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(file) == "" {
		fmt.Fprintln(stderr, "--file is required")
		return 2
	}
	members, err := fetchHQBlockMembers(ctx, file, hqClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, members)
}

func runExQuoteMarkets(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	fs := newFlagSet("exquote-markets", stderr)
	fs.Var(&servers, "server", "TDX ExHQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	markets, err := fetchExMarkets(ctx, exQuoteClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	encoded, err := json.MarshalIndent(markets, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, string(encoded))
	return 0
}

func runExQuoteCount(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	fs := newFlagSet("exquote-count", stderr)
	fs.Var(&servers, "server", "TDX ExHQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	count, err := fetchExInstrumentCount(ctx, exQuoteClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, count)
}

func runExQuoteInstruments(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var start int
	var count int
	fs := newFlagSet("exquote-instruments", stderr)
	fs.IntVar(&start, "start", 0, "instrument list start offset")
	fs.IntVar(&count, "count", tdx.DefaultExInstrumentListCount, "instrument list count")
	fs.Var(&servers, "server", "TDX ExHQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if start < 0 || count <= 0 || count > tdx.MaxExInstrumentListCount {
		fmt.Fprintf(stderr, "--start must be non-negative and --count must be between 1 and %d\n", tdx.MaxExInstrumentListCount)
		return 2
	}
	instruments, err := fetchExInstruments(ctx, start, count, exQuoteClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, instruments)
}

func runExQuote(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market int
	var code string
	fs := newFlagSet("exquote", stderr)
	fs.IntVar(&market, "market", 0, "TDX ExHQ numeric market id")
	fs.StringVar(&code, "code", "", "TDX ExHQ instrument code")
	fs.Var(&servers, "server", "TDX ExHQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseExQuoteRequest(market, code)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	quote, err := fetchExQuote(ctx, req, exQuoteClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	encoded, err := json.MarshalIndent(quote, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, string(encoded))
	return 0
}

func runExQuoteBars(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market int
	var code string
	var category int
	var start int
	var count int
	fs := newFlagSet("exquote-bars", stderr)
	fs.IntVar(&market, "market", 0, "TDX ExHQ numeric market id")
	fs.StringVar(&code, "code", "", "TDX ExHQ instrument code")
	fs.IntVar(&category, "category", tdx.ExKLineDaily, "K-line category: 0=5m, 1=15m, 2=30m, 3=1h, 4=day, 5=week, 6=month, 7=ExHQ 1m, 8=1m, 9=day, 10=quarter, 11=year")
	fs.IntVar(&start, "start", 0, "K-line start offset")
	fs.IntVar(&count, "count", 100, "K-line count, max 800")
	fs.Var(&servers, "server", "TDX ExHQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseExBarsRequest(category, market, code, start, count)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	bars, err := fetchExBars(ctx, req, exQuoteClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, bars)
}

func runExQuoteMinute(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market int
	var code string
	fs := newFlagSet("exquote-minute", stderr)
	fs.IntVar(&market, "market", 0, "TDX ExHQ numeric market id")
	fs.StringVar(&code, "code", "", "TDX ExHQ instrument code")
	fs.Var(&servers, "server", "TDX ExHQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseExQuoteRequest(market, code)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	points, err := fetchExMinuteTime(ctx, req, exQuoteClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, points)
}

func runExQuoteHistoryMinute(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market int
	var code string
	var dateText string
	fs := newFlagSet("exquote-history-minute", stderr)
	fs.IntVar(&market, "market", 0, "TDX ExHQ numeric market id")
	fs.StringVar(&code, "code", "", "TDX ExHQ instrument code")
	fs.StringVar(&dateText, "date", "", "history date YYYYMMDD")
	fs.Var(&servers, "server", "TDX ExHQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseExQuoteRequest(market, code)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	date, err := parseYYYYMMDDFlag("date", dateText)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	points, err := fetchExHistoryMinuteTime(ctx, req, date, exQuoteClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, points)
}

func runExQuoteTransactions(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market int
	var code string
	var start int
	var count int
	fs := newFlagSet("exquote-transactions", stderr)
	fs.IntVar(&market, "market", 0, "TDX ExHQ numeric market id")
	fs.StringVar(&code, "code", "", "TDX ExHQ instrument code")
	fs.IntVar(&start, "start", 0, "transaction start offset")
	fs.IntVar(&count, "count", tdx.MaxExTransactionCount, "transaction count, max 1800")
	fs.Var(&servers, "server", "TDX ExHQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseExQuoteRequest(market, code)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if start < 0 || count <= 0 || count > tdx.MaxExTransactionCount {
		fmt.Fprintf(stderr, "--start must be non-negative and --count must be between 1 and %d\n", tdx.MaxExTransactionCount)
		return 2
	}
	transactions, err := fetchExTransactions(ctx, req, start, count, exQuoteClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, transactions)
}

func runExQuoteHistoryTransactions(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market int
	var code string
	var dateText string
	var start int
	var count int
	fs := newFlagSet("exquote-history-transactions", stderr)
	fs.IntVar(&market, "market", 0, "TDX ExHQ numeric market id")
	fs.StringVar(&code, "code", "", "TDX ExHQ instrument code")
	fs.StringVar(&dateText, "date", "", "history date YYYYMMDD")
	fs.IntVar(&start, "start", 0, "transaction start offset")
	fs.IntVar(&count, "count", tdx.MaxExTransactionCount, "transaction count, max 1800")
	fs.Var(&servers, "server", "TDX ExHQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseExQuoteRequest(market, code)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	date, err := parseYYYYMMDDFlag("date", dateText)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if start < 0 || count <= 0 || count > tdx.MaxExTransactionCount {
		fmt.Fprintf(stderr, "--start must be non-negative and --count must be between 1 and %d\n", tdx.MaxExTransactionCount)
		return 2
	}
	transactions, err := fetchExHistoryTransactions(ctx, req, date, start, count, exQuoteClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, transactions)
}

func runExQuoteHistoryBars(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var servers listFlags
	var market int
	var code string
	var startDateText string
	var endDateText string
	fs := newFlagSet("exquote-history-bars", stderr)
	fs.IntVar(&market, "market", 0, "TDX ExHQ numeric market id")
	fs.StringVar(&code, "code", "", "TDX ExHQ instrument code")
	fs.StringVar(&startDateText, "start-date", "", "range start date YYYYMMDD")
	fs.StringVar(&endDateText, "end-date", "", "range end date YYYYMMDD")
	fs.Var(&servers, "server", "TDX ExHQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	req, err := tdx.ParseExQuoteRequest(market, code)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	startDate, err := parseYYYYMMDDFlag("start-date", startDateText)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	endDate, err := parseYYYYMMDDFlag("end-date", endDateText)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if startDate > endDate {
		fmt.Fprintln(stderr, "--start-date must be <= --end-date")
		return 2
	}
	bars, err := fetchExHistoryBarsRange(ctx, req, startDate, endDate, exQuoteClientOptions([]string(servers)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, bars)
}

func quoteClientOptions(servers []string, batchSize int, tradeDateText string, bestIP bestIPFlags) (tdx.QuoteClientOptions, error) {
	opts := tdx.QuoteClientOptions{
		Servers:   servers,
		BatchSize: batchSize,
	}
	applyBestIPFlags(&opts, bestIP)
	if tradeDateText == "" {
		return opts, nil
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return opts, err
	}
	tradeDate, err := time.ParseInLocation("2006-01-02", tradeDateText, loc)
	if err != nil {
		return opts, fmt.Errorf("parse --trade-date: %w", err)
	}
	opts.TradeDate = tradeDate
	opts.Location = loc
	return opts, nil
}

func exQuoteClientOptions(servers []string) tdx.ExQuoteClientOptions {
	return tdx.ExQuoteClientOptions{Servers: servers}
}

func hqClientOptions(servers []string) tdx.QuoteClientOptions {
	return tdx.QuoteClientOptions{Servers: servers}
}

func parseYYYYMMDDFlag(name, value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("--%s is required", name)
	}
	if len(value) != 8 {
		return 0, fmt.Errorf("--%s must be YYYYMMDD", name)
	}
	if _, err := time.Parse("20060102", value); err != nil {
		return 0, fmt.Errorf("parse --%s: %w", name, err)
	}
	date, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse --%s: %w", name, err)
	}
	return date, nil
}

func writeJSON(stdout io.Writer, stderr io.Writer, value any) int {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, string(encoded))
	return 0
}

func runBootstrap(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var dryRun bool
	fs := newFlagSet("bootstrap", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.BoolVar(&dryRun, "dry-run", false, "print DDL without executing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := config.Load(overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cleanup, ok := setupLogging(cfg, stderr)
	if !ok {
		return 1
	}
	defer cleanup()
	if dryRun {
		ddl, err := chstore.BootstrapDDL(chstore.SchemaConfig{MarketDB: cfg.ClickHouse.Databases.Market, OpsDB: cfg.ClickHouse.Databases.Ops})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		for _, stmt := range ddl {
			fmt.Fprintln(stdout, stmt+";")
		}
		return 0
	}
	store, err := chstore.Open(ctx, cfg.ClickHouse)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer store.Close()
	if err := store.Bootstrap(ctx); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "bootstrap ok")
	return 0
}

func runStatus(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	fs := newFlagSet("status", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := config.Load(overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cleanup, ok := setupLogging(cfg, stderr)
	if !ok {
		return 1
	}
	defer cleanup()
	store, err := chstore.Open(ctx, cfg.ClickHouse)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer store.Close()
	fmt.Fprintln(stdout, "clickhouse: ok")
	watermarks, err := store.LatestWatermarks(ctx, 20)
	if err != nil {
		fmt.Fprintf(stderr, "watermarks: %v\n", err)
		return 1
	}
	if len(watermarks) == 0 {
		fmt.Fprintln(stdout, "watermarks: none")
		return 0
	}
	for _, wm := range watermarks {
		fmt.Fprintf(stdout, "%s %s %s %s %s\n", wm.Dataset, wm.Asset, wm.Status, wm.Updated.Format("2006-01-02 15:04:05"), wm.Message)
	}
	return 0
}

func runImport(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer, period tdx.Period) int {
	var overrides config.Overrides
	var file, code, market, since, until string
	var dryRun bool
	fs := newFlagSet("import", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&file, "file", "", "input file")
	fs.StringVar(&code, "code", "", "six-digit symbol")
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&since, "since", "", "start date/time")
	fs.StringVar(&until, "until", "", "end date/time")
	fs.BoolVar(&dryRun, "dry-run", false, "parse and summarize without writing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := config.Load(overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cleanup, ok := setupLogging(cfg, stderr)
	if !ok {
		return 1
	}
	defer cleanup()
	var store *chstore.Store
	if !dryRun {
		store, err = chstore.Open(ctx, cfg.ClickHouse)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer store.Close()
	}
	if file == "" && code == "" {
		return runBulkImport(ctx, stdout, stderr, bulkImportOptions{
			Period:    period,
			Root:      cfg.TDX.Root,
			Market:    market,
			Since:     since,
			Until:     until,
			DryRun:    dryRun,
			Store:     store,
			Timezone:  cfg.Runtime.Timezone,
			BatchSize: cfg.Runtime.BatchSize,
		})
	}
	summary, err := ingest.Import(ctx, ingest.ImportOptions{
		Period:    period,
		File:      file,
		Root:      cfg.TDX.Root,
		Code:      code,
		Market:    market,
		Since:     since,
		Until:     until,
		DryRun:    dryRun,
		Store:     store,
		Timezone:  cfg.Runtime.Timezone,
		BatchSize: cfg.Runtime.BatchSize,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printSummary(stdout, summary)
	return 0
}

func runImportVIPDocZip(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var file, periodText, market, since, until string
	var dryRun bool
	fs := newFlagSet("import-tdx-vipdoc-zip", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&file, "file", "", "vipdoc zip file")
	fs.StringVar(&periodText, "period", "all", "period to import: all, 1m, or 5m")
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&since, "since", "", "start date/time")
	fs.StringVar(&until, "until", "", "end date/time")
	fs.BoolVar(&dryRun, "dry-run", false, "parse and summarize without writing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(file) == "" {
		fmt.Fprintln(stderr, "--file is required")
		return 2
	}
	periods, err := parseVIPDocZipPeriods(periodText)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	cfg, err := config.Load(overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cleanup, ok := setupLogging(cfg, stderr)
	if !ok {
		return 1
	}
	defer cleanup()
	var store *chstore.Store
	if !dryRun {
		store, err = chstore.Open(ctx, cfg.ClickHouse)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer store.Close()
	}
	summary, err := ingest.ImportVIPDocZip(ctx, ingest.VIPDocZipOptions{
		ZipPath:   file,
		Periods:   periods,
		Market:    market,
		Since:     since,
		Until:     until,
		DryRun:    dryRun,
		Store:     store,
		Timezone:  cfg.Runtime.Timezone,
		BatchSize: cfg.Runtime.BatchSize,
		Progress: func(processed int, summary ingest.VIPDocZipSummary) {
			if processed == 1 || processed%100 == 0 || processed == summary.Files {
				fmt.Fprintf(stderr, "processed %d/%d files, rows_1m=%d, rows_5m=%d, issues=%d\n", processed, summary.Files, summary.Rows1m, summary.Rows5m, summary.QualityIssues)
			}
		},
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printVIPDocZipSummary(stdout, summary)
	return 0
}

func runImportTDXFinancial(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var file string
	var dryRun bool
	fs := newFlagSet("import-tdx-fin", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&file, "file", "", "tdxfin zip file")
	fs.BoolVar(&dryRun, "dry-run", false, "parse and summarize without writing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(file) == "" {
		fmt.Fprintln(stderr, "--file is required")
		return 2
	}
	cfg, err := config.Load(overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cleanup, ok := setupLogging(cfg, stderr)
	if !ok {
		return 1
	}
	defer cleanup()
	var store *chstore.Store
	if !dryRun {
		store, err = chstore.Open(ctx, cfg.ClickHouse)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer store.Close()
	}
	summary, err := ingest.ImportTDXFinancial(ctx, ingest.TDXFinancialOptions{
		File:      file,
		DryRun:    dryRun,
		Store:     store,
		Timezone:  cfg.Runtime.Timezone,
		BatchSize: cfg.Runtime.BatchSize,
		Progress: func(processed int, total int, summary ingest.TDXFinancialSummary) {
			if processed == 1 || processed%10 == 0 || processed == total {
				fmt.Fprintf(stderr, "processed %d/%d files, rows=%d, issues=%d\n", processed, total, summary.RowsWritten, summary.QualityIssues)
			}
		},
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printTDXFinancialSummary(stdout, summary)
	return 0
}

func runImportTDXGP(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var file string
	var dryRun bool
	fs := newFlagSet("import-tdx-gp", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&file, "file", "", "tdxgp zip file")
	fs.BoolVar(&dryRun, "dry-run", false, "parse and summarize without writing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(file) == "" {
		fmt.Fprintln(stderr, "--file is required")
		return 2
	}
	cfg, err := config.Load(overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cleanup, ok := setupLogging(cfg, stderr)
	if !ok {
		return 1
	}
	defer cleanup()
	var store *chstore.Store
	if !dryRun {
		store, err = chstore.Open(ctx, cfg.ClickHouse)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer store.Close()
	}
	summary, err := ingest.ImportTDXGP(ctx, ingest.TDXGPOptions{
		File:      file,
		DryRun:    dryRun,
		Store:     store,
		Timezone:  cfg.Runtime.Timezone,
		BatchSize: cfg.Runtime.BatchSize,
		Progress: func(processed int, total int, summary ingest.TDXGPSummary) {
			if processed == 1 || processed%500 == 0 || processed == total {
				fmt.Fprintf(stderr, "processed %d/%d files, rows=%d, issues=%d\n", processed, total, summary.RowsWritten, summary.QualityIssues)
			}
		},
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printTDXFinancialSummary(stdout, summary)
	return 0
}

type bulkImportOptions struct {
	Period    tdx.Period
	Root      string
	Market    string
	Since     string
	Until     string
	DryRun    bool
	Store     *chstore.Store
	Timezone  string
	BatchSize int
}

type bulkImportSummary struct {
	DryRun        bool
	Dataset       string
	TargetTable   string
	Root          string
	Files         int
	RowsWritten   uint64
	RowsSkipped   uint64
	QualityIssues int
}

func runBulkImport(ctx context.Context, stdout io.Writer, stderr io.Writer, opts bulkImportOptions) int {
	root := expandHome(opts.Root)
	files, err := tdx.DiscoverFiles(root, opts.Period, opts.Market)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if !opts.DryRun && opts.Period == tdx.PeriodDay {
		summary, err := ingest.ImportDailyBulk(ctx, ingest.BulkOptions{
			Period:    opts.Period,
			Files:     files,
			Market:    opts.Market,
			Since:     opts.Since,
			Until:     opts.Until,
			Store:     opts.Store,
			Timezone:  opts.Timezone,
			BatchSize: opts.BatchSize,
			Progress: func(processed int, summary ingest.BulkSummary) {
				if processed == 1 || processed%100 == 0 || processed == len(files) {
					fmt.Fprintf(stderr, "processed %d/%d files, rows=%d, issues=%d\n", processed, len(files), summary.RowsWritten, summary.QualityIssues)
				}
			},
		})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		printBulkSummary(stdout, bulkImportSummary{
			DryRun:        false,
			Dataset:       summary.Dataset,
			TargetTable:   summary.TargetTable,
			Root:          root,
			Files:         summary.Files,
			RowsWritten:   summary.RowsWritten,
			RowsSkipped:   summary.RowsSkipped,
			QualityIssues: summary.QualityIssues,
		})
		return 0
	}
	bulk := bulkImportSummary{
		DryRun:      opts.DryRun,
		Dataset:     datasetFor(opts.Period),
		TargetTable: datasetFor(opts.Period),
		Root:        root,
		Files:       len(files),
	}
	for i, file := range files {
		summary, err := ingest.Import(ctx, ingest.ImportOptions{
			Period:    opts.Period,
			File:      file,
			Market:    opts.Market,
			Since:     opts.Since,
			Until:     opts.Until,
			DryRun:    opts.DryRun,
			Store:     opts.Store,
			Timezone:  opts.Timezone,
			BatchSize: opts.BatchSize,
		})
		if err != nil {
			fmt.Fprintf(stderr, "import %s: %v\n", file, err)
			return 1
		}
		bulk.RowsWritten += summary.RowsWritten
		bulk.RowsSkipped += summary.RowsSkipped
		bulk.QualityIssues += len(summary.Issues)
		if i == 0 || (i+1)%100 == 0 || i+1 == len(files) {
			fmt.Fprintf(stderr, "processed %d/%d files, rows=%d, issues=%d\n", i+1, len(files), bulk.RowsWritten, bulk.QualityIssues)
		}
	}
	printBulkSummary(stdout, bulk)
	return 0
}

func printSummary(out io.Writer, summary ingest.Summary) {
	mode := "write"
	if summary.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(out, "run_id: %s\n", summary.RunID)
	fmt.Fprintf(out, "mode: %s\n", mode)
	fmt.Fprintf(out, "dataset: %s\n", summary.Dataset)
	fmt.Fprintf(out, "target_table: %s\n", summary.TargetTable)
	fmt.Fprintf(out, "input_path: %s\n", summary.InputPath)
	fmt.Fprintf(out, "input_format: %s\n", summary.InputFormat)
	fmt.Fprintf(out, "rows_written: %d\n", summary.RowsWritten)
	fmt.Fprintf(out, "rows_skipped: %d\n", summary.RowsSkipped)
	fmt.Fprintf(out, "quality_issues: %d\n", len(summary.Issues))
	for _, issue := range summary.Issues {
		parts := []string{issue.Severity, issue.IssueType}
		if issue.LogicalKey != "" {
			parts = append(parts, issue.LogicalKey)
		}
		if issue.Message != "" {
			parts = append(parts, issue.Message)
		}
		fmt.Fprintf(out, "- %s\n", strings.Join(parts, " "))
	}
}

func printBulkSummary(out io.Writer, summary bulkImportSummary) {
	mode := "write"
	if summary.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(out, "mode: %s\n", mode)
	fmt.Fprintf(out, "dataset: %s\n", summary.Dataset)
	fmt.Fprintf(out, "target_table: %s\n", summary.TargetTable)
	fmt.Fprintf(out, "root: %s\n", summary.Root)
	fmt.Fprintf(out, "files: %d\n", summary.Files)
	fmt.Fprintf(out, "rows_written: %d\n", summary.RowsWritten)
	fmt.Fprintf(out, "rows_skipped: %d\n", summary.RowsSkipped)
	fmt.Fprintf(out, "quality_issues: %d\n", summary.QualityIssues)
}

func printVIPDocZipSummary(out io.Writer, summary ingest.VIPDocZipSummary) {
	mode := "write"
	if summary.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(out, "mode: %s\n", mode)
	fmt.Fprintf(out, "zip_path: %s\n", summary.ZipPath)
	fmt.Fprintf(out, "files: %d\n", summary.Files)
	fmt.Fprintf(out, "files_1m: %d\n", summary.Files1m)
	fmt.Fprintf(out, "files_5m: %d\n", summary.Files5m)
	fmt.Fprintf(out, "rows_written: %d\n", summary.RowsWritten)
	fmt.Fprintf(out, "rows_1m: %d\n", summary.Rows1m)
	fmt.Fprintf(out, "rows_5m: %d\n", summary.Rows5m)
	fmt.Fprintf(out, "rows_skipped: %d\n", summary.RowsSkipped)
	fmt.Fprintf(out, "quality_issues: %d\n", summary.QualityIssues)
}

func printTDXFinancialSummary(out io.Writer, summary ingest.TDXFinancialSummary) {
	mode := "write"
	if summary.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(out, "mode: %s\n", mode)
	fmt.Fprintf(out, "dataset: %s\n", summary.Dataset)
	fmt.Fprintf(out, "target_table: %s\n", summary.TargetTable)
	fmt.Fprintf(out, "input_path: %s\n", summary.InputPath)
	fmt.Fprintf(out, "input_format: %s\n", summary.InputFormat)
	fmt.Fprintf(out, "files_discovered: %d\n", summary.FilesDiscovered)
	fmt.Fprintf(out, "files_processed: %d\n", summary.FilesProcessed)
	fmt.Fprintf(out, "manifest_files: %d\n", summary.ManifestFiles)
	fmt.Fprintf(out, "dictionary_count: %d\n", summary.DictionaryCount)
	fmt.Fprintf(out, "rows_written: %d\n", summary.RowsWritten)
	fmt.Fprintf(out, "rows_skipped: %d\n", summary.RowsSkipped)
	fmt.Fprintf(out, "manifest_issues: %d\n", summary.ManifestIssues)
	fmt.Fprintf(out, "quality_issues: %d\n", summary.QualityIssues)
}

func setupLogging(cfg config.Config, stderr io.Writer) (func(), bool) {
	_, cleanup, err := logging.InitGlobal(cfg.Logging)
	if err != nil {
		fmt.Fprintf(stderr, "logging: %v\n", err)
		return nil, false
	}
	return func() {
		if err := cleanup(); err != nil {
			fmt.Fprintf(stderr, "logging cleanup: %v\n", err)
		}
	}, true
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, "usage: marketd <command> [flags]")
	fmt.Fprintln(out, "commands:")
	fmt.Fprintln(out, "  bootstrap")
	fmt.Fprintln(out, "  status")
	fmt.Fprintln(out, "  import-tdx-day")
	fmt.Fprintln(out, "  import-tdx-1m")
	fmt.Fprintln(out, "  import-tdx-5m")
	fmt.Fprintln(out, "  import-tdx-vipdoc-zip")
	fmt.Fprintln(out, "  import-tdx-fin")
	fmt.Fprintln(out, "  import-tdx-gp")
	fmt.Fprintln(out, "  quote")
	fmt.Fprintln(out, "  quote-probe")
	fmt.Fprintln(out, "  quote-bestip")
	fmt.Fprintln(out, "  quote-sweep")
	fmt.Fprintln(out, "  hq-bars")
	fmt.Fprintln(out, "  hq-index-bars")
	fmt.Fprintln(out, "  hq-minute")
	fmt.Fprintln(out, "  hq-history-minute")
	fmt.Fprintln(out, "  hq-transactions")
	fmt.Fprintln(out, "  hq-history-transactions")
	fmt.Fprintln(out, "  hq-company-categories")
	fmt.Fprintln(out, "  hq-company-content")
	fmt.Fprintln(out, "  hq-xdxr")
	fmt.Fprintln(out, "  hq-finance")
	fmt.Fprintln(out, "  hq-block-meta")
	fmt.Fprintln(out, "  hq-block")
	fmt.Fprintln(out, "  quote-serve")
	fmt.Fprintln(out, "  quote-status")
	fmt.Fprintln(out, "  exquote-markets")
	fmt.Fprintln(out, "  exquote-count")
	fmt.Fprintln(out, "  exquote-instruments")
	fmt.Fprintln(out, "  exquote")
	fmt.Fprintln(out, "  exquote-bars")
	fmt.Fprintln(out, "  exquote-minute")
	fmt.Fprintln(out, "  exquote-history-minute")
	fmt.Fprintln(out, "  exquote-transactions")
	fmt.Fprintln(out, "  exquote-history-transactions")
	fmt.Fprintln(out, "  exquote-history-bars")
}

func datasetFor(period tdx.Period) string {
	switch period {
	case tdx.PeriodDay:
		return "a_share_bars_1d"
	case tdx.Period1m:
		return "a_share_bars_1m"
	case tdx.Period5m:
		return "a_share_bars_5m"
	default:
		return string(period)
	}
}

func parseVIPDocZipPeriods(value string) ([]tdx.Period, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "all":
		return []tdx.Period{tdx.Period1m, tdx.Period5m}, nil
	case "1m":
		return []tdx.Period{tdx.Period1m}, nil
	case "5m":
		return []tdx.Period{tdx.Period5m}, nil
	default:
		return nil, fmt.Errorf("--period must be all, 1m, or 5m")
	}
}

func expandHome(path string) string {
	if path == "" || path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
