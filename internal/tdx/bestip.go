package tdx

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	defaultHQBestIPCacheName = "tdx-hq-bestip.json"
	defaultHQBestIPMaxAge    = 24 * time.Hour
)

type HQBestIPCache struct {
	Version     int                 `json:"version"`
	GeneratedAt time.Time           `json:"generated_at"`
	ExpiresAt   time.Time           `json:"expires_at"`
	Preferred   string              `json:"preferred"`
	Results     []ServerProbeResult `json:"results"`
}

func DefaultHQBestIPCachePath() string {
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		return filepath.Join(dir, "infinity-marketd", defaultHQBestIPCacheName)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".cache", "infinity-marketd", defaultHQBestIPCacheName)
	}
	return defaultHQBestIPCacheName
}

func DefaultHQBestIPMaxAge() time.Duration {
	return defaultHQBestIPMaxAge
}

func LoadHQBestIPCache(path string) (HQBestIPCache, error) {
	if path == "" {
		path = DefaultHQBestIPCachePath()
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return HQBestIPCache{}, err
	}
	var cache HQBestIPCache
	if err := json.Unmarshal(raw, &cache); err != nil {
		return HQBestIPCache{}, fmt.Errorf("parse TDX HQ bestip cache %s: %w", path, err)
	}
	return cache, nil
}

func SaveHQBestIPCache(path string, results []ServerProbeResult, now time.Time, maxAge time.Duration) (HQBestIPCache, error) {
	if path == "" {
		path = DefaultHQBestIPCachePath()
	}
	if maxAge <= 0 {
		maxAge = defaultHQBestIPMaxAge
	}
	if now.IsZero() {
		now = time.Now()
	}
	results = normalizedProbeResults(results)
	cache := HQBestIPCache{
		Version:     1,
		GeneratedAt: now,
		ExpiresAt:   now.Add(maxAge),
		Preferred:   BestHQServer(results),
		Results:     results,
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return cache, fmt.Errorf("create TDX HQ bestip cache dir: %w", err)
	}
	raw, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return cache, err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return cache, fmt.Errorf("write TDX HQ bestip cache %s: %w", path, err)
	}
	return cache, nil
}

func ResolveHQServers(ctx context.Context, opts QuoteClientOptions) []string {
	if hasExplicitHQServers(opts) || !opts.BestIP {
		return NormalizeHQServers(opts)
	}
	now := time.Now()
	if opts.BestIPMaxAge <= 0 {
		opts.BestIPMaxAge = defaultHQBestIPMaxAge
	}
	if cache, err := LoadHQBestIPCache(opts.BestIPCachePath); err == nil {
		if servers := serversFromBestIPCache(cache, now, opts.BestIPMaxAge); len(servers) > 0 {
			return servers
		}
	}
	if opts.BestIPRefresh {
		results := ProbeHQServers(ctx, nil, QuoteClientOptions{Timeout: opts.Timeout})
		if servers := successfulProbeServers(results); len(servers) > 0 {
			_, _ = SaveHQBestIPCache(opts.BestIPCachePath, results, now, opts.BestIPMaxAge)
			return servers
		}
	}
	return NormalizeHQServers(QuoteClientOptions{})
}

func RefreshHQBestIPCache(ctx context.Context, servers []string, opts QuoteClientOptions) (HQBestIPCache, error) {
	results := ProbeHQServers(ctx, servers, opts)
	return SaveHQBestIPCache(opts.BestIPCachePath, results, time.Now(), opts.BestIPMaxAge)
}

func hasExplicitHQServers(opts QuoteClientOptions) bool {
	return opts.Server != "" || len(opts.Servers) > 0
}

func serversFromBestIPCache(cache HQBestIPCache, now time.Time, maxAge time.Duration) []string {
	if cache.GeneratedAt.IsZero() {
		return nil
	}
	if !cache.ExpiresAt.IsZero() && now.After(cache.ExpiresAt) {
		return nil
	}
	if maxAge > 0 && now.Sub(cache.GeneratedAt) > maxAge {
		return nil
	}
	return successfulProbeServers(cache.Results)
}

func successfulProbeServers(results []ServerProbeResult) []string {
	results = normalizedProbeResults(results)
	servers := make([]string, 0, len(results))
	for _, result := range results {
		if result.Success && result.Server != "" {
			servers = append(servers, result.Server)
		}
	}
	return servers
}

func normalizedProbeResults(results []ServerProbeResult) []ServerProbeResult {
	out := append([]ServerProbeResult(nil), results...)
	SortProbeResults(out)
	preferred := BestHQServer(out)
	for i := range out {
		out[i].Preferred = out[i].Server == preferred && preferred != ""
	}
	return out
}

func SortProbeResults(results []ServerProbeResult) {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Success != results[j].Success {
			return results[i].Success
		}
		return results[i].LatencyMS < results[j].LatencyMS
	})
}
