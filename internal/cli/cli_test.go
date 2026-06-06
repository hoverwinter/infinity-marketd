package cli

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
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

func TestQuoteCommandEmitsJSONAndUsesServerOverride(t *testing.T) {
	original := fetchRealtimeQuotes
	defer func() { fetchRealtimeQuotes = original }()

	var gotRequests []tdx.QuoteRequest
	var gotServers []string
	var gotBatchSize int
	fetchRealtimeQuotes = func(ctx context.Context, requests []tdx.QuoteRequest, opts tdx.QuoteClientOptions) ([]tdx.Quote, error) {
		gotRequests = append([]tdx.QuoteRequest(nil), requests...)
		gotServers = append([]string(nil), opts.Servers...)
		gotBatchSize = opts.BatchSize
		return []tdx.Quote{
			{
				Market:             "sh",
				Symbol:             "600519",
				Price:              123.45,
				LastClose:          123.00,
				Open:               123.40,
				High:               124.00,
				Low:                122.00,
				ServerTime:         "9:30:00.000",
				ServerIntradayTime: "9:30:00.000",
				Volume:             10000,
				CurrentVol:         100,
				Amount:             1000000,
				Bids: []tdx.QuoteLevel{
					{Price: 123.44, Volume: 100},
				},
				Asks: []tdx.QuoteLevel{
					{Price: 123.46, Volume: 120},
				},
			},
		}, nil
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"quote", "--server", "127.0.0.1:7709,127.0.0.2:7709", "--batch-size", "2", "--symbol", "sh:600519,000001"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if strings.Join(gotServers, ",") != "127.0.0.1:7709,127.0.0.2:7709" {
		t.Fatalf("servers = %#v", gotServers)
	}
	if gotBatchSize != 2 {
		t.Fatalf("batch size = %d", gotBatchSize)
	}
	if len(gotRequests) != 2 {
		t.Fatalf("requests = %#v", gotRequests)
	}
	if gotRequests[0] != (tdx.QuoteRequest{Market: "sh", Symbol: "600519"}) {
		t.Fatalf("first request = %#v", gotRequests[0])
	}
	if gotRequests[1] != (tdx.QuoteRequest{Market: "sz", Symbol: "000001"}) {
		t.Fatalf("second request = %#v", gotRequests[1])
	}
	var decoded []tdx.Quote
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if len(decoded) != 1 || decoded[0].Symbol != "600519" || decoded[0].Price != 123.45 {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestQuoteProbeCommandEmitsJSON(t *testing.T) {
	original := probeHQServers
	defer func() { probeHQServers = original }()

	var gotServers []string
	probeHQServers = func(ctx context.Context, servers []string, opts tdx.QuoteClientOptions) []tdx.ServerProbeResult {
		gotServers = append([]string(nil), servers...)
		return []tdx.ServerProbeResult{
			{Server: "fast:7709", Success: true, LatencyMS: 5, Preferred: true},
			{Server: "slow:7709", Success: false, LatencyMS: 20, Error: "timeout"},
		}
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"quote-probe", "--server", "slow:7709,fast:7709"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if strings.Join(gotServers, ",") != "slow:7709,fast:7709" {
		t.Fatalf("servers = %#v", gotServers)
	}
	var decoded []tdx.ServerProbeResult
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if len(decoded) != 2 || !decoded[0].Preferred {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestQuoteSweepCommandUsesStubbedWorkflow(t *testing.T) {
	original := fetchQuoteSweep
	defer func() { fetchQuoteSweep = original }()

	var got tdx.QuoteSweepOptions
	fetchQuoteSweep = func(ctx context.Context, opts tdx.QuoteSweepOptions) ([]tdx.Quote, error) {
		got = opts
		return []tdx.Quote{{Market: "sz", Symbol: "000001", Price: 10}}, nil
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"quote-sweep", "--market", "sh,sz", "--limit", "5", "--server", "127.0.0.1:7709"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if strings.Join(got.Markets, ",") != "sh,sz" || got.Limit != 5 {
		t.Fatalf("sweep opts = %#v", got)
	}
	if strings.Join(got.Client.Servers, ",") != "127.0.0.1:7709" {
		t.Fatalf("servers = %#v", got.Client.Servers)
	}
	var decoded []tdx.Quote
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if len(decoded) != 1 || decoded[0].Symbol != "000001" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestExQuoteMarketsCommandEmitsJSON(t *testing.T) {
	original := fetchExMarkets
	defer func() { fetchExMarkets = original }()

	var gotServers []string
	fetchExMarkets = func(ctx context.Context, opts tdx.ExQuoteClientOptions) ([]tdx.ExMarket, error) {
		gotServers = append([]string(nil), opts.Servers...)
		return []tdx.ExMarket{{Market: 47, Category: 3, Name: "Futures", ShortName: "CZ"}}, nil
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"exquote-markets", "--server", "127.0.0.1:7727,127.0.0.2:7727"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if strings.Join(gotServers, ",") != "127.0.0.1:7727,127.0.0.2:7727" {
		t.Fatalf("servers = %#v", gotServers)
	}
	var decoded []tdx.ExMarket
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if len(decoded) != 1 || decoded[0].Market != 47 {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestExQuoteCommandEmitsJSONAndUsesServerOverride(t *testing.T) {
	original := fetchExQuote
	defer func() { fetchExQuote = original }()

	var gotRequest tdx.ExQuoteRequest
	var gotServers []string
	fetchExQuote = func(ctx context.Context, req tdx.ExQuoteRequest, opts tdx.ExQuoteClientOptions) (tdx.ExQuote, error) {
		gotRequest = req
		gotServers = append([]string(nil), opts.Servers...)
		return tdx.ExQuote{
			Market:    req.Market,
			Code:      req.Code,
			PreClose:  3718.2,
			Open:      3717.2,
			High:      3724,
			Low:       3696.6,
			Price:     3703,
			KaiCang:   2043,
			ZongLiang: 1728,
			XianLiang: 3,
			NeiPan:    869,
			WaiPan:    859,
			ChiCang:   13340,
			Bids:      []tdx.QuoteLevel{{Price: 3702.8, Volume: 1}},
			Asks:      []tdx.QuoteLevel{{Price: 3704.4, Volume: 1}},
		}, nil
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"exquote", "--market", "47", "--code", "IF1709", "--server", "127.0.0.1:7727"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if gotRequest != (tdx.ExQuoteRequest{Market: 47, Code: "IF1709"}) {
		t.Fatalf("request = %#v", gotRequest)
	}
	if strings.Join(gotServers, ",") != "127.0.0.1:7727" {
		t.Fatalf("servers = %#v", gotServers)
	}
	var decoded tdx.ExQuote
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if decoded.Market != 47 || decoded.Code != "IF1709" || decoded.Price != 3703 {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestQuoteCommandValidatesArguments(t *testing.T) {
	tests := [][]string{
		{"quote"},
		{"quote", "--symbol", "bj:920001"},
		{"quote", "--symbol", "bad"},
	}
	for _, args := range tests {
		var out bytes.Buffer
		var errOut bytes.Buffer
		code := Run(context.Background(), args, &out, &errOut)
		if code != 2 {
			t.Fatalf("%v exit %d stderr=%s stdout=%s", args, code, errOut.String(), out.String())
		}
	}
}

func TestExQuoteCommandValidatesArguments(t *testing.T) {
	tests := [][]string{
		{"exquote"},
		{"exquote", "--market", "0", "--code", "IF1709"},
		{"exquote", "--market", "47"},
		{"exquote", "--market", "47", "--code", "IF 1709"},
	}
	for _, args := range tests {
		var out bytes.Buffer
		var errOut bytes.Buffer
		code := Run(context.Background(), args, &out, &errOut)
		if code != 2 {
			t.Fatalf("%v exit %d stderr=%s stdout=%s", args, code, errOut.String(), out.String())
		}
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
