package tdx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInferMarketFromCode(t *testing.T) {
	tests := map[string]string{
		"920002": "bj",
		"830000": "bj",
		"430000": "bj",
		"600519": "sh",
		"900001": "sh",
		"000001": "sz",
	}
	for code, want := range tests {
		if got := InferMarketFromCode(code); got != want {
			t.Fatalf("InferMarketFromCode(%q) = %q, want %q", code, got, want)
		}
	}
}

func TestParseMarketSymbol(t *testing.T) {
	market, symbol, err := ParseMarketSymbol("/data/vipdoc/bj/lday/bj920002.day", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if market != "bj" || symbol != "920002" {
		t.Fatalf("got %s %s", market, symbol)
	}

	market, symbol, err = ParseMarketSymbol(`/data/sh\lday\sh600519.day`, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if market != "sh" || symbol != "600519" {
		t.Fatalf("got %s %s", market, symbol)
	}
}

func TestDiscoverFiles(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		filepath.Join("vipdoc", "sh", "lday", "sh600519.day"),
		`sz\lday\sz000001.day`,
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := DiscoverFiles(root, PeriodDay, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("len(files) = %d, want 2: %v", len(files), files)
	}
}
