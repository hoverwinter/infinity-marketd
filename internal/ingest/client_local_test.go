package ingest

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportTDXGBBQDryRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gbbq")
	raw := make([]byte, 4)
	binary.LittleEndian.PutUint32(raw[:4], 1)
	raw = append(raw, ingestGBBQRecord(1, "600519", 20260605, 1)...)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	summary, err := ImportTDXGBBQ(context.Background(), GBBQOptions{File: path, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !summary.DryRun || summary.Dataset != "a_share_capital_change_events" || summary.RowsWritten != 1 {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestImportTDXBlockDryRunCustom(t *testing.T) {
	dir := t.TempDir()
	cfg := append(ingestFixedBytes([]byte("Watch"), 50), ingestFixedBytes([]byte("watch"), 70)...)
	if err := os.WriteFile(filepath.Join(dir, "blocknew.cfg"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "watch.blk"), []byte("1600519\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	summary, err := ImportTDXBlock(context.Background(), BlockOptions{File: dir, Scope: "custom", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !summary.DryRun || summary.RowsWritten != 3 || !strings.Contains(summary.TargetTable, "tdx_block") {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestImportTDXExDailyRejectsOfflinePackage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hsjday.zip")
	if err := os.WriteFile(path, []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ImportTDXExDaily(context.Background(), ExDailyOptions{File: path, Market: 29, Code: "A1801", DryRun: true})
	if err == nil || !strings.Contains(err.Error(), "offline package") {
		t.Fatalf("err = %v", err)
	}
}

func ingestGBBQRecord(market int, symbol string, date uint32, category byte) []byte {
	out := make([]byte, 29)
	out[0] = byte(market)
	copy(out[1:8], []byte(symbol))
	binary.LittleEndian.PutUint32(out[8:12], date)
	out[12] = category
	for offset, value := range map[int]float32{13: 1, 17: 2, 21: 3, 25: 4} {
		binary.LittleEndian.PutUint32(out[offset:offset+4], math.Float32bits(value))
	}
	return out
}

func ingestFixedBytes(value []byte, size int) []byte {
	out := make([]byte, size)
	copy(out, value)
	return out
}

func TestImportTDXExDailyDryRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "29#A1801.day")
	raw := make([]byte, 32)
	binary.LittleEndian.PutUint32(raw[0:4], 20260605)
	binary.LittleEndian.PutUint32(raw[4:8], math.Float32bits(1))
	binary.LittleEndian.PutUint32(raw[8:12], math.Float32bits(2))
	binary.LittleEndian.PutUint32(raw[12:16], math.Float32bits(1))
	binary.LittleEndian.PutUint32(raw[16:20], math.Float32bits(1.5))
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	summary, err := ImportTDXExDaily(context.Background(), ExDailyOptions{File: path, Market: 29, Code: "A1801", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if summary.RowsWritten != 1 || summary.TargetTable != "tdx_ex_bars_1d" || summary.InputFormat == "" {
		t.Fatalf("summary = %#v", summary)
	}
}
