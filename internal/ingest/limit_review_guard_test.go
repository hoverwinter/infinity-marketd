package ingest

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

func emptyLimitEvents(context.Context, string) ([]model.LimitEvent, error) { return nil, nil }

func TestLimitEnrichmentProtectsEveryCoreField(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	day := time.Date(2016, 1, 4, 0, 0, 0, 0, loc)
	value := 12.5
	before := model.LimitEvent{TradeDate: day, Market: "sz", Symbol: "000001", EventType: "limit_up", CloseStatus: "sealed", BoardCount: 1, ThemePrimary: "未分类", Amount: &value}
	after := before
	after.ReasonText, after.ThemePrimary = "bank", "finance"
	if err := validateLimitEnrichment(before, after); err != nil {
		t.Fatal(err)
	}
	mutations := []func(*model.LimitEvent){
		func(r *model.LimitEvent) { r.BoardCount = 2 },
		func(r *model.LimitEvent) { r.Amount = nil },
		func(r *model.LimitEvent) { r.EventType = "open_limit" },
		func(r *model.LimitEvent) { r.CloseStatus = "unsealed" },
		func(r *model.LimitEvent) { r.Symbol = "000002" },
		func(r *model.LimitEvent) { r.Market = "sh" },
		func(r *model.LimitEvent) { r.TradeDate = day.AddDate(0, 0, 1) },
		func(r *model.LimitEvent) { r.ThemeTags = []string{"new"} },
		func(r *model.LimitEvent) { v := "09:31"; r.FirstLimitMinute = &v },
		func(r *model.LimitEvent) { v := uint16(0); r.OpenCount = &v },
	}
	for i, mutate := range mutations {
		changed := after
		mutate(&changed)
		if validateLimitEnrichment(before, changed) == nil {
			t.Fatalf("accepted mutation %d", i)
		}
	}
	before.ReasonText = "confirmed"
	if validateLimitEnrichment(before, after) == nil {
		t.Fatal("overwrote reason")
	}
	before.ReasonText, before.ThemePrimary = "", "confirmed"
	if validateLimitEnrichment(before, after) == nil {
		t.Fatal("overwrote theme")
	}
}

func TestLimitEnrichmentFailsClosedAndWritesVerifiedRows(t *testing.T) {
	raw := strings.Replace(correctionFixture, "upsert", "enrich_existing", 1)
	loc, _ := time.LoadLocation("Asia/Shanghai")
	before := model.LimitEvent{TradeDate: time.Date(2016, 1, 4, 0, 0, 0, 0, loc), Market: "sz", Symbol: "000001", EventType: "limit_up", CloseStatus: "sealed", BoardCount: 1}
	load := func(context.Context, string) ([]model.LimitEvent, error) { return []model.LimitEvent{before}, nil }
	for _, opts := range []LimitReviewImportOptions{{DryRun: true}, {DryRun: true, LoadEvents: emptyLimitEvents}} {
		if _, err := ImportLimitReviewCorrectionsReader(context.Background(), strings.NewReader(raw), opts); err == nil {
			t.Fatal("missing current row accepted")
		}
	}
	store := &reviewMemoryStore{}
	opts := LimitReviewImportOptions{Store: store, LoadEvents: load}
	if _, err := ImportLimitReviewCorrectionsReader(context.Background(), strings.NewReader(raw), opts); err != nil {
		t.Fatal(err)
	}
	if len(store.events) != 1 || store.events[0].ReasonText != "bank" {
		t.Fatal(store.events)
	}
	store.events = nil
	before.BoardCount = 2
	if _, err := ImportLimitReviewCorrectionsReader(context.Background(), strings.NewReader(raw), opts); err == nil || len(store.events) != 0 {
		t.Fatal("core drift accepted")
	}
	if _, err := ImportLimitReviewCorrectionsReader(context.Background(), strings.NewReader(correctionFixture), opts); err == nil {
		t.Fatal("implicit operator overwrite accepted")
	}
}

func TestSnapshotOccupiedDateIsProtected(t *testing.T) {
	store := &reviewMemoryStore{}
	path := reviewFile(t, "day.json", reviewFixture)
	load := func(context.Context, string) ([]model.LimitEvent, error) {
		return []model.LimitEvent{{Symbol: "000001"}}, nil
	}
	_, err := ImportLimitReviewSnapshots(context.Background(), LimitReviewImportOptions{File: path, Store: store, LoadEvents: load})
	if err == nil || len(store.events) != 0 || len(store.summaries) != 0 {
		t.Fatalf("%v %+v", err, store)
	}
}

func TestProviderPreservesEnrichmentAndMissingValues(t *testing.T) {
	value := 100.0
	old := model.LimitEvent{ReasonText: "verified", ThemePrimary: "theme", ThemeTags: []string{"tag"}, Amount: &value, BoardCount: 2}
	next := preserveLimitEnrichment(old, model.LimitEvent{ReasonText: "provider", BoardCount: 3})
	if next.ReasonText != "verified" || next.ThemePrimary != "theme" || len(next.ThemeTags) != 1 || next.Amount != &value || next.BoardCount != 3 {
		t.Fatal(next)
	}
}
