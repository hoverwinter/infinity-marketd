package tdx

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHQBestIPCachePersistsSortedSuccessfulServers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bestip.json")
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	results := []ServerProbeResult{
		{Server: "down:7709", Success: false, LatencyMS: 1, Error: "timeout"},
		{Server: "slow:7709", Success: true, LatencyMS: 30},
		{Server: "fast:7709", Success: true, LatencyMS: 5},
	}

	cache, err := SaveHQBestIPCache(path, results, now, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if cache.Preferred != "fast:7709" {
		t.Fatalf("preferred = %q", cache.Preferred)
	}
	if cache.Results[0].Server != "fast:7709" || !cache.Results[0].Preferred {
		t.Fatalf("results not sorted/preferred: %#v", cache.Results)
	}

	loaded, err := LoadHQBestIPCache(path)
	if err != nil {
		t.Fatal(err)
	}
	servers := serversFromBestIPCache(loaded, now.Add(30*time.Minute), time.Hour)
	if strings.Join(servers, ",") != "fast:7709,slow:7709" {
		t.Fatalf("servers = %#v", servers)
	}
	if servers := serversFromBestIPCache(loaded, now.Add(2*time.Hour), time.Hour); len(servers) != 0 {
		t.Fatalf("expected expired cache, got %#v", servers)
	}
	if servers := serversFromBestIPCache(loaded, now.Add(30*time.Minute), 10*time.Minute); len(servers) != 0 {
		t.Fatalf("expected max-age override to expire cache, got %#v", servers)
	}
}

func TestResolveHQServersUsesFreshBestIPCacheWhenEnabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bestip.json")
	_, err := SaveHQBestIPCache(path, []ServerProbeResult{
		{Server: "slow:7709", Success: true, LatencyMS: 40},
		{Server: "fast:7709", Success: true, LatencyMS: 4},
	}, time.Now(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	got := ResolveHQServers(context.Background(), QuoteClientOptions{
		BestIP:          true,
		BestIPCachePath: path,
		BestIPMaxAge:    time.Hour,
	})
	if strings.Join(got, ",") != "fast:7709,slow:7709" {
		t.Fatalf("servers = %#v", got)
	}

	got = ResolveHQServers(context.Background(), QuoteClientOptions{
		Servers:         []string{"explicit:7709"},
		BestIP:          true,
		BestIPCachePath: path,
		BestIPMaxAge:    time.Hour,
	})
	if strings.Join(got, ",") != "explicit:7709" {
		t.Fatalf("explicit servers should win, got %#v", got)
	}
}
