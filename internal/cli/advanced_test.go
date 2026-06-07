package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

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

func TestFundCommandsRequireServer(t *testing.T) {
	if _, e, c := run("fund-kline", "--code", "159915"); c != 2 || !strings.Contains(e, "--server is required") {
		t.Fatalf("fund-kline: code=%d err=%s", c, e)
	}
	if _, e, c := run("fund-detail", "--code", "159915"); c != 2 || !strings.Contains(e, "--server is required") {
		t.Fatalf("fund-detail: code=%d err=%s", c, e)
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
