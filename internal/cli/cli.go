package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	chstore "github.com/hoverwinter/infinity-marketd/internal/clickhouse"
	"github.com/hoverwinter/infinity-marketd/internal/config"
	"github.com/hoverwinter/infinity-marketd/internal/ingest"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

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
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
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
