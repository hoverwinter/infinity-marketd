package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/config"
	"github.com/hoverwinter/infinity-marketd/internal/ingest"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/querier"
)

var refreshTHSLimitReview = ingest.RefreshTHSLimitReview

var importTDXLimitIndex = ingest.ImportTDXLimitIndex

func runImportLimitIndex(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var overrides config.Overrides
	var opts ingest.LimitIndexImportOptions
	var servers listFlags
	fs := newFlagSet("import-limit-performance-tdx", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&opts.IndexCode, "index-code", "", "verified semantic index code")
	fs.StringVar(&opts.Since, "since", "2016-01-01", "inclusive start date")
	fs.StringVar(&opts.Until, "until", "", "inclusive end date")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "fetch and validate without writes")
	fs.Var(&servers, "server", "TDX HQ server host:port")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if opts.Until == "" || opts.IndexCode == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "--index-code and --until are required")
		return 2
	}
	cfg, store, cleanup, ok := loadConfigAndMaybeStore(ctx, overrides, opts.DryRun, stderr)
	if !ok {
		return 1
	}
	defer cleanup()
	if store != nil {
		opts.Store = store
	}
	if len(servers) == 0 {
		servers = append(servers, cfg.TDX.HQServers...)
	}
	opts.ClientOptions = hqClientOptions([]string(servers))
	result, err := importTDXLimitIndex(ctx, opts)
	if status := writeJSON(stdout, stderr, result); status != 0 {
		return status
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runRefreshLimitReview(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var overrides config.Overrides
	var opts ingest.THSReviewOptions
	var readURL string
	fs := newFlagSet("refresh-limit-review", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&readURL, "read-url", "http://127.0.0.1:8808", "querier for current-event protection")
	fs.StringVar(&opts.Date, "date", "", "closed trading date YYYY-MM-DD")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "fetch and validate without writes")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 || opts.Date == "" {
		fmt.Fprintln(stderr, "--date is required with no positional arguments")
		return 2
	}
	_, store, cleanup, ok := loadConfigAndMaybeStore(ctx, overrides, opts.DryRun, stderr)
	if !ok {
		return 1
	}
	defer cleanup()
	opts.LoadEvents = limitEventReader(readURL)
	if store != nil {
		opts.Store = store
	}
	result, err := refreshTHSLimitReview(ctx, opts)
	if status := writeJSON(stdout, stderr, result); status != 0 {
		return status
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runImportLimitReview(ctx context.Context, args []string, stdout, stderr io.Writer, command string) int {
	var overrides config.Overrides
	var opts ingest.LimitReviewImportOptions
	var readURL string
	fs := newFlagSet(command, stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&readURL, "read-url", "http://127.0.0.1:8808", "querier for current-event protection")
	fs.BoolVar(&opts.AllowFactReplacement, "allow-fact-replacement", false, "operator-only: allow replacing core facts and prior attribution")
	fs.StringVar(&opts.File, "file", "", "input JSON or JSONL file")
	if command == "import-limit-review-json" {
		fs.StringVar(&opts.PercentUnit, "percent-unit", "percent", "input relay percentage unit: percent (THS) or ratio (historical replay)")
		fs.StringVar(&opts.SnapshotKind, "snapshot-kind", "generic", "legacy writer profile: generic, historical-replay, or ths (known placeholder zeros become null)")
	}
	fs.StringVar(&opts.Since, "since", "2016-01-01", "inclusive start date YYYY-MM-DD")
	fs.StringVar(&opts.Until, "until", "", "inclusive end date YYYY-MM-DD")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "validate and report without writing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if command == "import-limit-review-json" {
		opts.Root = overrides.TDXRoot
		overrides.TDXRoot = ""
	} else if overrides.TDXRoot != "" {
		fmt.Fprintln(stderr, "--root is only supported for snapshot imports")
		return 2
	}
	if fs.NArg() != 0 || (opts.File == "" && opts.Root == "") || (opts.File != "" && opts.Root != "") {
		fmt.Fprintln(stderr, "exactly one of --file or --root is required, with no positional arguments")
		return 2
	}
	cfg, store, cleanup, ok := loadConfigAndMaybeStore(ctx, overrides, opts.DryRun, stderr)
	if !ok {
		return 1
	}
	defer cleanup()
	opts.Timezone = cfg.Runtime.Timezone
	opts.LoadEvents = limitEventReader(readURL)
	if store != nil {
		opts.Store = store
	}
	var result any
	var err error
	switch command {
	case "import-limit-review-json":
		result, err = ingest.ImportLimitReviewSnapshots(ctx, opts)
	case "import-limit-review-corrections":
		result, err = ingest.ImportLimitReviewCorrections(ctx, opts)
	case "import-limit-performance-json":
		result, err = ingest.ImportLimitPerformanceJSON(ctx, opts)
	case "import-market-breadth-json":
		result, err = ingest.ImportMarketBreadthJSON(ctx, opts)
	default:
		fmt.Fprintln(stderr, "unsupported review command")
		return 2
	}
	if status := writeJSON(stdout, stderr, result); status != 0 {
		return status
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func limitEventReader(baseURL string) func(context.Context, string) ([]model.LimitEvent, error) {
	client := querier.NewHTTPClient(baseURL, &http.Client{Timeout: 30 * time.Second})
	return func(ctx context.Context, day string) ([]model.LimitEvent, error) {
		return querier.LoadLimitEventFacts(ctx, client, day)
	}
}
