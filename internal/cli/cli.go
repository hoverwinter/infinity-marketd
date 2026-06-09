package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/adjust"
	chstore "github.com/hoverwinter/infinity-marketd/internal/clickhouse"
	"github.com/hoverwinter/infinity-marketd/internal/config"
	"github.com/hoverwinter/infinity-marketd/internal/derived"
	"github.com/hoverwinter/infinity-marketd/internal/ingest"
	"github.com/hoverwinter/infinity-marketd/internal/logging"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/securitymaster"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
	"github.com/hoverwinter/infinity-marketd/internal/tdx/finance"
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
var fetchHQSecurityList = tdx.FetchSecurityList
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
var openSecurityMasterStore = securitymaster.Open
var bootstrapSecurityMaster = securitymaster.Bootstrap
var refreshSecurityMaster = securitymaster.Refresh

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
	case "tdx-fin-files":
		return runTDXFinFiles(ctx, args[1:], stdout, stderr)
	case "tdx-fin-fetch":
		return runTDXFinFetch(ctx, args[1:], stdout, stderr)
	case "tdx-fin-parse":
		return runTDXFinParse(ctx, args[1:], stdout, stderr)
	case "import-tdx-intraday-points":
		return runImportIntradayPoints(ctx, args[1:], stdout, stderr)
	case "import-tdx-gbbq":
		return runImportGBBQ(ctx, args[1:], stdout, stderr)
	case "import-tdx-block":
		return runImportTDXBlock(ctx, args[1:], stdout, stderr)
	case "import-tdx-ex-daily":
		return runImportExDaily(ctx, args[1:], stdout, stderr)
	case "write-tdx-custom-block":
		return runWriteCustomBlock(args[1:], stdout, stderr)
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
	case "refresh-tdx-xdxr":
		return runRefreshTDXXDXR(ctx, args[1:], stdout, stderr)
	case "refresh-adjust-factors":
		return runRefreshAdjustFactors(ctx, args[1:], stdout, stderr)
	case "refresh-daily-derived":
		return runRefreshDailyDerived(ctx, args[1:], stdout, stderr)
	case "refresh-minute-scan":
		return runRefreshMinuteScan(ctx, args[1:], stdout, stderr)
	case "refresh-security-master":
		return runRefreshSecurityMaster(ctx, args[1:], stdout, stderr)
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
	case "hq-quotes-list":
		return runHQQuotesList(ctx, args[1:], stdout, stderr)
	case "hq-top-board":
		return runHQTopBoard(ctx, args[1:], stdout, stderr)
	case "hq-lhb":
		return runHQLHB(ctx, args[1:], stdout, stderr)
	case "sp-board-members":
		return runSPBoardMembers(ctx, args[1:], stdout, stderr)
	case "fund-kline":
		return runFundKline(ctx, args[1:], stdout, stderr)
	case "fund-detail":
		return runFundDetail(ctx, args[1:], stdout, stderr)
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

func registerConfigPathFlag(fs *flag.FlagSet, overrides *config.Overrides) {
	fs.StringVar(&overrides.ConfigPath, "config", "", "config file path")
}

func configuredHQServers(explicit []string, overrides config.Overrides) ([]string, error) {
	if len(explicit) > 0 || overrides.ConfigPath == "" {
		return explicit, nil
	}
	cfg, err := config.Load(overrides)
	if err != nil {
		return nil, err
	}
	return append([]string(nil), cfg.TDX.HQServers...), nil
}

func configuredExHQServers(explicit []string, overrides config.Overrides) ([]string, error) {
	if len(explicit) > 0 || overrides.ConfigPath == "" {
		return explicit, nil
	}
	cfg, err := config.Load(overrides)
	if err != nil {
		return nil, err
	}
	return append([]string(nil), cfg.TDX.ExHQServers...), nil
}

func runQuote(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var symbols symbolFlags
	var servers listFlags
	var bestIP bestIPFlags
	var batchSize int
	var tradeDateText string
	fs := newFlagSet("quote", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	clientOpts, err := quoteClientOptions(serverList, batchSize, tradeDateText, bestIP)
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
	var overrides config.Overrides
	var servers listFlags
	fs := newFlagSet("quote-probe", stderr)
	registerConfigPathFlag(fs, &overrides)
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	serverList, err := configuredHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	results := probeHQServers(ctx, serverList, tdx.QuoteClientOptions{})
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
	var overrides config.Overrides
	var servers listFlags
	var cachePath string
	var maxAge time.Duration
	var watch bool
	var interval time.Duration
	fs := newFlagSet("quote-bestip", stderr)
	registerConfigPathFlag(fs, &overrides)
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
		serverList, err := configuredHQServers([]string(servers), overrides)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		cache, err := refreshHQBestIPCache(ctx, serverList, tdx.QuoteClientOptions{
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
	var overrides config.Overrides
	var symbols symbolFlags
	var servers listFlags
	var bestIP bestIPFlags
	var markets listFlags
	var batchSize int
	var limit int
	var tradeDateText string
	fs := newFlagSet("quote-sweep", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	clientOpts, err := quoteClientOptions(serverList, batchSize, tradeDateText, bestIP)
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
	var overrides config.Overrides
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
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var bars []tdx.HQBar
	if index {
		bars, err = fetchHQIndexBars(ctx, req, hqClientOptions(serverList))
	} else {
		bars, err = fetchHQSecurityBars(ctx, req, hqClientOptions(serverList))
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, bars)
}

func runHQMinute(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var market string
	var symbol string
	fs := newFlagSet("hq-minute", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	points, err := fetchHQMinuteTime(ctx, req, hqClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, points)
}

func runHQHistoryMinute(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var market string
	var symbol string
	var dateText string
	fs := newFlagSet("hq-history-minute", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	points, err := fetchHQHistoryMinuteTime(ctx, req, date, hqClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, points)
}

func runHQTransactions(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var market string
	var symbol string
	var start int
	var count int
	fs := newFlagSet("hq-transactions", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	transactions, err := fetchHQTransactions(ctx, req, start, count, hqClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, transactions)
}

func runHQHistoryTransactions(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var market string
	var symbol string
	var dateText string
	var start int
	var count int
	fs := newFlagSet("hq-history-transactions", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	transactions, err := fetchHQHistoryTransactions(ctx, req, date, start, count, hqClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, transactions)
}

func runHQCompanyCategories(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var market string
	var symbol string
	fs := newFlagSet("hq-company-categories", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	categories, err := fetchHQCompanyInfoCategories(ctx, req, hqClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, categories)
}

func runHQCompanyContent(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var market string
	var symbol string
	var filename string
	var start uint
	var length uint
	fs := newFlagSet("hq-company-content", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	content, err := fetchHQCompanyInfoContent(ctx, req, filename, uint32(start), uint32(length), hqClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, content)
}

func runHQXDXR(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var market string
	var symbol string
	fs := newFlagSet("hq-xdxr", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	rows, err := fetchHQXDXRInfo(ctx, req, hqClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, rows)
}

type refreshSummary struct {
	RunID         string
	Dataset       string
	TargetTable   string
	Asset         string
	Params        string
	MinWatermark  *time.Time
	MaxWatermark  *time.Time
	RowsWritten   uint64
	QualityIssues int
	DryRun        bool
}

func runRefreshSecurityMaster(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var markets listFlags
	var servers listFlags
	var sourceName string
	var filePath string
	var dryRun bool
	fs := newFlagSet("refresh-security-master", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&sourceName, "source", securitymaster.SourceTDX, "security master source: tdx or file")
	fs.Var(&markets, "market", "market to refresh; repeat or comma-separate, defaults to sh,sz,bj")
	fs.Var(&servers, "server", "TDX HQ server host:port for --source tdx; repeat or comma-separate")
	fs.StringVar(&filePath, "file", "", "normalized CSV file for --source file")
	fs.BoolVar(&dryRun, "dry-run", false, "fetch and normalize without writing MySQL")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	sourceName = strings.ToLower(strings.TrimSpace(sourceName))
	var source securitymaster.Source
	switch sourceName {
	case securitymaster.SourceTDX:
		serverList, err := configuredHQServers([]string(servers), overrides)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		source = securitymaster.NewTDXSource(fetchHQSecurityList, hqClientOptions(serverList))
	case securitymaster.SourceFile:
		if strings.TrimSpace(filePath) == "" {
			fmt.Fprintln(stderr, "--file is required for --source file")
			return 2
		}
		source = securitymaster.FileSource{Path: filePath, Source: securitymaster.SourceFile}
	default:
		fmt.Fprintln(stderr, "--source must be tdx or file")
		return 2
	}
	var store securitymaster.Writer
	var closeStore func() error
	if !dryRun {
		cfg, err := config.Load(overrides)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := cfg.MySQL.RequiredError(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		cleanup, ok := setupLogging(cfg, stderr)
		if !ok {
			return 1
		}
		defer cleanup()
		securityStore, err := openSecurityMasterStore(ctx, cfg.MySQL)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		store = securityStore
		closeStore = securityStore.Close
		defer closeStore()
	}
	summary, err := refreshSecurityMaster(ctx, securitymaster.RefreshOptions{
		SourceName: sourceName,
		Markets:    []string(markets),
		DryRun:     dryRun,
		Source:     source,
		Store:      store,
	})
	if err != nil {
		if summary.Error != "" {
			_ = writeJSON(stdout, stderr, summary)
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, summary)
}

func runRefreshTDXXDXR(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var market string
	var symbol string
	var all bool
	var limit int
	var offset int
	var workers int
	var dryRun bool
	fs := newFlagSet("refresh-tdx-xdxr", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.BoolVar(&all, "all", false, "refresh xdxr for all symbols present in daily bars")
	fs.IntVar(&limit, "limit", 0, "maximum symbols to process in --all mode")
	fs.IntVar(&offset, "offset", 0, "symbols to skip in --all mode")
	fs.IntVar(&workers, "workers", 8, "concurrent TDX requests in --all mode")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	fs.BoolVar(&dryRun, "dry-run", false, "fetch and normalize without writing ClickHouse")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if all {
		return runRefreshTDXXDXRAll(ctx, stdout, stderr, overrides, market, []string(servers), limit, offset, workers, dryRun)
	}
	req, err := tdx.ParseHQMinuteRequest(market, symbol)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	runID := newLocalRunID()
	started := time.Now()
	serverList, err := configuredHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	rows, err := fetchHQXDXRInfo(ctx, req, hqClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	events, adjustIssues := adjust.NormalizeXDXR(rows)
	summary := refreshSummary{
		RunID:         runID,
		Dataset:       "a_share_xdxr_events",
		TargetTable:   "a_share_xdxr_events",
		Asset:         req.Market + ":" + req.Symbol,
		MinWatermark:  xdxrMinDate(events),
		MaxWatermark:  xdxrMaxDate(events),
		RowsWritten:   uint64(len(events)),
		QualityIssues: len(adjustIssues),
		DryRun:        dryRun,
	}
	if dryRun {
		printRefreshSummary(stdout, summary)
		return 0
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
	if err := store.InsertXDXREvents(ctx, events); err != nil {
		recordRefreshFailure(ctx, store, summary, "tdx_xdxr_refresh", "tdx.hq.xdxr", started, err)
		fmt.Fprintln(stderr, err)
		return 1
	}
	qualityIssues := qualityIssuesFromAdjust(runID, summary.Dataset, adjustIssues)
	if err := store.InsertQualityIssues(ctx, qualityIssues); err != nil {
		recordRefreshFailure(ctx, store, summary, "tdx_xdxr_refresh", "tdx.hq.xdxr", started, err)
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := recordRefreshSuccess(ctx, store, summary, "tdx_xdxr_refresh", "tdx.hq.xdxr", started, "ok"); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printRefreshSummary(stdout, summary)
	return 0
}

func runRefreshAdjustFactors(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var market string
	var symbol string
	var all bool
	var limit int
	var offset int
	var dryRun bool
	fs := newFlagSet("refresh-adjust-factors", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.BoolVar(&all, "all", false, "refresh adjustment factors for all symbols present in daily bars")
	fs.IntVar(&limit, "limit", 0, "maximum symbols to process in --all mode")
	fs.IntVar(&offset, "offset", 0, "symbols to skip in --all mode")
	fs.BoolVar(&dryRun, "dry-run", false, "calculate factors without writing ClickHouse")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if all {
		return runRefreshAdjustFactorsAll(ctx, stdout, stderr, overrides, market, limit, offset, dryRun)
	}
	req, err := tdx.ParseHQMinuteRequest(market, symbol)
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
	store, err := chstore.Open(ctx, cfg.ClickHouse)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer store.Close()
	runID := newLocalRunID()
	started := time.Now()
	bars, err := store.DailyBarsForSymbol(ctx, req.Market, req.Symbol)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	events, err := store.XDXREventsForSymbol(ctx, req.Market, req.Symbol)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	factors, adjustIssues := adjust.GenerateFactors(bars, events, time.Now())
	summary := refreshSummary{
		RunID:         runID,
		Dataset:       "a_share_adjust_factors_1d",
		TargetTable:   "a_share_adjust_factors_1d",
		Asset:         req.Market + ":" + req.Symbol,
		MinWatermark:  factorMinDate(factors),
		MaxWatermark:  factorMaxDate(factors),
		RowsWritten:   uint64(len(factors)),
		QualityIssues: len(adjustIssues),
		DryRun:        dryRun,
	}
	if dryRun {
		printRefreshSummary(stdout, summary)
		return 0
	}
	if err := store.InsertAdjustFactors(ctx, factors); err != nil {
		recordRefreshFailure(ctx, store, summary, "adjust_factor_refresh", "derived.adjust_factors_1d", started, err)
		fmt.Fprintln(stderr, err)
		return 1
	}
	qualityIssues := qualityIssuesFromAdjust(runID, summary.Dataset, adjustIssues)
	if err := store.InsertQualityIssues(ctx, qualityIssues); err != nil {
		recordRefreshFailure(ctx, store, summary, "adjust_factor_refresh", "derived.adjust_factors_1d", started, err)
		fmt.Fprintln(stderr, err)
		return 1
	}
	message := "ok"
	if len(adjustIssues) > 0 {
		message = "completed with quality issues"
	}
	if err := recordRefreshSuccess(ctx, store, summary, "adjust_factor_refresh", "derived.adjust_factors_1d", started, message); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printRefreshSummary(stdout, summary)
	return 0
}

func runRefreshMinuteScan(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var periodText string
	var sinceText string
	var untilText string
	var dryRun bool
	fs := newFlagSet("refresh-minute-scan", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&periodText, "period", "", "minute period 1m or 5m")
	fs.StringVar(&sinceText, "since", "", "start trade date YYYY-MM-DD")
	fs.StringVar(&untilText, "until", "", "end trade date YYYY-MM-DD")
	fs.BoolVar(&dryRun, "dry-run", false, "count source rows without writing scan rows")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	period, err := normalizeMinuteScanPeriod(periodText)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	cfg, err := config.Load(overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	refresh, err := parseMinuteScanRefresh(period, sinceText, untilText, cfg.Runtime.Timezone)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
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
	runID := newLocalRunID()
	started := time.Now()
	targetTable := minuteScanTargetTable(period)
	summary := refreshSummary{
		RunID:        runID,
		Dataset:      targetTable,
		TargetTable:  targetTable,
		Asset:        period,
		Params:       minuteScanRefreshParams(refresh),
		MinWatermark: &refresh.Since,
		MaxWatermark: &refresh.Until,
		DryRun:       dryRun,
	}
	var rows uint64
	if dryRun {
		rows, err = store.CountMinuteScanSourceRows(ctx, refresh)
	} else {
		rows, err = store.RefreshMinuteScan(ctx, refresh)
	}
	if err != nil {
		if !dryRun {
			recordRefreshFailure(ctx, store, summary, "minute_scan_refresh", "derived.minute_scan", started, err)
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	summary.RowsWritten = rows
	if dryRun {
		printRefreshSummary(stdout, summary)
		return 0
	}
	if err := recordRefreshSuccess(ctx, store, summary, "minute_scan_refresh", "derived.minute_scan", started, "ok"); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printRefreshSummary(stdout, summary)
	return 0
}

func runRefreshDailyDerived(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var market string
	var symbol string
	var sinceText string
	var untilText string
	var all bool
	var limit int
	var offset int
	var dryRun bool
	fs := newFlagSet("refresh-daily-derived", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.StringVar(&sinceText, "since", "", "start date YYYY-MM-DD")
	fs.StringVar(&untilText, "until", "", "end date YYYY-MM-DD")
	fs.BoolVar(&all, "all", false, "refresh daily derived metrics for all symbols present in daily bars")
	fs.IntVar(&limit, "limit", 0, "maximum symbols to process in --all mode")
	fs.IntVar(&offset, "offset", 0, "symbols to skip in --all mode")
	fs.BoolVar(&dryRun, "dry-run", false, "calculate derived metrics without writing ClickHouse")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := config.Load(overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	dateRange, err := parseDailyDerivedRange(sinceText, untilText, cfg.Runtime.Timezone)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if all {
		return runRefreshDailyDerivedAll(ctx, stdout, stderr, cfg, market, limit, offset, dryRun, dateRange)
	}
	req, err := tdx.ParseHQMinuteRequest(market, symbol)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
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
	runID := newLocalRunID()
	started := time.Now()
	bars, err := store.DailyBarsForSymbol(ctx, req.Market, req.Symbol)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	rows := derived.GenerateDaily(bars, dateRange, time.Now())
	summary := refreshSummary{
		RunID:        runID,
		Dataset:      "a_share_daily_derived",
		TargetTable:  "a_share_daily_derived",
		Asset:        req.Market + ":" + req.Symbol,
		MinWatermark: dailyDerivedMinDate(rows),
		MaxWatermark: dailyDerivedMaxDate(rows),
		RowsWritten:  uint64(len(rows)),
		DryRun:       dryRun,
	}
	if dryRun {
		printRefreshSummary(stdout, summary)
		return 0
	}
	if err := store.InsertDailyDerived(ctx, rows); err != nil {
		recordRefreshFailure(ctx, store, summary, "daily_derived_refresh", "derived.daily_ohlcv", started, err)
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := recordRefreshSuccess(ctx, store, summary, "daily_derived_refresh", "derived.daily_ohlcv", started, "ok"); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printRefreshSummary(stdout, summary)
	return 0
}

func runRefreshTDXXDXRAll(ctx context.Context, stdout io.Writer, stderr io.Writer, overrides config.Overrides, market string, servers []string, limit int, offset int, workers int, dryRun bool) int {
	if err := validateOptionalMarket(market); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if offset < 0 || limit < 0 {
		fmt.Fprintln(stderr, "--offset and --limit must be non-negative")
		return 2
	}
	if workers <= 0 {
		fmt.Fprintln(stderr, "--workers must be positive")
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
	symbols, err := store.DailySymbols(ctx, market)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	symbols = sliceSymbols(symbols, offset, limit)
	total := refreshSummary{RunID: newLocalRunID(), Dataset: "a_share_xdxr_events", TargetTable: "a_share_xdxr_events", Asset: "all", DryRun: dryRun}
	if market != "" {
		total.Asset = market + ":all"
	}
	serverList := servers
	if len(serverList) == 0 {
		serverList = append([]string(nil), cfg.TDX.HQServers...)
	}
	opts := hqClientOptions(serverList)
	jobs := make(chan model.Symbol)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error
	processed := 0
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				if ctx.Err() != nil {
					return
				}
				req := tdx.HQMinuteRequest{Market: item.Market, Symbol: item.Symbol}
				runID := newLocalRunID()
				started := time.Now()
				rows, err := fetchHQXDXRInfo(ctx, req, opts)
				if err != nil {
					mu.Lock()
					total.QualityIssues++
					processed++
					fmt.Fprintf(stderr, "xdxr %s:%s: %v\n", item.Market, item.Symbol, err)
					printXDXRProgress(stderr, processed, len(symbols), total)
					mu.Unlock()
					continue
				}
				events, adjustIssues := adjust.NormalizeXDXR(rows)
				summary := refreshSummary{
					RunID:         runID,
					Dataset:       "a_share_xdxr_events",
					TargetTable:   "a_share_xdxr_events",
					Asset:         item.Market + ":" + item.Symbol,
					MinWatermark:  xdxrMinDate(events),
					MaxWatermark:  xdxrMaxDate(events),
					RowsWritten:   uint64(len(events)),
					QualityIssues: len(adjustIssues),
					DryRun:        dryRun,
				}
				mu.Lock()
				if firstErr == nil {
					total.RowsWritten += summary.RowsWritten
					total.QualityIssues += summary.QualityIssues
					if !dryRun {
						if err := store.InsertXDXREvents(ctx, events); err != nil {
							recordRefreshFailure(ctx, store, summary, "tdx_xdxr_refresh", "tdx.hq.xdxr", started, err)
							firstErr = fmt.Errorf("write xdxr %s: %w", summary.Asset, err)
						} else if err := store.InsertQualityIssues(ctx, qualityIssuesFromAdjust(runID, summary.Dataset, adjustIssues)); err != nil {
							recordRefreshFailure(ctx, store, summary, "tdx_xdxr_refresh", "tdx.hq.xdxr", started, err)
							firstErr = fmt.Errorf("quality issues %s: %w", summary.Asset, err)
						} else if err := recordRefreshSuccess(ctx, store, summary, "tdx_xdxr_refresh", "tdx.hq.xdxr", started, "ok"); err != nil {
							firstErr = fmt.Errorf("ops %s: %w", summary.Asset, err)
						}
					}
				}
				processed++
				printXDXRProgress(stderr, processed, len(symbols), total)
				mu.Unlock()
			}
		}()
	}
	for _, item := range symbols {
		mu.Lock()
		shouldStop := firstErr != nil
		mu.Unlock()
		if shouldStop {
			break
		}
		jobs <- item
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		fmt.Fprintln(stderr, firstErr)
		return 1
	}
	printRefreshSummary(stdout, total)
	return 0
}

func runRefreshAdjustFactorsAll(ctx context.Context, stdout io.Writer, stderr io.Writer, overrides config.Overrides, market string, limit int, offset int, dryRun bool) int {
	if err := validateOptionalMarket(market); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if offset < 0 || limit < 0 {
		fmt.Fprintln(stderr, "--offset and --limit must be non-negative")
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
	symbols, err := store.DailySymbols(ctx, market)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	symbols = sliceSymbols(symbols, offset, limit)
	total := refreshSummary{RunID: newLocalRunID(), Dataset: "a_share_adjust_factors_1d", TargetTable: "a_share_adjust_factors_1d", Asset: "all", DryRun: dryRun}
	if market != "" {
		total.Asset = market + ":all"
	}
	factorBuffers := map[string][]model.AdjustFactor{}
	var allIssues []adjust.Issue
	flushThreshold := cfg.Runtime.BatchSize * 10
	if flushThreshold < 100000 {
		flushThreshold = 100000
	}
	for i, item := range symbols {
		bars, err := store.DailyBarsForSymbol(ctx, item.Market, item.Symbol)
		if err != nil {
			fmt.Fprintf(stderr, "daily bars %s:%s: %v\n", item.Market, item.Symbol, err)
			return 1
		}
		events, err := store.XDXREventsForSymbol(ctx, item.Market, item.Symbol)
		if err != nil {
			fmt.Fprintf(stderr, "xdxr events %s:%s: %v\n", item.Market, item.Symbol, err)
			return 1
		}
		factors, adjustIssues := adjust.GenerateFactors(bars, events, time.Now())
		mergeFactorWatermarks(&total, factors)
		total.RowsWritten += uint64(len(factors))
		total.QualityIssues += len(adjustIssues)
		allIssues = append(allIssues, adjustIssues...)
		if !dryRun {
			for _, factor := range factors {
				year := factor.TradeDate.Format("2006")
				factorBuffers[year] = append(factorBuffers[year], factor)
			}
			if err := flushLargeFactorBuffers(ctx, store, factorBuffers, flushThreshold); err != nil {
				recordRefreshFailure(ctx, store, total, "adjust_factor_refresh", "derived.adjust_factors_1d", time.Now(), err)
				fmt.Fprintf(stderr, "write factors: %v\n", err)
				return 1
			}
		}
		if i == 0 || (i+1)%100 == 0 || i+1 == len(symbols) {
			fmt.Fprintf(stderr, "processed factors %d/%d rows=%d issues=%d\n", i+1, len(symbols), total.RowsWritten, total.QualityIssues)
		}
	}
	if !dryRun {
		started := time.Now()
		if err := flushAllFactorBuffers(ctx, store, factorBuffers); err != nil {
			recordRefreshFailure(ctx, store, total, "adjust_factor_refresh", "derived.adjust_factors_1d", started, err)
			fmt.Fprintf(stderr, "write factors: %v\n", err)
			return 1
		}
		if err := store.InsertQualityIssues(ctx, qualityIssuesFromAdjust(total.RunID, total.Dataset, allIssues)); err != nil {
			recordRefreshFailure(ctx, store, total, "adjust_factor_refresh", "derived.adjust_factors_1d", started, err)
			fmt.Fprintf(stderr, "quality issues: %v\n", err)
			return 1
		}
		message := "ok"
		if len(allIssues) > 0 {
			message = "completed with quality issues"
		}
		if err := recordRefreshSuccess(ctx, store, total, "adjust_factor_refresh", "derived.adjust_factors_1d", started, message); err != nil {
			fmt.Fprintf(stderr, "ops: %v\n", err)
			return 1
		}
	}
	printRefreshSummary(stdout, total)
	return 0
}

func runRefreshDailyDerivedAll(ctx context.Context, stdout io.Writer, stderr io.Writer, cfg config.Config, market string, limit int, offset int, dryRun bool, dateRange derived.DailyRange) int {
	if err := validateOptionalMarket(market); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if offset < 0 || limit < 0 {
		fmt.Fprintln(stderr, "--offset and --limit must be non-negative")
		return 2
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
	symbols, err := store.DailySymbols(ctx, market)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	symbols = sliceSymbols(symbols, offset, limit)
	total := refreshSummary{RunID: newLocalRunID(), Dataset: "a_share_daily_derived", TargetTable: "a_share_daily_derived", Asset: "all", DryRun: dryRun}
	if market != "" {
		total.Asset = market + ":all"
	}
	buffers := map[string][]model.DailyDerived{}
	flushThreshold := cfg.Runtime.BatchSize * 10
	if flushThreshold < 100000 {
		flushThreshold = 100000
	}
	for i, item := range symbols {
		bars, err := store.DailyBarsForSymbol(ctx, item.Market, item.Symbol)
		if err != nil {
			fmt.Fprintf(stderr, "daily bars %s:%s: %v\n", item.Market, item.Symbol, err)
			return 1
		}
		rows := derived.GenerateDaily(bars, dateRange, time.Now())
		mergeDailyDerivedWatermarks(&total, rows)
		total.RowsWritten += uint64(len(rows))
		if !dryRun {
			for _, row := range rows {
				year := row.TradeDate.Format("2006")
				buffers[year] = append(buffers[year], row)
			}
			if err := flushLargeDailyDerivedBuffers(ctx, store, buffers, flushThreshold); err != nil {
				recordRefreshFailure(ctx, store, total, "daily_derived_refresh", "derived.daily_ohlcv", time.Now(), err)
				fmt.Fprintf(stderr, "write daily derived: %v\n", err)
				return 1
			}
		}
		if i == 0 || (i+1)%100 == 0 || i+1 == len(symbols) {
			fmt.Fprintf(stderr, "processed daily derived %d/%d rows=%d issues=%d\n", i+1, len(symbols), total.RowsWritten, total.QualityIssues)
		}
	}
	if !dryRun {
		started := time.Now()
		if err := flushAllDailyDerivedBuffers(ctx, store, buffers); err != nil {
			recordRefreshFailure(ctx, store, total, "daily_derived_refresh", "derived.daily_ohlcv", started, err)
			fmt.Fprintf(stderr, "write daily derived: %v\n", err)
			return 1
		}
		if err := recordRefreshSuccess(ctx, store, total, "daily_derived_refresh", "derived.daily_ohlcv", started, "ok"); err != nil {
			fmt.Fprintf(stderr, "ops: %v\n", err)
			return 1
		}
	}
	printRefreshSummary(stdout, total)
	return 0
}

func runHQFinance(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var market string
	var symbol string
	fs := newFlagSet("hq-finance", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	info, err := fetchHQFinanceInfo(ctx, req, hqClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, info)
}

func runHQBlockMeta(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var file string
	fs := newFlagSet("hq-block-meta", stderr)
	registerConfigPathFlag(fs, &overrides)
	fs.StringVar(&file, "file", "", "block file, such as block.dat, block_zs.dat, block_fg.dat, block_gn.dat")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(file) == "" {
		fmt.Fprintln(stderr, "--file is required")
		return 2
	}
	serverList, err := configuredHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	meta, err := fetchHQBlockMeta(ctx, file, hqClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, meta)
}

func runHQBlock(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var file string
	fs := newFlagSet("hq-block", stderr)
	registerConfigPathFlag(fs, &overrides)
	fs.StringVar(&file, "file", "", "block file, such as block.dat, block_zs.dat, block_fg.dat, block_gn.dat")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(file) == "" {
		fmt.Fprintln(stderr, "--file is required")
		return 2
	}
	serverList, err := configuredHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	members, err := fetchHQBlockMembers(ctx, file, hqClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, members)
}

func runExQuoteMarkets(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	fs := newFlagSet("exquote-markets", stderr)
	registerConfigPathFlag(fs, &overrides)
	fs.Var(&servers, "server", "TDX ExHQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	serverList, err := configuredExHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	markets, err := fetchExMarkets(ctx, exQuoteClientOptions(serverList))
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
	var overrides config.Overrides
	var servers listFlags
	fs := newFlagSet("exquote-count", stderr)
	registerConfigPathFlag(fs, &overrides)
	fs.Var(&servers, "server", "TDX ExHQ server host:port; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	serverList, err := configuredExHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	count, err := fetchExInstrumentCount(ctx, exQuoteClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, count)
}

func runExQuoteInstruments(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var start int
	var count int
	fs := newFlagSet("exquote-instruments", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredExHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	instruments, err := fetchExInstruments(ctx, start, count, exQuoteClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, instruments)
}

func runExQuote(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var market int
	var code string
	fs := newFlagSet("exquote", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredExHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	quote, err := fetchExQuote(ctx, req, exQuoteClientOptions(serverList))
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
	var overrides config.Overrides
	var servers listFlags
	var market int
	var code string
	var category int
	var start int
	var count int
	fs := newFlagSet("exquote-bars", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredExHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	bars, err := fetchExBars(ctx, req, exQuoteClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, bars)
}

func runExQuoteMinute(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var market int
	var code string
	fs := newFlagSet("exquote-minute", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredExHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	points, err := fetchExMinuteTime(ctx, req, exQuoteClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, points)
}

func runExQuoteHistoryMinute(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var market int
	var code string
	var dateText string
	fs := newFlagSet("exquote-history-minute", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredExHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	points, err := fetchExHistoryMinuteTime(ctx, req, date, exQuoteClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, points)
}

func runExQuoteTransactions(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var market int
	var code string
	var start int
	var count int
	fs := newFlagSet("exquote-transactions", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredExHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	transactions, err := fetchExTransactions(ctx, req, start, count, exQuoteClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, transactions)
}

func runExQuoteHistoryTransactions(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var market int
	var code string
	var dateText string
	var start int
	var count int
	fs := newFlagSet("exquote-history-transactions", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredExHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	transactions, err := fetchExHistoryTransactions(ctx, req, date, start, count, exQuoteClientOptions(serverList))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeJSON(stdout, stderr, transactions)
}

func runExQuoteHistoryBars(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var market int
	var code string
	var startDateText string
	var endDateText string
	fs := newFlagSet("exquote-history-bars", stderr)
	registerConfigPathFlag(fs, &overrides)
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
	serverList, err := configuredExHQServers([]string(servers), overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	bars, err := fetchExHistoryBarsRange(ctx, req, startDate, endDate, exQuoteClientOptions(serverList))
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
		if cfg.MySQL.Configured() {
			if err := cfg.MySQL.RequiredError(); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			mysqlDDL, err := securitymaster.BootstrapDDL(cfg.MySQL.Database)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			for _, stmt := range mysqlDDL {
				fmt.Fprintln(stdout, stmt+";")
			}
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
	if cfg.MySQL.Configured() {
		if err := bootstrapSecurityMaster(ctx, cfg.MySQL); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
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

func runImportIntradayPoints(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var servers listFlags
	var bestIP bestIPFlags
	var market, symbol, dateText, since, until string
	var today bool
	var dryRun bool
	fs := newFlagSet("import-tdx-intraday-points", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.StringVar(&dateText, "date", "", "historical date YYYY-MM-DD or YYYYMMDD")
	fs.StringVar(&since, "since", "", "start date YYYY-MM-DD or YYYYMMDD")
	fs.StringVar(&until, "until", "", "end date YYYY-MM-DD or YYYYMMDD")
	fs.BoolVar(&today, "today", false, "fetch current-day minute-time data")
	fs.Var(&servers, "server", "TDX HQ server host:port; repeat or comma-separate")
	registerBestIPFlags(fs, &bestIP)
	fs.BoolVar(&dryRun, "dry-run", false, "fetch and summarize without writing")
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
	serverList := []string(servers)
	if len(serverList) == 0 {
		serverList = append([]string(nil), cfg.TDX.HQServers...)
	}
	clientOpts, err := quoteClientOptions(serverList, cfg.Runtime.BatchSize, "", bestIP)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var store *chstore.Store
	if !dryRun {
		store, err = chstore.Open(ctx, cfg.ClickHouse)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer store.Close()
	}
	summary, err := ingest.ImportIntradayPoints(ctx, ingest.IntradayImportOptions{
		Market:             market,
		Symbol:             symbol,
		Date:               dateText,
		Since:              since,
		Until:              until,
		Today:              today,
		DryRun:             dryRun,
		Store:              store,
		Timezone:           cfg.Runtime.Timezone,
		ClientOptions:      clientOpts,
		FetchMinuteTime:    fetchHQMinuteTime,
		FetchHistoryMinute: fetchHQHistoryMinuteTime,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	printIntradaySummary(stdout, summary)
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

func runTDXFinFiles(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var baseURL string
	fs := newFlagSet("tdx-fin-files", stderr)
	fs.StringVar(&baseURL, "base-url", finance.DefaultFinancialRemoteBaseURL, "remote tdxfin base URL")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	files, err := finance.RemoteClient{BaseURL: baseURL}.List(ctx)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printRemoteFinancialFiles(stdout, files)
	return 0
}

func runTDXFinFetch(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var baseURL, filename, dir string
	var all bool
	fs := newFlagSet("tdx-fin-fetch", stderr)
	fs.StringVar(&baseURL, "base-url", finance.DefaultFinancialRemoteBaseURL, "remote tdxfin base URL")
	fs.StringVar(&filename, "filename", "", "financial package filename, e.g. gpcw20251231.zip")
	fs.StringVar(&dir, "dir", ".", "download directory")
	fs.BoolVar(&all, "all", false, "fetch every package in gpcw.txt")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	filename = strings.TrimSpace(filename)
	if all && filename != "" {
		fmt.Fprintln(stderr, "use either --filename or --all")
		return 2
	}
	if !all && filename == "" {
		fmt.Fprintln(stderr, "--filename or --all is required")
		return 2
	}
	client := finance.RemoteClient{BaseURL: baseURL}
	files, err := client.List(ctx)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	selected := files
	if !all {
		if err := finance.ValidateRemoteFinancialFilename(filename); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		file, ok := finance.FindRemoteFinancialFile(files, filename)
		if !ok {
			fmt.Fprintf(stderr, "%s not found in remote manifest\n", filename)
			return 1
		}
		selected = []finance.RemoteFinancialFile{file}
	}
	for _, file := range selected {
		result, err := client.Fetch(ctx, file, dir)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		printRemoteFinancialFetchResult(stdout, result)
	}
	return 0
}

func runTDXFinParse(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var file string
	fs := newFlagSet("tdx-fin-parse", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&file, "file", "", "downloaded gpcw zip or dat file")
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
	summary, err := ingest.ParseTDXFinancial(ctx, ingest.TDXFinancialOptions{
		File:     file,
		DryRun:   true,
		Timezone: cfg.Runtime.Timezone,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printTDXFinancialSummary(stdout, summary)
	return 0
}

func runImportGBBQ(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var file, since, until string
	var dryRun bool
	fs := newFlagSet("import-tdx-gbbq", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&file, "file", "", "client-local gbbq file")
	fs.StringVar(&since, "since", "", "start date YYYY-MM-DD")
	fs.StringVar(&until, "until", "", "end date YYYY-MM-DD")
	fs.BoolVar(&dryRun, "dry-run", false, "parse and summarize without writing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(file) == "" {
		fmt.Fprintln(stderr, "--file is required")
		return 2
	}
	cfg, store, cleanup, ok := loadConfigAndMaybeStore(ctx, overrides, dryRun, stderr)
	if !ok {
		return 1
	}
	defer cleanup()
	summary, err := ingest.ImportTDXGBBQ(ctx, ingest.GBBQOptions{File: file, Since: since, Until: until, DryRun: dryRun, Store: store, Timezone: cfg.Runtime.Timezone, BatchSize: cfg.Runtime.BatchSize})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printSummary(stdout, summary)
	return 0
}

func runImportTDXBlock(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var file, scope string
	var dryRun bool
	fs := newFlagSet("import-tdx-block", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&file, "file", "", "client-local system block file or custom blocknew directory")
	fs.StringVar(&scope, "scope", "system", "block scope: system or custom")
	fs.BoolVar(&dryRun, "dry-run", false, "parse and summarize without writing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(file) == "" {
		fmt.Fprintln(stderr, "--file is required")
		return 2
	}
	cfg, store, cleanup, ok := loadConfigAndMaybeStore(ctx, overrides, dryRun, stderr)
	if !ok {
		return 1
	}
	defer cleanup()
	summary, err := ingest.ImportTDXBlock(ctx, ingest.BlockOptions{File: file, Scope: scope, DryRun: dryRun, Store: store, Timezone: cfg.Runtime.Timezone, BatchSize: cfg.Runtime.BatchSize})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printSummary(stdout, summary)
	return 0
}

func runImportExDaily(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var file, marketText, code, since, until string
	var dryRun bool
	fs := newFlagSet("import-tdx-ex-daily", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&file, "file", "", "client-local ex_daily file")
	fs.StringVar(&marketText, "market", "", "extension market id")
	fs.StringVar(&code, "code", "", "extension instrument code")
	fs.StringVar(&since, "since", "", "start date YYYY-MM-DD")
	fs.StringVar(&until, "until", "", "end date YYYY-MM-DD")
	fs.BoolVar(&dryRun, "dry-run", false, "parse and summarize without writing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(file) == "" || strings.TrimSpace(marketText) == "" || strings.TrimSpace(code) == "" {
		fmt.Fprintln(stderr, "--file, --market, and --code are required")
		return 2
	}
	market, err := tdx.ParseExMarket(marketText)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	cfg, store, cleanup, ok := loadConfigAndMaybeStore(ctx, overrides, dryRun, stderr)
	if !ok {
		return 1
	}
	defer cleanup()
	summary, err := ingest.ImportTDXExDaily(ctx, ingest.ExDailyOptions{File: file, Market: market, Code: code, Since: since, Until: until, DryRun: dryRun, Store: store, Timezone: cfg.Runtime.Timezone})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printSummary(stdout, summary)
	return 0
}

func runWriteCustomBlock(args []string, stdout io.Writer, stderr io.Writer) int {
	var file, blockID, addText, removeText, replaceText string
	var dryRun bool
	fs := newFlagSet("write-tdx-custom-block", stderr)
	fs.StringVar(&file, "file", "", "client-local T0002/blocknew directory")
	fs.StringVar(&blockID, "block-id", "", "custom block id")
	fs.StringVar(&addText, "add", "", "comma-separated symbols to add")
	fs.StringVar(&removeText, "remove", "", "comma-separated symbols to remove")
	fs.StringVar(&replaceText, "replace", "", "comma-separated replacement symbols")
	fs.BoolVar(&dryRun, "dry-run", false, "print planned result without writing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(file) == "" || strings.TrimSpace(blockID) == "" {
		fmt.Fprintln(stderr, "--file and --block-id are required")
		return 2
	}
	parsed, err := tdx.ParseCustomBlockDir(expandHome(file), time.Now())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	edited, err := tdx.ApplyCustomBlockEdit(parsed, tdx.CustomBlockEdit{BlockID: blockID, Add: splitCSV(addText), Remove: splitCSV(removeText), Replace: splitCSV(replaceText)})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if dryRun {
		printCustomBlockPlan(stdout, edited, blockID)
		return 0
	}
	if err := tdx.WriteCustomBlockDir(expandHome(file), edited); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printCustomBlockPlan(stdout, edited, blockID)
	return 0
}

func loadConfigAndMaybeStore(ctx context.Context, overrides config.Overrides, dryRun bool, stderr io.Writer) (config.Config, *chstore.Store, func(), bool) {
	cfg, err := config.Load(overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return config.Config{}, nil, func() {}, false
	}
	cleanupLogging, ok := setupLogging(cfg, stderr)
	if !ok {
		return config.Config{}, nil, func() {}, false
	}
	cleanup := cleanupLogging
	var store *chstore.Store
	if !dryRun {
		store, err = chstore.Open(ctx, cfg.ClickHouse)
		if err != nil {
			cleanupLogging()
			fmt.Fprintln(stderr, err)
			return config.Config{}, nil, func() {}, false
		}
		cleanup = func() {
			_ = store.Close()
			cleanupLogging()
		}
	}
	return cfg, store, cleanup, true
}

func splitCSV(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func printCustomBlockPlan(out io.Writer, parsed tdx.BlockParseResult, blockID string) {
	fmt.Fprintf(out, "block_id: %s\n", blockID)
	fmt.Fprintf(out, "snapshot_id: %s\n", parsed.Snapshot.SnapshotID)
	var count int
	for _, member := range parsed.Memberships {
		if member.BlockID == blockID {
			count++
			fmt.Fprintf(out, "- %s:%s\n", member.Market, member.Symbol)
		}
	}
	fmt.Fprintf(out, "members: %d\n", count)
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

func printIntradaySummary(out io.Writer, summary ingest.IntradaySummary) {
	mode := "write"
	if summary.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(out, "mode: %s\n", mode)
	fmt.Fprintf(out, "run_id: %s\n", summary.RunID)
	fmt.Fprintf(out, "dataset: %s\n", summary.Dataset)
	fmt.Fprintf(out, "target_table: %s\n", summary.TargetTable)
	fmt.Fprintf(out, "market: %s\n", summary.Market)
	fmt.Fprintf(out, "symbol: %s\n", summary.Symbol)
	fmt.Fprintf(out, "dates_fetched: %d\n", summary.DatesFetched)
	fmt.Fprintf(out, "empty_dates: %d\n", summary.EmptyDates)
	fmt.Fprintf(out, "rows_written: %d\n", summary.RowsWritten)
	fmt.Fprintf(out, "rows_skipped: %d\n", summary.RowsSkipped)
	fmt.Fprintf(out, "quality_issues: %d\n", len(summary.Issues))
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

func printRemoteFinancialFiles(out io.Writer, files []finance.RemoteFinancialFile) {
	for _, file := range files {
		fmt.Fprintf(out, "%s %s %d", file.Filename, file.MD5, file.Size)
		if file.ReportDate != "" {
			fmt.Fprintf(out, " %s", file.ReportDate)
		}
		fmt.Fprintln(out)
	}
}

func printRemoteFinancialFetchResult(out io.Writer, result finance.RemoteFinancialFetchResult) {
	status := "downloaded"
	if result.Skipped {
		status = "skipped"
	}
	fmt.Fprintf(out, "%s %s %s bytes=%d\n", status, result.Filename, result.Path, result.Bytes)
}

func printRefreshSummary(out io.Writer, summary refreshSummary) {
	mode := "write"
	if summary.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(out, "run_id: %s\n", summary.RunID)
	fmt.Fprintf(out, "mode: %s\n", mode)
	fmt.Fprintf(out, "dataset: %s\n", summary.Dataset)
	fmt.Fprintf(out, "target_table: %s\n", summary.TargetTable)
	fmt.Fprintf(out, "asset: %s\n", summary.Asset)
	fmt.Fprintf(out, "rows_written: %d\n", summary.RowsWritten)
	fmt.Fprintf(out, "quality_issues: %d\n", summary.QualityIssues)
}

func recordRefreshSuccess(ctx context.Context, store *chstore.Store, summary refreshSummary, taskType string, inputFormat string, started time.Time, message string) error {
	now := time.Now()
	duration := uint64(now.Sub(started).Milliseconds())
	status := "success"
	if summary.QualityIssues > 0 {
		status = "degraded"
	}
	params := refreshParams(summary)
	if err := store.InsertWatermark(ctx, model.Watermark{
		Dataset:      summary.Dataset,
		Asset:        summary.Asset,
		Status:       status,
		MinWatermark: summary.MinWatermark,
		MaxWatermark: summary.MaxWatermark,
		RowsWritten:  summary.RowsWritten,
		Message:      message,
		UpdatedAt:    now,
	}); err != nil {
		return err
	}
	return store.InsertTaskRun(ctx, model.TaskRun{
		RunID:       summary.RunID,
		Dataset:     summary.Dataset,
		TaskType:    taskType,
		Status:      status,
		TargetTable: summary.TargetTable,
		InputFormat: inputFormat,
		Params:      params,
		StartedAt:   started,
		FinishedAt:  &now,
		DurationMS:  &duration,
		RowsWritten: summary.RowsWritten,
		UpdatedAt:   now,
	})
}

func recordRefreshFailure(ctx context.Context, store *chstore.Store, summary refreshSummary, taskType string, inputFormat string, started time.Time, err error) {
	now := time.Now()
	duration := uint64(now.Sub(started).Milliseconds())
	_ = store.InsertTaskRun(ctx, model.TaskRun{
		RunID:       summary.RunID,
		Dataset:     summary.Dataset,
		TaskType:    taskType,
		Status:      "failed",
		TargetTable: summary.TargetTable,
		InputFormat: inputFormat,
		Params:      refreshParams(summary),
		StartedAt:   started,
		FinishedAt:  &now,
		DurationMS:  &duration,
		RowsWritten: summary.RowsWritten,
		Error:       err.Error(),
		UpdatedAt:   now,
	})
}

func refreshParams(summary refreshSummary) string {
	if summary.Params != "" {
		return summary.Params
	}
	return "asset=" + summary.Asset
}

func qualityIssuesFromAdjust(runID string, dataset string, issues []adjust.Issue) []model.QualityIssue {
	if len(issues) == 0 {
		return nil
	}
	now := time.Now()
	out := make([]model.QualityIssue, 0, len(issues))
	for _, issue := range issues {
		logicalKey := issue.Market + ":" + issue.Symbol
		if !issue.TradeDate.IsZero() {
			logicalKey += ":" + issue.TradeDate.Format("2006-01-02")
		}
		severity := "warning"
		switch issue.Type {
		case "missing_previous_close", "missing_xdxr_fields", "invalid_xdxr_denominator", "invalid_adjust_ratio", "zero_daily_bars":
			severity = "error"
		}
		out = append(out, model.QualityIssue{
			RunID:      runID,
			Dataset:    dataset,
			Severity:   severity,
			IssueType:  issue.Type,
			Market:     issue.Market,
			Symbol:     issue.Symbol,
			LogicalKey: logicalKey,
			ObservedAt: now,
			Message:    issue.Message,
		})
	}
	return out
}

func xdxrMinDate(events []model.XDXREvent) *time.Time {
	if len(events) == 0 {
		return nil
	}
	min := events[0].EventDate
	for _, event := range events[1:] {
		if event.EventDate.Before(min) {
			min = event.EventDate
		}
	}
	return &min
}

func xdxrMaxDate(events []model.XDXREvent) *time.Time {
	if len(events) == 0 {
		return nil
	}
	max := events[0].EventDate
	for _, event := range events[1:] {
		if event.EventDate.After(max) {
			max = event.EventDate
		}
	}
	return &max
}

func factorMinDate(factors []model.AdjustFactor) *time.Time {
	if len(factors) == 0 {
		return nil
	}
	min := factors[0].TradeDate
	for _, factor := range factors[1:] {
		if factor.TradeDate.Before(min) {
			min = factor.TradeDate
		}
	}
	return &min
}

func factorMaxDate(factors []model.AdjustFactor) *time.Time {
	if len(factors) == 0 {
		return nil
	}
	max := factors[0].TradeDate
	for _, factor := range factors[1:] {
		if factor.TradeDate.After(max) {
			max = factor.TradeDate
		}
	}
	return &max
}

func mergeFactorWatermarks(summary *refreshSummary, factors []model.AdjustFactor) {
	minDate := factorMinDate(factors)
	maxDate := factorMaxDate(factors)
	if minDate != nil && (summary.MinWatermark == nil || minDate.Before(*summary.MinWatermark)) {
		summary.MinWatermark = minDate
	}
	if maxDate != nil && (summary.MaxWatermark == nil || maxDate.After(*summary.MaxWatermark)) {
		summary.MaxWatermark = maxDate
	}
}

func dailyDerivedMinDate(rows []model.DailyDerived) *time.Time {
	if len(rows) == 0 {
		return nil
	}
	min := rows[0].TradeDate
	for _, row := range rows[1:] {
		if row.TradeDate.Before(min) {
			min = row.TradeDate
		}
	}
	return &min
}

func dailyDerivedMaxDate(rows []model.DailyDerived) *time.Time {
	if len(rows) == 0 {
		return nil
	}
	max := rows[0].TradeDate
	for _, row := range rows[1:] {
		if row.TradeDate.After(max) {
			max = row.TradeDate
		}
	}
	return &max
}

func mergeDailyDerivedWatermarks(summary *refreshSummary, rows []model.DailyDerived) {
	minDate := dailyDerivedMinDate(rows)
	maxDate := dailyDerivedMaxDate(rows)
	if minDate != nil && (summary.MinWatermark == nil || minDate.Before(*summary.MinWatermark)) {
		summary.MinWatermark = minDate
	}
	if maxDate != nil && (summary.MaxWatermark == nil || maxDate.After(*summary.MaxWatermark)) {
		summary.MaxWatermark = maxDate
	}
}

func flushLargeFactorBuffers(ctx context.Context, store *chstore.Store, buffers map[string][]model.AdjustFactor, threshold int) error {
	for year, factors := range buffers {
		if len(factors) < threshold {
			continue
		}
		if err := store.InsertAdjustFactors(ctx, factors); err != nil {
			return err
		}
		buffers[year] = factors[:0]
	}
	return nil
}

func flushAllFactorBuffers(ctx context.Context, store *chstore.Store, buffers map[string][]model.AdjustFactor) error {
	for year, factors := range buffers {
		if len(factors) == 0 {
			continue
		}
		if err := store.InsertAdjustFactors(ctx, factors); err != nil {
			return fmt.Errorf("year %s: %w", year, err)
		}
		buffers[year] = factors[:0]
	}
	return nil
}

func flushLargeDailyDerivedBuffers(ctx context.Context, store *chstore.Store, buffers map[string][]model.DailyDerived, threshold int) error {
	for year, rows := range buffers {
		if len(rows) < threshold {
			continue
		}
		if err := store.InsertDailyDerived(ctx, rows); err != nil {
			return err
		}
		buffers[year] = rows[:0]
	}
	return nil
}

func flushAllDailyDerivedBuffers(ctx context.Context, store *chstore.Store, buffers map[string][]model.DailyDerived) error {
	for year, rows := range buffers {
		if len(rows) == 0 {
			continue
		}
		if err := store.InsertDailyDerived(ctx, rows); err != nil {
			return fmt.Errorf("year %s: %w", year, err)
		}
		buffers[year] = rows[:0]
	}
	return nil
}

func normalizeMinuteScanPeriod(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1m":
		return "1m", nil
	case "5m":
		return "5m", nil
	default:
		return "", fmt.Errorf("--period must be 1m or 5m")
	}
}

func minuteScanTargetTable(period string) string {
	if period == "5m" {
		return "a_share_bars_5m_scan"
	}
	return "a_share_bars_1m_scan"
}

func parseMinuteScanRefresh(period string, sinceText string, untilText string, timezone string) (chstore.MinuteScanRefresh, error) {
	if strings.TrimSpace(sinceText) == "" || strings.TrimSpace(untilText) == "" {
		return chstore.MinuteScanRefresh{}, fmt.Errorf("--since and --until are required")
	}
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return chstore.MinuteScanRefresh{}, err
	}
	since, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(sinceText), loc)
	if err != nil {
		return chstore.MinuteScanRefresh{}, fmt.Errorf("parse --since: %w", err)
	}
	until, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(untilText), loc)
	if err != nil {
		return chstore.MinuteScanRefresh{}, fmt.Errorf("parse --until: %w", err)
	}
	if since.After(until) {
		return chstore.MinuteScanRefresh{}, fmt.Errorf("--since must be <= --until")
	}
	return chstore.MinuteScanRefresh{Period: period, Since: since, Until: until}, nil
}

func minuteScanRefreshParams(refresh chstore.MinuteScanRefresh) string {
	return fmt.Sprintf("period=%s since=%s until=%s", refresh.Period, refresh.Since.Format("2006-01-02"), refresh.Until.Format("2006-01-02"))
}

func parseDailyDerivedRange(sinceText string, untilText string, timezone string) (derived.DailyRange, error) {
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return derived.DailyRange{}, err
	}
	var r derived.DailyRange
	if strings.TrimSpace(sinceText) != "" {
		since, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(sinceText), loc)
		if err != nil {
			return derived.DailyRange{}, fmt.Errorf("parse --since: %w", err)
		}
		r.Since = &since
	}
	if strings.TrimSpace(untilText) != "" {
		until, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(untilText), loc)
		if err != nil {
			return derived.DailyRange{}, fmt.Errorf("parse --until: %w", err)
		}
		r.Until = &until
	}
	if r.Since != nil && r.Until != nil && r.Since.After(*r.Until) {
		return derived.DailyRange{}, fmt.Errorf("--since must be <= --until")
	}
	return r, nil
}

func validateOptionalMarket(market string) error {
	switch strings.TrimSpace(market) {
	case "", "sh", "sz", "bj":
		return nil
	default:
		return fmt.Errorf("market must be sh, sz, or bj")
	}
}

func sliceSymbols(symbols []model.Symbol, offset int, limit int) []model.Symbol {
	if offset >= len(symbols) {
		return nil
	}
	symbols = symbols[offset:]
	if limit > 0 && limit < len(symbols) {
		return symbols[:limit]
	}
	return symbols
}

func printXDXRProgress(stderr io.Writer, processed int, totalSymbols int, summary refreshSummary) {
	if processed == 1 || processed%100 == 0 || processed == totalSymbols {
		fmt.Fprintf(stderr, "processed xdxr %d/%d rows=%d issues=%d\n", processed, totalSymbols, summary.RowsWritten, summary.QualityIssues)
	}
}

func newLocalRunID() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw[:])
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
	fmt.Fprintln(out, "  tdx-fin-files")
	fmt.Fprintln(out, "  tdx-fin-fetch")
	fmt.Fprintln(out, "  tdx-fin-parse")
	fmt.Fprintln(out, "  import-tdx-intraday-points")
	fmt.Fprintln(out, "  import-tdx-gbbq")
	fmt.Fprintln(out, "  import-tdx-block")
	fmt.Fprintln(out, "  import-tdx-ex-daily")
	fmt.Fprintln(out, "  write-tdx-custom-block")
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
	fmt.Fprintln(out, "  refresh-tdx-xdxr")
	fmt.Fprintln(out, "  refresh-adjust-factors")
	fmt.Fprintln(out, "  refresh-daily-derived")
	fmt.Fprintln(out, "  refresh-minute-scan")
	fmt.Fprintln(out, "  refresh-security-master")
	fmt.Fprintln(out, "  hq-finance")
	fmt.Fprintln(out, "  hq-block-meta")
	fmt.Fprintln(out, "  hq-block")
	fmt.Fprintln(out, "  quote-serve")
	fmt.Fprintln(out, "  quote-status")
	fmt.Fprintln(out, "  hq-quotes-list")
	fmt.Fprintln(out, "  hq-top-board")
	fmt.Fprintln(out, "  hq-lhb")
	fmt.Fprintln(out, "  sp-board-members")
	fmt.Fprintln(out, "  fund-kline")
	fmt.Fprintln(out, "  fund-detail")
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
