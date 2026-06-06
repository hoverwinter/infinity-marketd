package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/quotesvc"
)

func TestFormatQuoteRun(t *testing.T) {
	var out bytes.Buffer
	formatQuoteRun(&out, model.QuoteServiceRun{
		RunID:            "qs-1",
		Status:           "completed_with_failures",
		StartedAt:        time.Date(2026, 6, 7, 9, 30, 0, 0, time.UTC),
		PlannedBatches:   5,
		SucceededBatches: 3,
		FailedBatches:    1,
		SkippedBatches:   1,
		RowsFetched:      240,
	})
	s := out.String()
	for _, want := range []string{"qs-1", "completed_with_failures", "batches=5/5", "succeeded=3", "failed=1", "skipped=1", "rows=240"} {
		if !strings.Contains(s, want) {
			t.Fatalf("run output missing %q: %s", want, s)
		}
	}
}

func TestFormatQuoteHealth(t *testing.T) {
	var out bytes.Buffer
	last := time.Date(2026, 6, 7, 9, 31, 0, 0, time.UTC)
	formatQuoteHealth(&out, quotesvc.Health{
		State:          "running",
		Servers:        []string{"s1", "s2"},
		HealthyServers: 1,
		Pools: []quotesvc.PoolStats{
			{Server: "s1", Open: 2, Idle: 1, HeartbeatFailures: 3},
		},
		LastSuccessfulQuote: &last,
	})
	s := out.String()
	for _, want := range []string{"health: running", "servers=1/2", "server s1 open=2 idle=1 heartbeat_failures=3", "2026-06-07 09:31:00"} {
		if !strings.Contains(s, want) {
			t.Fatalf("health output missing %q: %s", want, s)
		}
	}
}

func TestFormatQuoteHealthNeverQuoted(t *testing.T) {
	var out bytes.Buffer
	formatQuoteHealth(&out, quotesvc.Health{State: "running", Servers: []string{"s1"}, HealthyServers: 1})
	if !strings.Contains(out.String(), "last_quote=never") {
		t.Fatalf("expected last_quote=never, got %s", out.String())
	}
}
