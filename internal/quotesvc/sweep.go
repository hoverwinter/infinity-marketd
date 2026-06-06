package quotesvc

import (
	"context"
	"fmt"
	"sort"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

// SymbolSource records how a sweep's symbols were obtained, so a resume can
// reject an incompatible symbol source.
type SymbolSource string

const (
	SourceExplicit  SymbolSource = "explicit"
	SourceDiscovery SymbolSource = "discovery"
)

const defaultBatchSize = 80

// Batch is one unit of quote work with a stable, recorded batch number.
type Batch struct {
	No       int
	Requests []tdx.QuoteRequest
}

func (b Batch) FirstSymbol() string {
	if len(b.Requests) == 0 {
		return ""
	}
	return b.Requests[0].Symbol
}

func (b Batch) LastSymbol() string {
	if len(b.Requests) == 0 {
		return ""
	}
	return b.Requests[len(b.Requests)-1].Symbol
}

// Discoverer loads tradable symbols for a market (online security list).
type Discoverer func(ctx context.Context, market string) ([]tdx.QuoteRequest, error)

// PlanOptions describes a requested sweep.
type PlanOptions struct {
	Markets   []string           // discovery markets when Requests is empty
	Requests  []tdx.QuoteRequest // explicit symbols; if set, discovery is skipped
	BatchSize int                // 0 -> defaultBatchSize
	Limit     int                // 0 -> no limit
}

// SweepPlan is the resolved, partitioned plan for a sweep run.
type SweepPlan struct {
	Markets      []string
	SymbolSource SymbolSource
	BatchSize    int
	Requests     []tdx.QuoteRequest
	Batches      []Batch
}

// PlanSweep resolves symbols (explicit or discovered) and partitions them into
// stable batches.
func PlanSweep(ctx context.Context, opts PlanOptions, discover Discoverer) (SweepPlan, error) {
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}

	var requests []tdx.QuoteRequest
	var source SymbolSource
	var markets []string

	if len(opts.Requests) > 0 {
		source = SourceExplicit
		requests = append([]tdx.QuoteRequest(nil), opts.Requests...)
		markets = distinctMarkets(requests)
	} else {
		source = SourceDiscovery
		markets = opts.Markets
		if len(markets) == 0 {
			markets = []string{"sh", "sz"}
		}
		if discover == nil {
			return SweepPlan{}, fmt.Errorf("symbol discovery required but no discoverer configured")
		}
		for _, market := range markets {
			found, err := discover(ctx, market)
			if err != nil {
				return SweepPlan{}, fmt.Errorf("discover %s symbols: %w", market, err)
			}
			requests = append(requests, found...)
		}
	}

	if opts.Limit > 0 && len(requests) > opts.Limit {
		requests = requests[:opts.Limit]
	}
	if len(requests) == 0 {
		return SweepPlan{}, fmt.Errorf("sweep has no symbols")
	}

	return SweepPlan{
		Markets:      markets,
		SymbolSource: source,
		BatchSize:    batchSize,
		Requests:     requests,
		Batches:      PlanBatches(requests, batchSize),
	}, nil
}

// PlanBatches partitions requests into stable batches numbered from 1.
func PlanBatches(requests []tdx.QuoteRequest, batchSize int) []Batch {
	parts := tdx.SplitQuoteRequests(requests, batchSize)
	batches := make([]Batch, 0, len(parts))
	for i, part := range parts {
		batches = append(batches, Batch{No: i + 1, Requests: part})
	}
	return batches
}

func distinctMarkets(requests []tdx.QuoteRequest) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, r := range requests {
		if _, ok := seen[r.Market]; ok {
			continue
		}
		seen[r.Market] = struct{}{}
		out = append(out, r.Market)
	}
	sort.Strings(out)
	return out
}
