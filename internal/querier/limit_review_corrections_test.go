package querier

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCorrectionHTTPGuards(t *testing.T) {
	const path = "/api/console/imports/limit-review-corrections"
	calls := 0
	importer := func(_ context.Context, payload []byte, dry bool) (LimitCorrectionImportResult, error) {
		calls++
		if string(payload) != `{"mode":"upsert"}` {
			t.Fatal(string(payload))
		}
		return LimitCorrectionImportResult{DryRun: dry}, nil
	}
	h := NewServer(&fakeRepo{}).WithLimitCorrectionImporter("test-secret", importer).Handler()
	for _, tc := range []struct {
		name, query, body, token, origin, content string
		status                                    int
	}{
		{"default-preview", "", `{ "mode": "upsert" }`, "test-secret", "", "application/json", 200},
		{"write", "?dry_run=false", `{"mode":"upsert"}`, "test-secret", "", "application/json", 200},
		{"no-auth", "", `{}`, "", "", "application/json", 401},
		{"bad-auth", "", `{}`, "wrong", "", "application/json", 401},
		{"origin", "", `{}`, "test-secret", "https://untrusted.example", "application/json", 403},
		{"content-type", "", `{}`, "test-secret", "", "text/plain", 415},
		{"too-large", "", strings.Repeat(" ", 4<<20) + "{}", "test-secret", "", "application/json", 413},
		{"array", "", `[]`, "test-secret", "", "application/json", 400},
		{"multiple", "", `{} {}`, "test-secret", "", "application/json", 400},
		{"invalid-mode", "?dry_run=0", `{}`, "test-secret", "", "application/json", 400},
		{"duplicate-mode", "?dry_run=true&dry_run=false", `{}`, "test-secret", "", "application/json", 400},
		{"unknown-param", "?file=/tmp/input", `{}`, "test-secret", "", "application/json", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", path+tc.query, strings.NewReader(tc.body))
			r.Header.Set("Authorization", "Bearer "+tc.token)
			r.Header.Set("Content-Type", tc.content)
			r.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			if tc.name == "default-preview" && !strings.Contains(w.Body.String(), `"dry_run":true`) {
				t.Fatal(w.Body.String())
			}
		})
	}
	if calls != 2 {
		t.Fatal(calls)
	}
	for _, server := range []*Server{NewServer(&fakeRepo{}), NewServer(&fakeRepo{}).WithLimitCorrectionImporter("", importer)} {
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader(`{}`)))
		if w.Code != 404 {
			t.Fatal("read-only server exposed correction route")
		}
	}
}

func TestCorrectionHTTPSerializesAndReportsErrors(t *testing.T) {
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	h := NewServer(&fakeRepo{}).WithLimitCorrectionImporter("secret", func(context.Context, []byte, bool) (LimitCorrectionImportResult, error) {
		close(entered)
		<-release
		return LimitCorrectionImportResult{RunID: "failed-run"}, errors.New("write failed")
	}).Handler()
	request := func() *http.Request {
		r := httptest.NewRequest("POST", "/api/console/imports/limit-review-corrections?dry_run=false", strings.NewReader(`{}`))
		r.Header.Set("Authorization", "Bearer secret")
		r.Header.Set("Content-Type", "application/json")
		return r
	}
	first := httptest.NewRecorder()
	go func() { defer close(done); h.ServeHTTP(first, request()) }()
	<-entered
	second := httptest.NewRecorder()
	h.ServeHTTP(second, request())
	close(release)
	<-done
	if second.Code != 409 || second.Header().Get("Retry-After") == "" {
		t.Fatal(second.Code)
	}
	if first.Code != 500 || !strings.Contains(first.Body.String(), "failed-run") {
		t.Fatal(first.Body.String())
	}
}
