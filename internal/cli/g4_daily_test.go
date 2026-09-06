package cli

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

func TestImportTDXG4DayRemoteDryRun(t *testing.T) {
	original := fetchG4DayPackage
	defer func() { fetchG4DayPackage = original }()
	payload := cliG4DayZIP(t, "260904")
	var gotDate string
	var gotBaseURL string
	fetchG4DayPackage = func(_ context.Context, date time.Time, opts tdx.G4DayFetchOptions) ([]byte, string, error) {
		gotDate = date.Format("2006-01-02")
		gotBaseURL = opts.BaseURL
		return payload, "https://example.test/g4day/20260904.zip", nil
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"import-tdx-g4-day", "--date", "2026-09-04", "--base-url", "https://example.test/g4day/", "--dry-run"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	for _, want := range []string{
		"mode: dry-run",
		"dataset: a_share_bars_1d",
		"target_table: a_share_bars_1d",
		"source: https://example.test/g4day/20260904.zip",
		"trade_date: 2026-09-04",
		"records: 6",
		"sh_records: 2",
		"sz_records: 2",
		"bj_records: 2",
		"equity_records: 4",
		"no_trade_records: 1",
		"non_equity_records: 2",
		"rows_written: 3",
		"rows_skipped: 3",
		"quality_issues: 0",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q:\n%s", want, out.String())
		}
	}
	if gotDate != "2026-09-04" || gotBaseURL != "https://example.test/g4day/" {
		t.Fatalf("date=%q baseURL=%q", gotDate, gotBaseURL)
	}
}

func TestImportTDXG4DayLocalDryRun(t *testing.T) {
	original := fetchG4DayPackage
	defer func() { fetchG4DayPackage = original }()
	fetchG4DayPackage = func(context.Context, time.Time, tdx.G4DayFetchOptions) ([]byte, string, error) {
		t.Fatal("local replay must not fetch")
		return nil, "", nil
	}
	filePath := filepath.Join(t.TempDir(), "20260904.zip")
	writeFile(t, filePath, cliG4DayZIP(t, "260904"))

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"import-tdx-g4-day", "--file", filePath, "--date", "20260904", "--dry-run"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if !strings.Contains(out.String(), "source: "+filePath) || !strings.Contains(out.String(), "rows_written: 3") || !strings.Contains(out.String(), "sha256: ") {
		t.Fatalf("stdout=%s", out.String())
	}
}

func TestImportTDXG4DayRejectsInvalidSources(t *testing.T) {
	tests := [][]string{
		{"import-tdx-g4-day", "--dry-run"},
		{"import-tdx-g4-day", "--file", "x.zip", "--base-url", "https://example.test", "--dry-run"},
	}
	for _, args := range tests {
		var out bytes.Buffer
		var errOut bytes.Buffer
		code := Run(context.Background(), args, &out, &errOut)
		if code != 2 {
			t.Fatalf("%v exit %d stderr=%s stdout=%s", args, code, errOut.String(), out.String())
		}
	}
}

func cliG4DayZIP(t *testing.T, date string) []byte {
	t.Helper()
	return zipBytes(t, map[string][]byte{
		"sh" + date + ".cod": cliG4DayCodes("600001", "110001"),
		"sh" + date + ".md1": append(cliG4DayQuote(10, 12, 9, 11, 100, 1100), cliG4DayQuote(100, 101, 99, 100, 200, 20000)...),
		"sz" + date + ".cod": cliG4DayCodes("000001", "399001"),
		"sz" + date + ".md1": append(cliG4DayQuote(20, 22, 19, 21, 300, 6300), cliG4DayQuote(200, 202, 198, 201, 400, 80400)...),
		"bj" + date + ".cod": cliG4DayCodes("920001", "920002"),
		"bj" + date + ".md1": append(cliG4DayQuote(30, 33, 29, 32, 500, 16000), cliG4DayQuote(0, 0, 0, 31, 0, 0)...),
	})
}

func cliG4DayCodes(symbols ...string) []byte {
	raw := make([]byte, len(symbols)*150)
	for i, symbol := range symbols {
		copy(raw[i*150:], symbol)
	}
	return raw
}

func cliG4DayQuote(open float64, high float64, low float64, closeValue float64, volume uint64, amount float64) []byte {
	raw := make([]byte, 512)
	binary.LittleEndian.PutUint64(raw[12:20], math.Float64bits(open))
	binary.LittleEndian.PutUint64(raw[20:28], math.Float64bits(high))
	binary.LittleEndian.PutUint64(raw[28:36], math.Float64bits(low))
	binary.LittleEndian.PutUint64(raw[36:44], math.Float64bits(closeValue))
	binary.LittleEndian.PutUint64(raw[56:64], volume)
	binary.LittleEndian.PutUint64(raw[72:80], math.Float64bits(amount))
	return raw
}
