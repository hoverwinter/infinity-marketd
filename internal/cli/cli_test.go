package cli

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportDryRunCommands(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "vipdoc", "sh", "lday", "sh600519.day"), dailyRecord())
	writeFile(t, filepath.Join(root, "vipdoc", "sh", "minline", "sh600519.lc1"), lcMinuteRecord(9*60+30))
	writeFile(t, filepath.Join(root, "vipdoc", "sh", "fzline", "sh600519.lc5"), lcMinuteRecord(9*60+35))

	tests := [][]string{
		{"import-tdx-day", "--root", root, "--code", "600519", "--dry-run"},
		{"import-tdx-1m", "--root", root, "--code", "600519", "--dry-run"},
		{"import-tdx-5m", "--root", root, "--code", "600519", "--dry-run"},
	}
	for _, args := range tests {
		var out bytes.Buffer
		var errOut bytes.Buffer
		code := Run(context.Background(), args, &out, &errOut)
		if code != 0 {
			t.Fatalf("%v exit %d stderr=%s stdout=%s", args, code, errOut.String(), out.String())
		}
		if !strings.Contains(out.String(), "rows_written: 1") {
			t.Fatalf("%v output missing row count:\n%s", args, out.String())
		}
		if !strings.Contains(out.String(), "quality_issues: 0") {
			t.Fatalf("%v output has quality issue:\n%s", args, out.String())
		}
	}
}

func TestImportDayRootDryRunWithoutCode(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "vipdoc", "sh", "lday", "sh600519.day"), dailyRecord())
	writeFile(t, filepath.Join(root, `sz\lday\sz000001.day`), dailyRecord())

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"import-tdx-day", "--root", root, "--dry-run"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if !strings.Contains(out.String(), "files: 2") {
		t.Fatalf("output missing file count:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "rows_written: 2") {
		t.Fatalf("output missing row count:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "quality_issues: 0") {
		t.Fatalf("output has quality issue:\n%s", out.String())
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func dailyRecord() []byte {
	raw := make([]byte, 32)
	binary.LittleEndian.PutUint32(raw[0:4], 20260605)
	binary.LittleEndian.PutUint32(raw[4:8], 1234)
	binary.LittleEndian.PutUint32(raw[8:12], 1300)
	binary.LittleEndian.PutUint32(raw[12:16], 1200)
	binary.LittleEndian.PutUint32(raw[16:20], 1288)
	binary.LittleEndian.PutUint32(raw[20:24], math.Float32bits(100000))
	binary.LittleEndian.PutUint32(raw[24:28], 123456)
	return raw
}

func lcMinuteRecord(minute uint16) []byte {
	raw := make([]byte, 32)
	binary.LittleEndian.PutUint16(raw[0:2], uint16((2022-2004)*2048+7*100+29))
	binary.LittleEndian.PutUint16(raw[2:4], minute)
	putFloat32(raw[4:8], 12.88)
	putFloat32(raw[8:12], 12.90)
	putFloat32(raw[12:16], 12.80)
	putFloat32(raw[16:20], 12.86)
	putFloat32(raw[20:24], 100000)
	binary.LittleEndian.PutUint32(raw[24:28], 123456)
	return raw
}

func putFloat32(dst []byte, value float32) {
	binary.LittleEndian.PutUint32(dst, math.Float32bits(value))
}
