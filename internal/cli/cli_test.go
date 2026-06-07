package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestImportVIPDocZipDryRunCommand(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "vipdoc.zip")
	writeZip(t, zipPath, map[string][]byte{
		"vipdoc/sh/minline/sh600519.lc1": lcMinuteRecord(9*60 + 31),
		"vipdoc/sh/fzline/sh600519.lc5":  lcMinuteRecord(9*60 + 35),
		"vipdoc/sh/lday/sh600519.day":    dailyRecord(),
	})

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"import-tdx-vipdoc-zip", "--file", zipPath, "--dry-run"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	for _, want := range []string{
		"mode: dry-run",
		"files: 2",
		"files_1m: 1",
		"files_5m: 1",
		"rows_written: 2",
		"quality_issues: 0",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, out.String())
		}
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
	code := Run(context.Background(), []string{"quote", "--server", "127.0.0.1:7709,127.0.0.2:7709", "--batch-size", "2", "--symbol", "sh:600519,000001,bj:920001,920799"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if strings.Join(gotServers, ",") != "127.0.0.1:7709,127.0.0.2:7709" {
		t.Fatalf("servers = %#v", gotServers)
	}
	if gotBatchSize != 2 {
		t.Fatalf("batch size = %d", gotBatchSize)
	}
	if len(gotRequests) != 4 {
		t.Fatalf("requests = %#v", gotRequests)
	}
	if gotRequests[0] != (tdx.QuoteRequest{Market: "sh", Symbol: "600519"}) {
		t.Fatalf("first request = %#v", gotRequests[0])
	}
	if gotRequests[1] != (tdx.QuoteRequest{Market: "sz", Symbol: "000001"}) {
		t.Fatalf("second request = %#v", gotRequests[1])
	}
	if gotRequests[2] != (tdx.QuoteRequest{Market: "bj", Symbol: "920001"}) {
		t.Fatalf("third request = %#v", gotRequests[2])
	}
	if gotRequests[3] != (tdx.QuoteRequest{Market: "bj", Symbol: "920799"}) {
		t.Fatalf("fourth request = %#v", gotRequests[3])
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

func TestQuoteBestIPCommandRefreshesCache(t *testing.T) {
	original := refreshHQBestIPCache
	defer func() { refreshHQBestIPCache = original }()

	var gotServers []string
	var gotOpts tdx.QuoteClientOptions
	refreshHQBestIPCache = func(ctx context.Context, servers []string, opts tdx.QuoteClientOptions) (tdx.HQBestIPCache, error) {
		gotServers = append([]string(nil), servers...)
		gotOpts = opts
		return tdx.HQBestIPCache{
			Version:   1,
			Preferred: "fast:7709",
			Results: []tdx.ServerProbeResult{
				{Server: "fast:7709", Success: true, LatencyMS: 5, Preferred: true},
				{Server: "slow:7709", Success: true, LatencyMS: 20},
			},
		}, nil
	}

	cachePath := filepath.Join(t.TempDir(), "bestip.json")
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{
		"quote-bestip",
		"--server", "slow:7709,fast:7709",
		"--cache", cachePath,
		"--max-age", "2h",
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if strings.Join(gotServers, ",") != "slow:7709,fast:7709" {
		t.Fatalf("servers = %#v", gotServers)
	}
	if gotOpts.BestIPCachePath != cachePath || gotOpts.BestIPMaxAge != 2*time.Hour {
		t.Fatalf("opts = %#v", gotOpts)
	}
	var decoded tdx.HQBestIPCache
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if decoded.Preferred != "fast:7709" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestQuoteCommandWiresBestIPOptions(t *testing.T) {
	original := fetchRealtimeQuotes
	defer func() { fetchRealtimeQuotes = original }()

	var gotOpts tdx.QuoteClientOptions
	fetchRealtimeQuotes = func(ctx context.Context, requests []tdx.QuoteRequest, opts tdx.QuoteClientOptions) ([]tdx.Quote, error) {
		gotOpts = opts
		return []tdx.Quote{{Market: "sh", Symbol: "600519", Price: 1}}, nil
	}

	cachePath := filepath.Join(t.TempDir(), "bestip.json")
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{
		"quote",
		"--symbol", "sh:600519",
		"--bestip",
		"--bestip-cache", cachePath,
		"--bestip-max-age", "3h",
		"--bestip-refresh=false",
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if !gotOpts.BestIP || gotOpts.BestIPCachePath != cachePath || gotOpts.BestIPMaxAge != 3*time.Hour || gotOpts.BestIPRefresh {
		t.Fatalf("opts = %#v", gotOpts)
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

func TestQuoteSweepCommandAcceptsExplicitBeijingSymbols(t *testing.T) {
	original := fetchQuoteSweep
	defer func() { fetchQuoteSweep = original }()

	var got tdx.QuoteSweepOptions
	fetchQuoteSweep = func(ctx context.Context, opts tdx.QuoteSweepOptions) ([]tdx.Quote, error) {
		got = opts
		return []tdx.Quote{{Market: "bj", Symbol: "920001", Price: 10}}, nil
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"quote-sweep", "--symbol", "920001,bj:920799", "--server", "127.0.0.1:7709"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if len(got.Requests) != 2 {
		t.Fatalf("requests = %#v", got.Requests)
	}
	if got.Requests[0] != (tdx.QuoteRequest{Market: "bj", Symbol: "920001"}) {
		t.Fatalf("first request = %#v", got.Requests[0])
	}
	if got.Requests[1] != (tdx.QuoteRequest{Market: "bj", Symbol: "920799"}) {
		t.Fatalf("second request = %#v", got.Requests[1])
	}
	if strings.Join(got.Client.Servers, ",") != "127.0.0.1:7709" {
		t.Fatalf("servers = %#v", got.Client.Servers)
	}
	var decoded []tdx.Quote
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if len(decoded) != 1 || decoded[0].Market != "bj" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestHQReadCommandsUseStubbedWorkflows(t *testing.T) {
	origSecurityBars := fetchHQSecurityBars
	origIndexBars := fetchHQIndexBars
	origMinute := fetchHQMinuteTime
	origHistoryMinute := fetchHQHistoryMinuteTime
	origTransactions := fetchHQTransactions
	origHistoryTransactions := fetchHQHistoryTransactions
	origCategories := fetchHQCompanyInfoCategories
	origContent := fetchHQCompanyInfoContent
	origXDXR := fetchHQXDXRInfo
	origFinance := fetchHQFinanceInfo
	origBlockMeta := fetchHQBlockMeta
	origBlock := fetchHQBlockMembers
	defer func() {
		fetchHQSecurityBars = origSecurityBars
		fetchHQIndexBars = origIndexBars
		fetchHQMinuteTime = origMinute
		fetchHQHistoryMinuteTime = origHistoryMinute
		fetchHQTransactions = origTransactions
		fetchHQHistoryTransactions = origHistoryTransactions
		fetchHQCompanyInfoCategories = origCategories
		fetchHQCompanyInfoContent = origContent
		fetchHQXDXRInfo = origXDXR
		fetchHQFinanceInfo = origFinance
		fetchHQBlockMeta = origBlockMeta
		fetchHQBlockMembers = origBlock
	}()

	var seenServers []string
	captureServers := func(opts tdx.QuoteClientOptions) {
		seenServers = append([]string(nil), opts.Servers...)
	}
	fetchHQSecurityBars = func(ctx context.Context, req tdx.HQBarsRequest, opts tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
		captureServers(opts)
		if req.Market != "sh" || req.Symbol != "600519" || req.Count != 2 {
			t.Fatalf("security bars req = %#v", req)
		}
		return []tdx.HQBar{{Market: req.Market, Symbol: req.Symbol, Close: 1}}, nil
	}
	fetchHQIndexBars = func(ctx context.Context, req tdx.HQBarsRequest, opts tdx.QuoteClientOptions) ([]tdx.HQBar, error) {
		captureServers(opts)
		return []tdx.HQBar{{Market: req.Market, Symbol: req.Symbol, UpCount: 1}}, nil
	}
	fetchHQMinuteTime = func(ctx context.Context, req tdx.HQMinuteRequest, opts tdx.QuoteClientOptions) ([]tdx.HQMinutePoint, error) {
		captureServers(opts)
		return []tdx.HQMinutePoint{{Market: req.Market, Symbol: req.Symbol, Time: "09:30", Price: 1}}, nil
	}
	fetchHQHistoryMinuteTime = func(ctx context.Context, req tdx.HQMinuteRequest, date int, opts tdx.QuoteClientOptions) ([]tdx.HQMinutePoint, error) {
		captureServers(opts)
		if date != 20260605 {
			t.Fatalf("history minute date = %d", date)
		}
		return []tdx.HQMinutePoint{}, nil
	}
	fetchHQTransactions = func(ctx context.Context, req tdx.HQMinuteRequest, start, count int, opts tdx.QuoteClientOptions) ([]tdx.HQTransaction, error) {
		captureServers(opts)
		return []tdx.HQTransaction{{Market: req.Market, Symbol: req.Symbol, Price: 1}}, nil
	}
	fetchHQHistoryTransactions = func(ctx context.Context, req tdx.HQMinuteRequest, date, start, count int, opts tdx.QuoteClientOptions) ([]tdx.HQTransaction, error) {
		captureServers(opts)
		if date != 20260605 {
			t.Fatalf("history transaction date = %d", date)
		}
		return []tdx.HQTransaction{{Date: "2026-06-05"}}, nil
	}
	fetchHQCompanyInfoCategories = func(ctx context.Context, req tdx.HQMinuteRequest, opts tdx.QuoteClientOptions) ([]tdx.HQCompanyInfoCategory, error) {
		captureServers(opts)
		return []tdx.HQCompanyInfoCategory{{Market: req.Market, Symbol: req.Symbol, Name: "notice"}}, nil
	}
	fetchHQCompanyInfoContent = func(ctx context.Context, req tdx.HQMinuteRequest, filename string, start, length uint32, opts tdx.QuoteClientOptions) (tdx.HQCompanyInfoContent, error) {
		captureServers(opts)
		if filename != "600519.txt" || start != 1 || length != 2 {
			t.Fatalf("content args filename=%q start=%d length=%d", filename, start, length)
		}
		return tdx.HQCompanyInfoContent{Market: req.Market, Symbol: req.Symbol, Filename: filename, Content: "hello"}, nil
	}
	fetchHQXDXRInfo = func(ctx context.Context, req tdx.HQMinuteRequest, opts tdx.QuoteClientOptions) ([]tdx.HQXDXRInfo, error) {
		captureServers(opts)
		return []tdx.HQXDXRInfo{{Market: req.Market, Symbol: req.Symbol, Category: 1, Name: "除权除息"}}, nil
	}
	fetchHQFinanceInfo = func(ctx context.Context, req tdx.HQMinuteRequest, opts tdx.QuoteClientOptions) (tdx.HQFinanceInfo, error) {
		captureServers(opts)
		return tdx.HQFinanceInfo{Market: req.Market, Symbol: req.Symbol, IPODate: 20010827}, nil
	}
	fetchHQBlockMeta = func(ctx context.Context, file string, opts tdx.QuoteClientOptions) (tdx.HQBlockMeta, error) {
		captureServers(opts)
		if file != "block.dat" {
			t.Fatalf("file = %q", file)
		}
		return tdx.HQBlockMeta{File: file, Size: 10}, nil
	}
	fetchHQBlockMembers = func(ctx context.Context, file string, opts tdx.QuoteClientOptions) ([]tdx.HQBlockMember, error) {
		captureServers(opts)
		return []tdx.HQBlockMember{{BlockName: "A", Code: "600519", Market: "sh", Symbol: "600519"}}, nil
	}

	commands := [][]string{
		{"hq-bars", "--market", "sh", "--symbol", "600519", "--count", "2", "--server", "127.0.0.1:7709"},
		{"hq-index-bars", "--market", "sh", "--symbol", "000001", "--server", "127.0.0.1:7709"},
		{"hq-minute", "--market", "sh", "--symbol", "600519", "--server", "127.0.0.1:7709"},
		{"hq-history-minute", "--market", "sh", "--symbol", "600519", "--date", "20260605", "--server", "127.0.0.1:7709"},
		{"hq-transactions", "--market", "sh", "--symbol", "600519", "--count", "10", "--server", "127.0.0.1:7709"},
		{"hq-history-transactions", "--market", "sh", "--symbol", "600519", "--date", "20260605", "--count", "10", "--server", "127.0.0.1:7709"},
		{"hq-company-categories", "--market", "sh", "--symbol", "600519", "--server", "127.0.0.1:7709"},
		{"hq-company-content", "--market", "sh", "--symbol", "600519", "--filename", "600519.txt", "--start", "1", "--length", "2", "--server", "127.0.0.1:7709"},
		{"hq-xdxr", "--market", "sh", "--symbol", "600519", "--server", "127.0.0.1:7709"},
		{"hq-finance", "--market", "sh", "--symbol", "600519", "--server", "127.0.0.1:7709"},
		{"hq-block-meta", "--file", "block.dat", "--server", "127.0.0.1:7709"},
		{"hq-block", "--file", "block.dat", "--server", "127.0.0.1:7709"},
	}
	for _, args := range commands {
		var out bytes.Buffer
		var errOut bytes.Buffer
		code := Run(context.Background(), args, &out, &errOut)
		if code != 0 {
			t.Fatalf("%v exit %d stderr=%s stdout=%s", args, code, errOut.String(), out.String())
		}
		if strings.Join(seenServers, ",") != "127.0.0.1:7709" {
			t.Fatalf("%v servers = %#v", args, seenServers)
		}
		var decoded any
		if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
			t.Fatalf("%v invalid json: %v\n%s", args, err, out.String())
		}
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

func TestExQuoteCountCommandEmitsJSONAndUsesServerOverride(t *testing.T) {
	original := fetchExInstrumentCount
	defer func() { fetchExInstrumentCount = original }()

	var gotServers []string
	fetchExInstrumentCount = func(ctx context.Context, opts tdx.ExQuoteClientOptions) (int, error) {
		gotServers = append([]string(nil), opts.Servers...)
		return 12345, nil
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"exquote-count", "--server", "127.0.0.1:7727"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if strings.Join(gotServers, ",") != "127.0.0.1:7727" {
		t.Fatalf("servers = %#v", gotServers)
	}
	var decoded int
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if decoded != 12345 {
		t.Fatalf("decoded = %d", decoded)
	}
}

func TestExQuoteInstrumentsCommandEmitsJSON(t *testing.T) {
	original := fetchExInstruments
	defer func() { fetchExInstruments = original }()

	var gotStart int
	var gotCount int
	var gotServers []string
	fetchExInstruments = func(ctx context.Context, start, count int, opts tdx.ExQuoteClientOptions) ([]tdx.ExInstrument, error) {
		gotStart = start
		gotCount = count
		gotServers = append([]string(nil), opts.Servers...)
		return []tdx.ExInstrument{{Category: 3, Market: 47, Code: "IF1709", Name: "IF main"}}, nil
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"exquote-instruments", "--start", "100", "--count", "2", "--server", "127.0.0.1:7727"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if gotStart != 100 || gotCount != 2 || strings.Join(gotServers, ",") != "127.0.0.1:7727" {
		t.Fatalf("got start=%d count=%d servers=%#v", gotStart, gotCount, gotServers)
	}
	var decoded []tdx.ExInstrument
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if len(decoded) != 1 || decoded[0].Code != "IF1709" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestExQuoteBarsCommandEmitsJSON(t *testing.T) {
	original := fetchExBars
	defer func() { fetchExBars = original }()

	var gotRequest tdx.ExBarsRequest
	var gotServers []string
	fetchExBars = func(ctx context.Context, req tdx.ExBarsRequest, opts tdx.ExQuoteClientOptions) ([]tdx.ExBar, error) {
		gotRequest = req
		gotServers = append([]string(nil), opts.Servers...)
		return []tdx.ExBar{{Market: req.Market, Code: req.Code, Category: req.Category, DateTime: "2026-06-05 09:30", Open: 1, Close: 2}}, nil
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"exquote-bars", "--market", "47", "--code", "IF1709", "--category", "7", "--start", "10", "--count", "2", "--server", "127.0.0.1:7727"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if gotRequest != (tdx.ExBarsRequest{Category: 7, Market: 47, Code: "IF1709", Start: 10, Count: 2}) {
		t.Fatalf("request = %#v", gotRequest)
	}
	if strings.Join(gotServers, ",") != "127.0.0.1:7727" {
		t.Fatalf("servers = %#v", gotServers)
	}
	var decoded []tdx.ExBar
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if len(decoded) != 1 || decoded[0].Code != "IF1709" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestExQuoteHistoryCommandsParseDates(t *testing.T) {
	originalMinute := fetchExHistoryMinuteTime
	originalTransactions := fetchExHistoryTransactions
	originalBars := fetchExHistoryBarsRange
	defer func() {
		fetchExHistoryMinuteTime = originalMinute
		fetchExHistoryTransactions = originalTransactions
		fetchExHistoryBarsRange = originalBars
	}()

	var gotMinuteDate int
	fetchExHistoryMinuteTime = func(ctx context.Context, req tdx.ExQuoteRequest, date int, opts tdx.ExQuoteClientOptions) ([]tdx.ExMinutePoint, error) {
		gotMinuteDate = date
		return []tdx.ExMinutePoint{{Market: req.Market, Code: req.Code, Date: "2026-06-05"}}, nil
	}
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"exquote-history-minute", "--market", "47", "--code", "IF1709", "--date", "20260605"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("history minute exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if gotMinuteDate != 20260605 {
		t.Fatalf("minute date = %d", gotMinuteDate)
	}

	var gotTransactionDate int
	var gotTransactionStart int
	var gotTransactionCount int
	fetchExHistoryTransactions = func(ctx context.Context, req tdx.ExQuoteRequest, date, start, count int, opts tdx.ExQuoteClientOptions) ([]tdx.ExTransaction, error) {
		gotTransactionDate = date
		gotTransactionStart = start
		gotTransactionCount = count
		return []tdx.ExTransaction{{Market: req.Market, Code: req.Code, Date: "2026-06-05"}}, nil
	}
	out.Reset()
	errOut.Reset()
	code = Run(context.Background(), []string{"exquote-history-transactions", "--market", "47", "--code", "IF1709", "--date", "20260605", "--start", "10", "--count", "20"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("history transactions exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if gotTransactionDate != 20260605 || gotTransactionStart != 10 || gotTransactionCount != 20 {
		t.Fatalf("history transaction args date=%d start=%d count=%d", gotTransactionDate, gotTransactionStart, gotTransactionCount)
	}

	var gotStartDate int
	var gotEndDate int
	fetchExHistoryBarsRange = func(ctx context.Context, req tdx.ExQuoteRequest, startDate, endDate int, opts tdx.ExQuoteClientOptions) ([]tdx.ExBar, error) {
		gotStartDate = startDate
		gotEndDate = endDate
		return []tdx.ExBar{{Market: req.Market, Code: req.Code, DateTime: "2026-06-05 09:30"}}, nil
	}
	out.Reset()
	errOut.Reset()
	code = Run(context.Background(), []string{"exquote-history-bars", "--market", "74", "--code", "BABA", "--start-date", "20260601", "--end-date", "20260605"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("history bars exit %d stderr=%s stdout=%s", code, errOut.String(), out.String())
	}
	if gotStartDate != 20260601 || gotEndDate != 20260605 {
		t.Fatalf("history bars dates = %d %d", gotStartDate, gotEndDate)
	}
}

func TestQuoteCommandValidatesArguments(t *testing.T) {
	tests := [][]string{
		{"quote"},
		{"quote", "--symbol", "hk:00700"},
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

func writeZip(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, raw := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
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
