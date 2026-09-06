package infinitycli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/hoverwinter/infinity-marketd/internal/marketdata"
	"github.com/hoverwinter/infinity-marketd/internal/querier"
)

func runProviderCommand(ctx context.Context, command string, args []string, stdout, stderr io.Writer) int {
	var baseURL, provider, kind, code string
	var q marketdata.BarsQuery
	fs := newFlagSet("querier "+command, stderr)
	registerServiceFlags(fs, &baseURL)
	if command != "providers" {
		fs.StringVar(&provider, "provider", "", "explicit source ID: tdx, ths, or eastmoney")
	}
	switch command {
	case "provider-bars":
		fs.StringVar(&q.Instrument.Kind, "kind", "index", "instrument kind: index or security")
		fs.StringVar(&q.Instrument.Market, "market", "", "source market: board for THS/Eastmoney; sh/sz/bj for TDX")
		fs.StringVar(&q.Instrument.Symbol, "symbol", "", "source quotation symbol, not a board page code")
		fs.StringVar(&q.Period, "period", "1d", "bar period (see providers for supported periods)")
		fs.StringVar(&q.Adjust, "adjust", "none", "adjustment mode: none")
		fs.StringVar(&q.Since, "since", "", "inclusive start date YYYY-MM-DD")
		fs.StringVar(&q.Until, "until", "", "inclusive end date YYYY-MM-DD")
	case "provider-boards", "provider-board":
		fs.StringVar(&kind, "kind", "", "board kind: industry or concept")
		if command == "provider-board" {
			fs.StringVar(&code, "code", "", "board page/catalog code to resolve")
		}
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected positional arguments")
		return 2
	}
	if command != "providers" && strings.TrimSpace(provider) == "" {
		fmt.Fprintln(stderr, "--provider is required")
		return 2
	}
	client := querier.NewHTTPClient(baseURL, nil)
	var result any
	var err error
	switch command {
	case "providers":
		result, err = client.Providers(ctx)
	case "provider-bars":
		result, err = client.ProviderBars(ctx, provider, q)
	case "provider-boards":
		result, err = client.ProviderBoards(ctx, provider, kind)
	case "provider-board":
		result, err = client.ProviderBoard(ctx, provider, kind, code)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	writeJSON(stdout, result)
	return 0
}
