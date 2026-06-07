package finance

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRemoteFinancialManifest(t *testing.T) {
	raw := []byte("gpcw20251231.zip,d41d8cd98f00b204e9800998ecf8427e,123\r\n\n")
	files, err := ParseRemoteFinancialManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("len = %d", len(files))
	}
	if files[0].Filename != "gpcw20251231.zip" || files[0].MD5 != "d41d8cd98f00b204e9800998ecf8427e" || files[0].Size != 123 || files[0].ReportDate != "20251231" {
		t.Fatalf("file = %#v", files[0])
	}
}

func TestParseRemoteFinancialManifestRejectsUnsafeFilename(t *testing.T) {
	_, err := ParseRemoteFinancialManifest([]byte("../gpcw20251231.zip,d41d8cd98f00b204e9800998ecf8427e,123\n"))
	if err == nil || !strings.Contains(err.Error(), "unsafe filename") {
		t.Fatalf("err = %v", err)
	}
}

func TestRemoteClientListFetchAndSkip(t *testing.T) {
	payload := financialZipBytes(t)
	sum := fmt.Sprintf("%x", md5.Sum(payload))
	manifest := fmt.Sprintf("gpcw20251231.zip,%s,%d\n", sum, len(payload))

	mux := http.NewServeMux()
	mux.HandleFunc("/tdxfin/gpcw.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(manifest))
	})
	mux.HandleFunc("/tdxfin/gpcw20251231.zip", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := RemoteClient{BaseURL: server.URL + "/tdxfin/"}
	files, err := client.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Filename != "gpcw20251231.zip" {
		t.Fatalf("files = %#v", files)
	}

	dir := t.TempDir()
	first, err := client.Fetch(context.Background(), files[0], dir)
	if err != nil {
		t.Fatal(err)
	}
	if first.Skipped || first.Bytes != int64(len(payload)) {
		t.Fatalf("first = %#v", first)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "gpcw20251231.zip")); err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("downloaded file mismatch err=%v", err)
	}

	second, err := client.Fetch(context.Background(), files[0], dir)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Skipped {
		t.Fatalf("second = %#v", second)
	}
}

func financialZipBytes(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "finance", "gpcw_one_stock.dat"))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("gpcw20251231.dat")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
