package quotesvc

import (
	"context"
	"testing"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

func reqs(market string, symbols ...string) []tdx.QuoteRequest {
	out := make([]tdx.QuoteRequest, 0, len(symbols))
	for _, s := range symbols {
		out = append(out, tdx.QuoteRequest{Market: market, Symbol: s})
	}
	return out
}

func TestPlanBatchesStableNumbering(t *testing.T) {
	batches := PlanBatches(reqs("sh", "600000", "600001", "600002", "600003", "600004"), 2)
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches, got %d", len(batches))
	}
	for i, b := range batches {
		if b.No != i+1 {
			t.Fatalf("batch %d has number %d", i, b.No)
		}
	}
	if batches[2].FirstSymbol() != "600004" || batches[2].LastSymbol() != "600004" {
		t.Fatalf("unexpected last batch symbols: %+v", batches[2])
	}
}

func TestPlanSweepExplicit(t *testing.T) {
	plan, err := PlanSweep(context.Background(), PlanOptions{
		Requests:  reqs("sh", "600000", "600001"),
		BatchSize: 1,
	}, nil)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.SymbolSource != SourceExplicit {
		t.Fatalf("expected explicit source, got %s", plan.SymbolSource)
	}
	if len(plan.Batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(plan.Batches))
	}
	if len(plan.Markets) != 1 || plan.Markets[0] != "sh" {
		t.Fatalf("unexpected markets %v", plan.Markets)
	}
}

func TestPlanSweepDiscoveryWithLimit(t *testing.T) {
	discover := func(ctx context.Context, market string) ([]tdx.QuoteRequest, error) {
		switch market {
		case "sh":
			return reqs("sh", "600000", "600001"), nil
		case "sz":
			return reqs("sz", "000001", "000002"), nil
		}
		return nil, nil
	}
	plan, err := PlanSweep(context.Background(), PlanOptions{
		Markets:   []string{"sh", "sz"},
		BatchSize: 10,
		Limit:     3,
	}, discover)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.SymbolSource != SourceDiscovery {
		t.Fatalf("expected discovery source")
	}
	if len(plan.Requests) != 3 {
		t.Fatalf("expected limit of 3 symbols, got %d", len(plan.Requests))
	}
}

func TestPlanSweepEmptyErrors(t *testing.T) {
	discover := func(ctx context.Context, market string) ([]tdx.QuoteRequest, error) { return nil, nil }
	if _, err := PlanSweep(context.Background(), PlanOptions{Markets: []string{"sh"}}, discover); err == nil {
		t.Fatalf("expected error for empty sweep")
	}
}
