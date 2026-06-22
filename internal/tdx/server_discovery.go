package tdx

import (
	"context"
	"sort"
	"sync"
	"time"
)

var DefaultSPServers = []string{
	"121.36.248.138:7709",
	"123.60.47.136:7709",
	"121.37.207.165:7709",
}

var DefaultFundServers = []string{
	"112.74.214.43:7727",
	"120.25.218.6:7727",
	"47.107.75.159:7727",
	"47.106.204.218:7727",
	"47.106.209.131:7727",
	"139.9.191.175:7727",
	"47.115.94.72:7727",
	"106.14.95.149:7727",
	"47.102.108.214:7727",
	"47.103.86.229:7727",
	"47.103.88.146:7727",
	"116.205.143.214:7727",
	"124.71.223.19:7727",
}

type TDXServerCandidate struct {
	Protocol string `json:"protocol"`
	Server   string `json:"server"`
}

func SPServerCandidates() []TDXServerCandidate {
	return serverCandidates("sp", DefaultSPServers)
}

func FundServerCandidates() []TDXServerCandidate {
	return serverCandidates("fund", DefaultFundServers)
}

func serverCandidates(protocol string, servers []string) []TDXServerCandidate {
	out := make([]TDXServerCandidate, 0, len(servers))
	for _, server := range servers {
		out = append(out, TDXServerCandidate{Protocol: protocol, Server: server})
	}
	return out
}

func NormalizeSPServers(servers []string) []string {
	return normalizeProtocolServers(servers, DefaultSPServers)
}

func NormalizeFundServers(servers []string) []string {
	return normalizeProtocolServers(servers, DefaultFundServers)
}

func normalizeProtocolServers(servers []string, defaults []string) []string {
	if len(servers) == 0 {
		servers = defaults
	}
	seen := make(map[string]struct{}, len(servers))
	out := make([]string, 0, len(servers))
	for _, server := range servers {
		if server == "" {
			continue
		}
		if _, ok := seen[server]; ok {
			continue
		}
		seen[server] = struct{}{}
		out = append(out, server)
	}
	return out
}

func ProbeSPServers(ctx context.Context, servers []string, timeout time.Duration) []ServerProbeResult {
	return probeProtocolServers(ctx, NormalizeSPServers(servers), timeout, func(ctx context.Context, server string, timeout time.Duration) error {
		session, err := OpenSPSession(ctx, server, timeout)
		if err != nil {
			return err
		}
		return session.Close()
	})
}

func ProbeFundServers(ctx context.Context, servers []string, timeout time.Duration) []ServerProbeResult {
	return probeProtocolServers(ctx, NormalizeFundServers(servers), timeout, func(ctx context.Context, server string, timeout time.Duration) error {
		session, err := OpenFundSession(ctx, server, timeout)
		if err != nil {
			return err
		}
		return session.Close()
	})
}

func BestSPServer(results []ServerProbeResult) string {
	return bestProtocolServer(results)
}

func BestFundServer(results []ServerProbeResult) string {
	return bestProtocolServer(results)
}

func probeProtocolServers(ctx context.Context, servers []string, timeout time.Duration, probe func(context.Context, string, time.Duration) error) []ServerProbeResult {
	if ctx == nil {
		ctx = context.Background()
	}
	results := make([]ServerProbeResult, len(servers))
	var wg sync.WaitGroup
	for i, server := range servers {
		wg.Add(1)
		go func(i int, server string) {
			defer wg.Done()
			started := time.Now()
			err := probe(ctx, server, timeout)
			result := ServerProbeResult{Server: server, Success: err == nil, LatencyMS: time.Since(started).Milliseconds()}
			if err != nil {
				result.Error = err.Error()
			}
			results[i] = result
		}(i, server)
	}
	wg.Wait()
	sortProbePreferred(results)
	return results
}

func sortProbePreferred(results []ServerProbeResult) {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Success != results[j].Success {
			return results[i].Success
		}
		return results[i].LatencyMS < results[j].LatencyMS
	})
	if best := bestProtocolServer(results); best != "" {
		for i := range results {
			results[i].Preferred = results[i].Server == best
		}
	}
}

func bestProtocolServer(results []ServerProbeResult) string {
	bestIndex := -1
	for i, result := range results {
		if !result.Success {
			continue
		}
		if bestIndex < 0 || result.LatencyMS < results[bestIndex].LatencyMS {
			bestIndex = i
		}
	}
	if bestIndex < 0 {
		return ""
	}
	return results[bestIndex].Server
}
