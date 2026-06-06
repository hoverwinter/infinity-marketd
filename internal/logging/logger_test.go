package logging

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hoverwinter/infinity-marketd/internal/config"
	"go.uber.org/zap"
)

func TestNewWritesJSONToRotatingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "marketd.log")
	logger, cleanup, err := New(config.LoggingConfig{
		Level:            "debug",
		Encoding:         "json",
		OutputPaths:      []string{"file"},
		ErrorOutputPaths: []string{"stderr"},
		File: config.LoggingFileConfig{
			Path:       path,
			MaxSizeMB:  1,
			MaxBackups: 1,
			MaxAgeDays: 1,
			Compress:   false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	logger.Debug("quote service started", zap.String("server", "127.0.0.1:7709"))
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, `"msg":"quote service started"`) {
		t.Fatalf("missing message in log: %s", text)
	}
	if !strings.Contains(text, `"server":"127.0.0.1:7709"`) {
		t.Fatalf("missing field in log: %s", text)
	}
}

func TestNewRejectsUnsupportedOutput(t *testing.T) {
	_, _, err := New(config.LoggingConfig{
		Level:            "info",
		Encoding:         "console",
		OutputPaths:      []string{"socket"},
		ErrorOutputPaths: []string{"stderr"},
		File: config.LoggingFileConfig{
			MaxSizeMB:  1,
			MaxBackups: 1,
			MaxAgeDays: 1,
		},
	})
	if err == nil {
		t.Fatal("expected unsupported output error")
	}
	if !strings.Contains(err.Error(), "unsupported log output") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInitGlobalRedirectsStandardLogger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "marketd.log")
	_, cleanup, err := InitGlobal(config.LoggingConfig{
		Level:            "info",
		Encoding:         "json",
		OutputPaths:      []string{"file"},
		ErrorOutputPaths: []string{"file"},
		File: config.LoggingFileConfig{
			Path:       path,
			MaxSizeMB:  1,
			MaxBackups: 1,
			MaxAgeDays: 1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	zap.L().Info("global logger ready")
	log.Print("standard logger redirected")
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"msg":"global logger ready"`) {
		t.Fatalf("missing global log entry: %s", string(raw))
	}
	if !strings.Contains(string(raw), `"msg":"standard logger redirected"`) {
		t.Fatalf("missing standard log entry: %s", string(raw))
	}
}
