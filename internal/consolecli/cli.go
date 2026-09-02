package consolecli

import (
	"context"
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
	"go.uber.org/zap"
)

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var overrides config.Overrides
	var listen string
	var consoleDist string
	fs := flagSet("infinity-console", stderr)
	config.RegisterCommonFlags(fs, &overrides)
	fs.StringVar(&listen, "listen", envOrDefault("INFINITY_CONSOLE_LISTEN", "127.0.0.1:8809"), "HTTP listen address")
	fs.StringVar(&consoleDist, "console-dist", envOrDefault("INFINITY_CONSOLE_DIST", "web/console/dist"), "Vite console dist directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if consoleDist == "" {
		fmt.Fprintln(stderr, "--console-dist is required")
		return 2
	}
	if stat, err := os.Stat(consoleDist); err != nil || !stat.IsDir() {
		if err == nil {
			err = fmt.Errorf("not a directory")
		}
		fmt.Fprintf(stderr, "console dist %q: %v\n", consoleDist, err)
		return 1
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
	server := querier.NewServer(store).WithConsoleHQDailyImporter(consoleops.HQDailyImporter(store, cfg))
	httpServer := &http.Server{
		Addr:              listen,
		Handler:           server.HandlerWithConsoleDist(consoleDist),
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Fprintf(stdout, "infinity-console listening on http://%s/console/\n", listen)
	zap.L().Info("infinity-console listening", zap.String("listen", listen), zap.String("console_dist", consoleDist))
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		zap.L().Error("infinity-console stopped", zap.Error(err))
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func flagSet(name string, stderr io.Writer) *flag.FlagSet {
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

func envOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
