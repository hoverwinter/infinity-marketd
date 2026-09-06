package ingest

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

func TestImportG4DailyBarsLocalDryRun(t *testing.T) {
	filePath := writeIngestG4ZIP(t, ingestG4Entries("260904"))
	fetchCalled := false
	summary, err := ImportG4DailyBars(context.Background(), G4DailyImportOptions{
		File:   filePath,
		DryRun: true,
		FetchPackage: func(context.Context, time.Time, tdx.G4DayFetchOptions) ([]byte, string, error) {
			fetchCalled = true
			return nil, "", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if fetchCalled {
		t.Fatal("local import contacted remote fetcher")
	}
	if summary.Source != filePath || summary.TradeDate != "2026-09-04" || summary.Records != 6 || summary.EquityRecords != 4 || summary.NoTradeRecords != 1 || summary.NonEquityRecords != 2 {
		t.Fatalf("summary = %#v", summary)
	}
	if summary.RowsWritten != 3 || summary.RowsSkipped != 3 || len(summary.Issues) != 0 || len(summary.SHA256) != 64 {
		t.Fatalf("summary result = %#v", summary)
	}
}

func TestImportG4DailyBarsRemoteDryRun(t *testing.T) {
	raw := ingestG4ZIP(t, ingestG4Entries("260904"))
	var gotDate time.Time
	summary, err := ImportG4DailyBars(context.Background(), G4DailyImportOptions{
		Date:    "20260904",
		BaseURL: "https://example.test/g4day/",
		DryRun:  true,
		FetchPackage: func(_ context.Context, date time.Time, opts tdx.G4DayFetchOptions) ([]byte, string, error) {
			gotDate = date
			if opts.BaseURL != "https://example.test/g4day/" {
				t.Fatalf("base URL = %q", opts.BaseURL)
			}
			return raw, "https://example.test/g4day/20260904.zip", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotDate.Format("2006-01-02") != "2026-09-04" || summary.Source != "https://example.test/g4day/20260904.zip" || summary.RowsWritten != 3 {
		t.Fatalf("date=%s summary=%#v", gotDate, summary)
	}
}

func TestImportG4DailyBarsSuccessfulWriteLifecycle(t *testing.T) {
	filePath := writeIngestG4ZIP(t, ingestG4Entries("260904"))
	ops := &fakeOps{}
	var written []model.DailyBar
	now := time.Date(2026, 9, 4, 16, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	summary, err := ImportG4DailyBars(context.Background(), G4DailyImportOptions{
		File: filePath,
		Now:  func() time.Time { return now },
		ops:  ops,
		writeBars: func(_ context.Context, rows []model.DailyBar) error {
			written = append(written, rows...)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.RowsWritten != 3 || len(written) != 3 {
		t.Fatalf("summary=%#v written=%d", summary, len(written))
	}
	if len(ops.taskRuns) != 1 || ops.taskRuns[0].TaskType != "tdx_g4_daily_import" || ops.taskRuns[0].Status != "success" || ops.taskRuns[0].InputFormat != tdx.G4DayInputFormat {
		t.Fatalf("task runs = %#v", ops.taskRuns)
	}
	if len(ops.watermarks) != 1 || ops.watermarks[0].Dataset != g4DailyDataset || ops.watermarks[0].Asset != "all" || ops.watermarks[0].MaxWatermark == nil || ops.watermarks[0].MaxWatermark.Format("2006-01-02") != "2026-09-04" {
		t.Fatalf("watermarks = %#v", ops.watermarks)
	}
}

func TestImportG4DailyBarsValidationFailureWritesNoBars(t *testing.T) {
	entries := ingestG4Entries("260904")
	delete(entries, "bj260904.md1")
	filePath := writeIngestG4ZIP(t, entries)
	ops := &fakeOps{}
	writeCalled := false
	summary, err := ImportG4DailyBars(context.Background(), G4DailyImportOptions{
		File: filePath,
		ops:  ops,
		writeBars: func(context.Context, []model.DailyBar) error {
			writeCalled = true
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "missing bj .cod/.md1 pair") {
		t.Fatalf("error = %v", err)
	}
	if writeCalled || summary.RowsWritten != 0 || len(ops.watermarks) != 0 {
		t.Fatalf("write=%v summary=%#v watermarks=%#v", writeCalled, summary, ops.watermarks)
	}
	if len(ops.taskRuns) != 1 || ops.taskRuns[0].Status != "failed" {
		t.Fatalf("task runs = %#v", ops.taskRuns)
	}
}

func TestImportG4DailyBarsRejectsInvalidSourceOptions(t *testing.T) {
	if _, err := ImportG4DailyBars(context.Background(), G4DailyImportOptions{DryRun: true}); err == nil || !strings.Contains(err.Error(), "--date is required") {
		t.Fatalf("missing source error = %v", err)
	}
	if _, err := ImportG4DailyBars(context.Background(), G4DailyImportOptions{File: "x.zip", BaseURL: "https://example.test", DryRun: true}); err == nil || !strings.Contains(err.Error(), "--base-url cannot") {
		t.Fatalf("mixed source error = %v", err)
	}
}

func TestImportG4DailyBarsRejectsLocalDateMismatch(t *testing.T) {
	filePath := writeIngestG4ZIP(t, ingestG4Entries("260904"))
	_, err := ImportG4DailyBars(context.Background(), G4DailyImportOptions{File: filePath, Date: "2026-09-05", DryRun: true})
	if err == nil || !strings.Contains(err.Error(), "does not match requested date") {
		t.Fatalf("error = %v", err)
	}
}

func writeIngestG4ZIP(t *testing.T, entries map[string][]byte) string {
	t.Helper()
	filePath := filepath.Join(t.TempDir(), "20260904.zip")
	if err := os.WriteFile(filePath, ingestG4ZIP(t, entries), 0o600); err != nil {
		t.Fatal(err)
	}
	return filePath
}

func ingestG4ZIP(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, raw := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func ingestG4Entries(date string) map[string][]byte {
	return map[string][]byte{
		"sh" + date + ".cod": ingestG4Codes("600001", "110001"),
		"sh" + date + ".md1": append(ingestG4Quote(10, 12, 9, 11, 100, 1100), ingestG4Quote(100, 101, 99, 100, 200, 20000)...),
		"sz" + date + ".cod": ingestG4Codes("000001", "399001"),
		"sz" + date + ".md1": append(ingestG4Quote(20, 22, 19, 21, 300, 6300), ingestG4Quote(200, 202, 198, 201, 400, 80400)...),
		"bj" + date + ".cod": ingestG4Codes("920001", "920002"),
		"bj" + date + ".md1": append(ingestG4Quote(30, 33, 29, 32, 500, 16000), ingestG4Quote(0, 0, 0, 31, 0, 0)...),
	}
}

func ingestG4Codes(symbols ...string) []byte {
	raw := make([]byte, len(symbols)*150)
	for i, symbol := range symbols {
		copy(raw[i*150:], symbol)
	}
	return raw
}

func ingestG4Quote(open float64, high float64, low float64, closeValue float64, volume uint64, amount float64) []byte {
	raw := make([]byte, 512)
	binary.LittleEndian.PutUint64(raw[12:20], math.Float64bits(open))
	binary.LittleEndian.PutUint64(raw[20:28], math.Float64bits(high))
	binary.LittleEndian.PutUint64(raw[28:36], math.Float64bits(low))
	binary.LittleEndian.PutUint64(raw[36:44], math.Float64bits(closeValue))
	binary.LittleEndian.PutUint64(raw[56:64], volume)
	binary.LittleEndian.PutUint64(raw[72:80], math.Float64bits(amount))
	return raw
}
