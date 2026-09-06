package infinitycli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderCommandsUseHTTP(t *testing.T) {
	for _, tc := range []struct {
		command  string
		flags    []string
		path     string
		response string
	}{
		{"providers", nil, "/api/providers", `[{"id":"ths","bars":[],"board_kinds":["concept"]}]`},
		{"provider-bars", []string{"--provider", "ths", "--market", "board", "--symbol", "885611", "--since", "2026-09-03", "--until", "2026-09-04"}, "/api/providers/ths/bars", `{"provider":"ths","bars":[]}`},
		{"provider-boards", []string{"--provider", "ths", "--kind", "concept"}, "/api/providers/ths/boards", `{"provider":"ths","boards":[]}`},
		{"provider-board", []string{"--provider", "ths", "--kind", "concept", "--code", "301558"}, "/api/providers/ths/boards/concept/301558", `{"provider":"ths","board":{"code":"301558"}}`},
		{"provider-bars", []string{"--provider", "eastmoney", "--market", "board", "--symbol", "BK1027", "--since", "2026-09-03", "--until", "2026-09-04"}, "/api/providers/eastmoney/bars", `{"provider":"eastmoney","bars":[]}`},
		{"provider-boards", []string{"--provider", "eastmoney", "--kind", "industry"}, "/api/providers/eastmoney/boards", `{"provider":"eastmoney","boards":[]}`},
		{"provider-board", []string{"--provider", "eastmoney", "--kind", "industry", "--code", "BK1027"}, "/api/providers/eastmoney/boards/industry/BK1027", `{"provider":"eastmoney","board":{"code":"BK1027"}}`},
	} {
		t.Run(tc.command, func(t *testing.T) {
			provider, symbol := "ths", "885611"
			if strings.Contains(tc.path, "/eastmoney/") {
				provider, symbol = "eastmoney", "BK1027"
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != tc.path {
					t.Errorf("unexpected HTTP request: %s %s", r.Method, r.URL)
				}
				if tc.command == "provider-bars" {
					v := r.URL.Query()
					if v.Get("symbol") != symbol || v.Get("kind") != "index" || v.Get("since") != "2026-09-03" || v.Get("until") != "2026-09-04" || v.Get("period") != "1d" || v.Get("adjust") != "none" {
						t.Errorf("query=%v", v)
					}
				}
				fmt.Fprint(w, tc.response)
			}))
			defer server.Close()
			args := append([]string{"querier", tc.command, "--url", server.URL}, tc.flags...)
			var out, stderr bytes.Buffer
			if code := Run(context.Background(), args, &out, &stderr); code != 0 || calls != 1 || !strings.Contains(out.String(), provider) {
				t.Fatalf("exit=%d calls=%d stderr=%s stdout=%s", code, calls, stderr.String(), out.String())
			}
		})
	}
}

func TestProviderCLIRequiresExplicitSourceAndReportsErrors(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"querier", "provider-bars"}, &out, &stderr); code != 2 || !strings.Contains(stderr.String(), "--provider is required") {
		t.Fatalf("exit=%d %s", code, stderr.String())
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		fmt.Fprint(w, `{"error":"THS upstream unavailable"}`)
	}))
	defer server.Close()
	out.Reset()
	stderr.Reset()
	if code := Run(context.Background(), []string{"querier", "provider-boards", "--provider", "ths", "--kind", "concept", "--url", server.URL}, &out, &stderr); code != 1 || out.Len() != 0 || !strings.Contains(stderr.String(), "THS upstream unavailable") {
		t.Fatalf("exit=%d %s", code, stderr.String())
	}
}
