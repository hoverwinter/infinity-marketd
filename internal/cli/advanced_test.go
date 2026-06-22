package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/onlineadjust"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

func run(args ...string) (string, string, int) {
	var out, errOut bytes.Buffer
	code := Run(context.Background(), args, &out, &errOut)
	return out.String(), errOut.String(), code
}

func TestHQQuotesListCLI(t *testing.T) {
	orig := fetchHQQuotesList
	defer func() { fetchHQQuotesList = orig }()
	var gotSort uint16
	fetchHQQuotesList = func(ctx context.Context, req tdx.HQQuotesListRequest, opts tdx.QuoteClientOptions) ([]tdx.HQQuotesListItem, error) {
		gotSort = req.SortType
		return []tdx.HQQuotesListItem{{Market: "sh", Symbol: "600519", Price: 1272.86}}, nil
	}
	out, errOut, code := run("hq-quotes-list", "--category", "0", "--sort", "turnover", "--count", "5", "--server", "127.0.0.1:7709")
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, errOut)
	}
	if gotSort != tdx.QuotesSortTurnover {
		t.Fatalf("sort = %#x", gotSort)
	}
	if !strings.Contains(out, "600519") {
		t.Fatalf("output: %s", out)
	}
}

func TestHQQuotesListInvalidSort(t *testing.T) {
	if _, _, code := run("hq-quotes-list", "--sort", "nonsense"); code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestHQCompactQuotesCLI(t *testing.T) {
	orig := fetchHQCompactBatchQuotes
	defer func() { fetchHQCompactBatchQuotes = orig }()
	fetchHQCompactBatchQuotes = func(_ context.Context, requests []tdx.QuoteRequest, _ tdx.QuoteClientOptions) ([]tdx.HQCompactBatchQuote, error) {
		if len(requests) != 1 || requests[0].Symbol != "600519" {
			t.Fatalf("requests = %+v", requests)
		}
		return []tdx.HQCompactBatchQuote{{Market: "sh", Symbol: "600519", Price: 10}}, nil
	}
	out, errOut, code := run("hq-compact-quotes", "--symbol", "sh:600519", "--server", "x:7709")
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "600519") {
		t.Fatalf("output: %s", out)
	}
}

func TestHQCompactQuotesCLIRequiresSymbol(t *testing.T) {
	orig := fetchHQCompactBatchQuotes
	defer func() { fetchHQCompactBatchQuotes = orig }()
	fetchHQCompactBatchQuotes = func(context.Context, []tdx.QuoteRequest, tdx.QuoteClientOptions) ([]tdx.HQCompactBatchQuote, error) {
		t.Fatal("fetch should not be called")
		return nil, nil
	}
	if _, errOut, code := run("hq-compact-quotes"); code != 2 || !strings.Contains(errOut, "at least one symbol") {
		t.Fatalf("code=%d err=%s", code, errOut)
	}
}

func TestHQTickChartCLI(t *testing.T) {
	orig := fetchHQTickChart
	defer func() { fetchHQTickChart = orig }()
	fetchHQTickChart = func(_ context.Context, req tdx.HQTickChartRequest, _ tdx.QuoteClientOptions) ([]tdx.HQTickChartPoint, error) {
		if req.Symbol != "600519" || req.Count != 2 {
			t.Fatalf("req = %+v", req)
		}
		return []tdx.HQTickChartPoint{{Market: "sh", Symbol: "600519", Time: "09:30", Price: 10}}, nil
	}
	out, errOut, code := run("hq-tick-chart", "--market", "sh", "--symbol", "600519", "--count", "2", "--server", "x:7709")
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "09:30") {
		t.Fatalf("output: %s", out)
	}
}

func TestHQTopBoardCLI(t *testing.T) {
	orig := fetchHQTopBoard
	defer func() { fetchHQTopBoard = orig }()
	fetchHQTopBoard = func(ctx context.Context, category uint16, size int, opts tdx.QuoteClientOptions) ([]tdx.HQTopBoardGroup, error) {
		return []tdx.HQTopBoardGroup{{Name: "gainers", Items: []tdx.HQTopBoardItem{{Symbol: "600000"}}}}, nil
	}
	out, errOut, code := run("hq-top-board", "--category", "0", "--size", "3", "--server", "x:7709")
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "gainers") {
		t.Fatalf("output: %s", out)
	}
}

func TestSPBoardMembersRequiresServer(t *testing.T) {
	if _, errOut, code := run("sp-board-members", "--board", "880472"); code != 2 || !strings.Contains(errOut, "--server is required") {
		t.Fatalf("expected server-required error, got code=%d err=%s", code, errOut)
	}
}

func TestSPBoardMembersCLI(t *testing.T) {
	orig := fetchSPBoardMembers
	defer func() { fetchSPBoardMembers = orig }()
	fetchSPBoardMembers = func(ctx context.Context, server, board string, sortType uint16, count int, sortOrder uint16, timeout time.Duration) ([]tdx.HQBoardMember, error) {
		return []tdx.HQBoardMember{{Market: "sh", Symbol: "600519"}}, nil
	}
	out, _, code := run("sp-board-members", "--server", "x:7709", "--board", "880472", "--count", "1")
	if code != 0 || !strings.Contains(out, "600519") {
		t.Fatalf("code=%d out=%s", code, out)
	}
}

func TestSPBoardMembersBestCLI(t *testing.T) {
	origFetch := fetchSPBoardMembers
	origProbe := probeSPServers
	defer func() { fetchSPBoardMembers = origFetch; probeSPServers = origProbe }()
	probeSPServers = func(context.Context, []string, time.Duration) []tdx.ServerProbeResult {
		return []tdx.ServerProbeResult{{Server: "best:7709", Success: true}}
	}
	fetchSPBoardMembers = func(_ context.Context, server, board string, sortType uint16, count int, sortOrder uint16, timeout time.Duration) ([]tdx.HQBoardMember, error) {
		if server != "best:7709" {
			t.Fatalf("server = %q", server)
		}
		return []tdx.HQBoardMember{{Market: "sh", Symbol: "600519"}}, nil
	}
	out, _, code := run("sp-board-members", "--best", "--board", "880472", "--count", "1")
	if code != 0 || !strings.Contains(out, "600519") {
		t.Fatalf("code=%d out=%s", code, out)
	}
}

func TestSPBoardMembersBestNoReachableServer(t *testing.T) {
	origFetch := fetchSPBoardMembers
	origProbe := probeSPServers
	defer func() { fetchSPBoardMembers = origFetch; probeSPServers = origProbe }()
	probeSPServers = func(context.Context, []string, time.Duration) []tdx.ServerProbeResult {
		return []tdx.ServerProbeResult{{Server: "bad:7709", Success: false, Error: "timeout"}}
	}
	fetchSPBoardMembers = func(context.Context, string, string, uint16, int, uint16, time.Duration) ([]tdx.HQBoardMember, error) {
		t.Fatal("fetch should not be called")
		return nil, nil
	}
	if _, errOut, code := run("sp-board-members", "--best", "--board", "880472"); code != 1 || !strings.Contains(errOut, "no reachable SP server") {
		t.Fatalf("code=%d err=%s", code, errOut)
	}
}

func TestFundCommandsRequireServer(t *testing.T) {
	if _, e, c := run("fund-kline", "--code", "159915"); c != 2 || !strings.Contains(e, "--server is required") {
		t.Fatalf("fund-kline: code=%d err=%s", c, e)
	}
	if _, e, c := run("fund-detail", "--code", "159915"); c != 2 || !strings.Contains(e, "--server is required") {
		t.Fatalf("fund-detail: code=%d err=%s", c, e)
	}
}

func TestSPAndFundServerCommands(t *testing.T) {
	if out, errOut, code := run("sp-servers"); code != 0 || !strings.Contains(out, "sp") {
		t.Fatalf("sp-servers code=%d out=%s err=%s", code, out, errOut)
	}
	if out, errOut, code := run("fund-servers"); code != 0 || !strings.Contains(out, "fund") {
		t.Fatalf("fund-servers code=%d out=%s err=%s", code, out, errOut)
	}
}

func TestSPAndFundProbeCLI(t *testing.T) {
	origSP := probeSPServers
	origFund := probeFundServers
	defer func() { probeSPServers = origSP; probeFundServers = origFund }()
	probeSPServers = func(_ context.Context, servers []string, _ time.Duration) []tdx.ServerProbeResult {
		return []tdx.ServerProbeResult{{Server: "x:7709", Success: true}}
	}
	probeFundServers = func(_ context.Context, servers []string, _ time.Duration) []tdx.ServerProbeResult {
		return []tdx.ServerProbeResult{{Server: "x:7727", Success: true}}
	}
	if out, _, code := run("sp-probe", "--server", "x:7709"); code != 0 || !strings.Contains(out, "x:7709") {
		t.Fatalf("sp-probe code=%d out=%s", code, out)
	}
	if out, _, code := run("fund-probe", "--server", "x:7727"); code != 0 || !strings.Contains(out, "x:7727") {
		t.Fatalf("fund-probe code=%d out=%s", code, out)
	}
}

func TestFundDetailCLI(t *testing.T) {
	orig := fetchFundDetail
	defer func() { fetchFundDetail = orig }()
	fetchFundDetail = func(ctx context.Context, server, code string, mode uint16, timeout time.Duration) (tdx.HQFundDetail, error) {
		return tdx.HQFundDetail{Code: code, Category: 0x21}, nil
	}
	out, _, code := run("fund-detail", "--server", "x:7727", "--code", "159915")
	if code != 0 || !strings.Contains(out, "159915") {
		t.Fatalf("code=%d out=%s", code, out)
	}
}

func TestFundBestCLI(t *testing.T) {
	origProbe := probeFundServers
	origKline := fetchFundKline
	origDetail := fetchFundDetail
	defer func() { probeFundServers = origProbe; fetchFundKline = origKline; fetchFundDetail = origDetail }()
	probeFundServers = func(context.Context, []string, time.Duration) []tdx.ServerProbeResult {
		return []tdx.ServerProbeResult{{Server: "best:7727", Success: true}}
	}
	fetchFundKline = func(_ context.Context, server, code, period string, count int, timeout time.Duration) ([]tdx.HQFundBar, error) {
		if server != "best:7727" {
			t.Fatalf("server = %q", server)
		}
		return []tdx.HQFundBar{{Time: "2026-06-05"}}, nil
	}
	fetchFundDetail = func(_ context.Context, server, code string, mode uint16, timeout time.Duration) (tdx.HQFundDetail, error) {
		if server != "best:7727" {
			t.Fatalf("server = %q", server)
		}
		return tdx.HQFundDetail{Code: code}, nil
	}
	if out, _, code := run("fund-kline", "--best", "--code", "159915"); code != 0 || !strings.Contains(out, "2026-06-05") {
		t.Fatalf("fund-kline code=%d out=%s", code, out)
	}
	if out, _, code := run("fund-detail", "--best", "--code", "159915"); code != 0 || !strings.Contains(out, "159915") {
		t.Fatalf("fund-detail code=%d out=%s", code, out)
	}
}

func TestFundBestNoReachableServer(t *testing.T) {
	origProbe := probeFundServers
	origKline := fetchFundKline
	defer func() { probeFundServers = origProbe; fetchFundKline = origKline }()
	probeFundServers = func(context.Context, []string, time.Duration) []tdx.ServerProbeResult {
		return []tdx.ServerProbeResult{{Server: "bad:7727", Success: false, Error: "timeout"}}
	}
	fetchFundKline = func(context.Context, string, string, string, int, time.Duration) ([]tdx.HQFundBar, error) {
		t.Fatal("fetch should not be called")
		return nil, nil
	}
	if _, errOut, code := run("fund-kline", "--best", "--code", "159915"); code != 1 || !strings.Contains(errOut, "no reachable fund server") {
		t.Fatalf("code=%d err=%s", code, errOut)
	}
}

func TestHQLHBCLI(t *testing.T) {
	orig := fetchHQLHB
	defer func() { fetchHQLHB = orig }()
	fetchHQLHB = func(ctx context.Context, req tdx.HQMinuteRequest, aliases []string, opts tdx.QuoteClientOptions) (tdx.HQLHBResult, error) {
		return tdx.HQLHBResult{Market: req.Market, Symbol: req.Symbol, Found: true}, nil
	}
	out, errOut, code := run("hq-lhb", "--market", "sh", "--symbol", "600519", "--server", "x:7709")
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "600519") {
		t.Fatalf("output: %s", out)
	}
}

func TestHQAdjustedBarsOnlineCLI(t *testing.T) {
	orig := fetchHQAdjustedBarsOnline
	defer func() { fetchHQAdjustedBarsOnline = orig }()
	fetchHQAdjustedBarsOnline = func(_ context.Context, req onlineadjust.HQAdjustedBarsOnlineRequest, _ tdx.QuoteClientOptions) (onlineadjust.HQAdjustedBarsOnlineResult, error) {
		if req.Adjust != "qfq" || req.Symbol != "600519" {
			t.Fatalf("req = %+v", req)
		}
		return onlineadjust.HQAdjustedBarsOnlineResult{Query: req, Source: "tdx-live-provider", Bars: []onlineadjust.HQAdjustedBar{{Market: "sh", Symbol: "600519", Adjust: "qfq", Open: 9}}}, nil
	}
	out, errOut, code := run("hq-adjusted-bars-online", "--market", "sh", "--symbol", "600519", "--adjust", "qfq", "--server", "x:7709")
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "tdx-live-provider") {
		t.Fatalf("output: %s", out)
	}
}

func TestHQAdjustedBarsOnlineCLIRejectsBadAdjust(t *testing.T) {
	orig := fetchHQAdjustedBarsOnline
	defer func() { fetchHQAdjustedBarsOnline = orig }()
	fetchHQAdjustedBarsOnline = func(context.Context, onlineadjust.HQAdjustedBarsOnlineRequest, tdx.QuoteClientOptions) (onlineadjust.HQAdjustedBarsOnlineResult, error) {
		t.Fatal("fetch should not be called")
		return onlineadjust.HQAdjustedBarsOnlineResult{}, nil
	}
	if _, errOut, code := run("hq-adjusted-bars-online", "--market", "sh", "--symbol", "600519", "--adjust", "bad"); code != 2 || !strings.Contains(errOut, "adjust must be") {
		t.Fatalf("code=%d err=%s", code, errOut)
	}
}
