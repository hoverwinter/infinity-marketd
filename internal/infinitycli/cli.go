package infinitycli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	chstore "github.com/hoverwinter/infinity-marketd/internal/clickhouse"
	"github.com/hoverwinter/infinity-marketd/internal/config"
	"github.com/hoverwinter/infinity-marketd/internal/consoleops"
	"github.com/hoverwinter/infinity-marketd/internal/logging"
	"github.com/hoverwinter/infinity-marketd/internal/querier"
	"github.com/hoverwinter/infinity-marketd/internal/securitymaster"
	"go.uber.org/zap"
)

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "querier":
		return runQuerier(ctx, args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runQuerier(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printQuerierUsage(stderr)
		return 2
	}
	switch args[0] {
	case "serve":
		return runQuerierServe(ctx, args[1:], stdout, stderr)
	case "health":
		return runQuerierHealth(ctx, args[1:], stdout, stderr)
	case "bars":
		return runQuerierBars(ctx, args[1:], stdout, stderr)
	case "intraday-points":
		return runQuerierIntradayPoints(ctx, args[1:], stdout, stderr)
	case "resolve-symbol":
		return runQuerierResolveSymbol(ctx, args[1:], stdout, stderr)
	case "providers", "provider-bars", "provider-boards", "provider-board":
		return runProviderCommand(ctx, args[0], args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printQuerierUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown querier command %q\n", args[0])
		printQuerierUsage(stderr)
		return 2
	}
}

func runQuerierServe(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var listen string
	var consoleDist string
	fs := newFlagSet("querier serve", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&listen, "listen", envOrDefault("INFINITY_QUERIER_LISTEN", "127.0.0.1:8808"), "HTTP listen address")
	fs.StringVar(&consoleDist, "console-dist", envOrDefault("INFINITY_CONSOLE_DIST", ""), "Vite console dist directory to serve at /console/")
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
	var securitiesRepo securitymaster.Reader
	var closeSecurities func() error
	if cfg.MySQL.Configured() {
		securitiesStore, err := securitymaster.Open(ctx, cfg.MySQL)
		if err != nil {
			zap.L().Warn("securities master unavailable", zap.Error(err))
			securitiesRepo = securitymaster.NewUnavailableReader(err)
		} else {
			securitiesRepo = securitiesStore
			closeSecurities = securitiesStore.Close
		}
	} else {
		securitiesRepo = securitymaster.NewUnavailableReader(fmt.Errorf("mysql is not configured"))
	}
	if closeSecurities != nil {
		defer closeSecurities()
	}
	server := querier.NewServerWithSecurities(store, securitiesRepo).WithConsoleHQDailyImporter(consoleops.HQDailyImporter(store, cfg))
	server.WithTHSCookie(os.Getenv("INFINITY_THS_COOKIE"))
	httpServer := &http.Server{
		Addr:              listen,
		Handler:           server.HandlerWithConsoleDist(consoleDist),
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Fprintf(stdout, "querier listening on http://%s\n", listen)
	zap.L().Info("querier listening", zap.String("listen", listen))
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		zap.L().Error("querier server stopped", zap.Error(err))
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runQuerierHealth(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var baseURL string
	fs := newFlagSet("querier health", stderr)
	registerServiceFlags(fs, &baseURL)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	client := querier.NewHTTPClient(baseURL, nil)
	health, err := client.Health(ctx)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "querier: %s\n", health.Status)
	return 0
}

func runQuerierBars(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var baseURL, market, symbol, period, adjust, since, until string
	var limit int
	fs := newFlagSet("querier bars", stderr)
	registerServiceFlags(fs, &baseURL)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.StringVar(&period, "period", "1d", "bar period: 1d, 1m, or 5m")
	fs.StringVar(&adjust, "adjust", "none", "adjustment mode: none, qfq, or hfq")
	fs.StringVar(&since, "since", "", "inclusive lower bound date/time")
	fs.StringVar(&until, "until", "", "exclusive upper bound date/time")
	fs.IntVar(&limit, "limit", 1000, "maximum rows to return")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	client := querier.NewHTTPClient(baseURL, nil)
	result, err := client.Bars(ctx, querier.BarQuery{
		Market: market,
		Symbol: symbol,
		Period: period,
		Adjust: adjust,
		Since:  since,
		Until:  until,
		Limit:  limit,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	writeJSON(stdout, result)
	return 0
}

func runQuerierIntradayPoints(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var baseURL, market, symbol, dateText, since, until string
	var limit int
	fs := newFlagSet("querier intraday-points", stderr)
	registerServiceFlags(fs, &baseURL)
	fs.StringVar(&market, "market", "", "market sh/sz/bj")
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	fs.StringVar(&dateText, "date", "", "trade date YYYY-MM-DD or YYYYMMDD")
	fs.StringVar(&since, "since", "", "inclusive lower bound date/time")
	fs.StringVar(&until, "until", "", "inclusive upper bound date/time")
	fs.IntVar(&limit, "limit", 1000, "maximum rows to return")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	client := querier.NewHTTPClient(baseURL, nil)
	result, err := client.IntradayPoints(ctx, querier.IntradayPointQuery{
		Market: market,
		Symbol: symbol,
		Date:   dateText,
		Since:  since,
		Until:  until,
		Limit:  limit,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	writeJSON(stdout, result)
	return 0
}

func runQuerierResolveSymbol(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var baseURL, symbol string
	fs := newFlagSet("querier resolve-symbol", stderr)
	registerServiceFlags(fs, &baseURL)
	fs.StringVar(&symbol, "symbol", "", "six-digit symbol")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	client := querier.NewHTTPClient(baseURL, nil)
	result, err := client.ResolveSymbol(ctx, symbol)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	writeJSON(stdout, result)
	return 0
}

func registerServiceFlags(fs *flag.FlagSet, baseURL *string) {
	fs.StringVar(baseURL, "url", envOrDefault("INFINITY_QUERIER_URL", "http://127.0.0.1:8808"), "querier service URL")
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
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

func writeJSON(out io.Writer, value any) {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	_ = enc.Encode(value)
}

func envOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, "usage: infinity <command> [flags]")
	fmt.Fprintln(out, "commands:")
	fmt.Fprintln(out, "  querier")
}

func printQuerierUsage(out io.Writer) {
	fmt.Fprintln(out, "usage: infinity querier <command> [flags]")
	fmt.Fprintln(out, "commands:")
	fmt.Fprintln(out, "  serve")
	fmt.Fprintln(out, "  health")
	fmt.Fprintln(out, "  bars")
	fmt.Fprintln(out, "  intraday-points")
	fmt.Fprintln(out, "  resolve-symbol")
	fmt.Fprintln(out, "  providers / provider-bars / provider-boards / provider-board")
}
