package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hoverwinter/infinity-marketd/internal/ingest"
)

func TestOnlineReviewCLIOptions(t *testing.T) {
	oldTHS, oldTDX := refreshTHSLimitReview, importTDXLimitIndex
	defer func() { refreshTHSLimitReview, importTDXLimitIndex = oldTHS, oldTDX }()
	refreshTHSLimitReview = func(_ context.Context, opts ingest.THSReviewOptions) (ingest.LimitReviewImportSummary, error) {
		if !opts.DryRun || opts.Store != nil || opts.Date != "2026-09-04" {
			t.Fatalf("%+v", opts)
		}
		return ingest.LimitReviewImportSummary{DryRun: true}, nil
	}
	importTDXLimitIndex = func(_ context.Context, opts ingest.LimitIndexImportOptions) (ingest.NormalizedReviewImportSummary, error) {
		if !opts.DryRun || opts.Store != nil || opts.Since != "2016-01-01" || opts.IndexCode != "prev_ladder_perf" || len(opts.ClientOptions.Servers) != 1 {
			t.Fatalf("%+v", opts)
		}
		return ingest.NormalizedReviewImportSummary{DryRun: true}, nil
	}
	for _, args := range [][]string{{"refresh-limit-review", "--date", "2026-09-04", "--dry-run"}, {"import-limit-performance-tdx", "--index-code", "prev_ladder_perf", "--until", "2026-09-04", "--server", "127.0.0.1:7709", "--dry-run"}} {
		var out, errOut bytes.Buffer
		if status := Run(context.Background(), args, &out, &errOut); status != 0 {
			t.Fatalf("%s %s", out.String(), errOut.String())
		}
	}
}

func TestReviewImportCommandsDryRun(t *testing.T) {
	fixtures := map[string]string{
		"import-limit-review-json":        `{"trade_date":"2016-01-04","summary":{"limit_up_count":1},"limit_up_pool":[{"code":"000001","board_count":1,"status":"sealed"}],"broken":[],"limit_down":[]}`,
		"import-limit-review-corrections": `{"trade_date":"2016-01-04","mode":"upsert","reason":"backfill","events":[{"code":"000001","event_type":"limit_up","close_status":"sealed","board_count":1}]}`,
		"import-limit-performance-json":   `[{"index_code":"prev_limit_up_perf","trade_date":"2016-01-04","open":100,"high":102,"low":99,"close":101}]`,
		"import-market-breadth-json":      `[{"trade_date":"2016-01-04","up_count":1,"down_count":2,"total_count":3}]`,
	}
	for command, raw := range fixtures {
		t.Run(command, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input.json")
			if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			var out, errOut bytes.Buffer
			status := Run(context.Background(), []string{command, "--file", path, "--dry-run", "--allow-fact-replacement"}, &out, &errOut)
			if status != 0 {
				t.Fatalf("%d %s %s", status, out.String(), errOut.String())
			}
			var report struct {
				DryRun      bool `json:"dry_run"`
				RowsWritten int  `json:"rows_written"`
			}
			if err := json.Unmarshal(out.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if !report.DryRun || report.RowsWritten == 0 {
				t.Fatal(out.String())
			}
		})
	}
}
func TestReviewRootFlagDoesNotCollideWithCommonFlags(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "2016", "01")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "04.json"), []byte(`{"trade_date":"2016-01-04","summary":{},"limit_up_pool":[],"broken":[],"limit_down":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if status := Run(context.Background(), []string{"import-limit-review-json", "--root", root, "--dry-run"}, &out, &errOut); status != 0 {
		t.Fatalf("%s %s", out.String(), errOut.String())
	}
}
