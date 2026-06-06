package ingest

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

func TestImportVIPDocZipDryRun(t *testing.T) {
	zipPath := writeTestVIPDocZip(t, map[string][]byte{
		"vipdoc/sh/minline/sh600519.lc1": lcMinuteRecord(2026, 6, 5, 9*60+31),
		"vipdoc/sz/fzline/sz000001.lc5":  lcMinuteRecord(2026, 6, 5, 9*60+35),
		"vipdoc/sh/lday/sh600519.day":    make([]byte, 32),
	})

	summary, err := ImportVIPDocZip(context.Background(), VIPDocZipOptions{
		ZipPath: zipPath,
		DryRun:  true,
	})
	if err != nil {
		t.Fatalf("ImportVIPDocZip() error = %v", err)
	}
	if !summary.DryRun {
		t.Fatalf("DryRun = false")
	}
	if summary.Files != 2 || summary.Files1m != 1 || summary.Files5m != 1 {
		t.Fatalf("files summary = %#v", summary)
	}
	if summary.RowsWritten != 2 || summary.Rows1m != 1 || summary.Rows5m != 1 {
		t.Fatalf("rows summary = %#v", summary)
	}
	if summary.RowsSkipped != 0 || summary.QualityIssues != 0 {
		t.Fatalf("quality summary = %#v", summary)
	}
}

func TestImportVIPDocZipFiltersPeriodAndMarket(t *testing.T) {
	zipPath := writeTestVIPDocZip(t, map[string][]byte{
		"vipdoc/sh/minline/sh600519.lc1": lcMinuteRecord(2026, 6, 5, 9*60+31),
		"vipdoc/sz/minline/sz000001.lc1": lcMinuteRecord(2026, 6, 5, 9*60+31),
		"vipdoc/sh/fzline/sh600519.lc5":  lcMinuteRecord(2026, 6, 5, 9*60+35),
	})

	summary, err := ImportVIPDocZip(context.Background(), VIPDocZipOptions{
		ZipPath: zipPath,
		Periods: []tdx.Period{
			tdx.Period1m,
		},
		Market: "sh",
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("ImportVIPDocZip() error = %v", err)
	}
	if summary.Files != 1 || summary.Files1m != 1 || summary.Files5m != 0 {
		t.Fatalf("files summary = %#v", summary)
	}
	if summary.RowsWritten != 1 || summary.Rows1m != 1 || summary.Rows5m != 0 {
		t.Fatalf("rows summary = %#v", summary)
	}
}

func TestVIPDocZipMinutePeriod(t *testing.T) {
	tests := []struct {
		name   string
		period tdx.Period
		ok     bool
	}{
		{name: "vipdoc/sh/minline/sh600519.lc1", period: tdx.Period1m, ok: true},
		{name: `vipdoc\sh\minline\sh600519.1`, period: tdx.Period1m, ok: true},
		{name: "vipdoc/sh/fzline/sh600519.lc5", period: tdx.Period5m, ok: true},
		{name: "vipdoc/sh/fzline/sh600519.5", period: tdx.Period5m, ok: true},
		{name: "vipdoc/sh/lday/sh600519.day", ok: false},
	}
	for _, tt := range tests {
		period, ok := vipDocZipMinutePeriod(tt.name)
		if period != tt.period || ok != tt.ok {
			t.Fatalf("vipDocZipMinutePeriod(%q) = %q, %v; want %q, %v", tt.name, period, ok, tt.period, tt.ok)
		}
	}
}

func writeTestVIPDocZip(t *testing.T, files map[string][]byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vipdoc.zip")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, raw := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := w.Write(raw); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write zip: %v", err)
	}
	return path
}

func lcMinuteRecord(year int, month int, day int, minute int) []byte {
	raw := make([]byte, 32)
	dateNum := uint16((year-2004)*2048 + month*100 + day)
	binary.LittleEndian.PutUint16(raw[0:2], dateNum)
	binary.LittleEndian.PutUint16(raw[2:4], uint16(minute))
	binary.LittleEndian.PutUint32(raw[4:8], math.Float32bits(10.1))
	binary.LittleEndian.PutUint32(raw[8:12], math.Float32bits(10.2))
	binary.LittleEndian.PutUint32(raw[12:16], math.Float32bits(10.0))
	binary.LittleEndian.PutUint32(raw[16:20], math.Float32bits(10.15))
	binary.LittleEndian.PutUint32(raw[20:24], math.Float32bits(1000))
	binary.LittleEndian.PutUint32(raw[24:28], 100)
	return raw
}
