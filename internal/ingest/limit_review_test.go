package ingest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

type reviewMemoryStore struct {
	events     []model.LimitEvent
	summaries  []model.LimitDailySummary
	runs       []model.TaskRun
	watermarks []model.Watermark
	issues     []model.QualityIssue
	fail       error
}

func (s *reviewMemoryStore) InsertQualityIssues(_ context.Context, rows []model.QualityIssue) error {
	s.issues = append(s.issues, rows...)
	return nil
}
func (s *reviewMemoryStore) InsertTaskRun(_ context.Context, r model.TaskRun) error {
	s.runs = append(s.runs, r)
	return nil
}
func (s *reviewMemoryStore) InsertWatermark(_ context.Context, r model.Watermark) error {
	s.watermarks = append(s.watermarks, r)
	return nil
}
func (s *reviewMemoryStore) InsertLimitEvents(_ context.Context, r []model.LimitEvent) error {
	if s.fail != nil {
		return s.fail
	}
	s.events = append(s.events, r...)
	return nil
}
func (s *reviewMemoryStore) InsertLimitDailySummaries(_ context.Context, r []model.LimitDailySummary) error {
	if s.fail == nil {
		s.summaries = append(s.summaries, r...)
	}
	return s.fail
}
func (s *reviewMemoryStore) InsertLimitRelayEvents(_ context.Context, r []model.LimitRelayEvent) error {
	return s.fail
}
func (s *reviewMemoryStore) InsertLimitThemeDaily(_ context.Context, r []model.LimitThemeDaily) error {
	return s.fail
}
func (s *reviewMemoryStore) InsertLimitPerformanceIndexBars(_ context.Context, r []model.LimitPerformanceIndexBar) error {
	return s.fail
}
func (s *reviewMemoryStore) InsertMarketBreadthDaily(_ context.Context, r []model.MarketBreadthDaily) error {
	return s.fail
}

const reviewFixture = `{
"trade_date":"2026-09-04","prev_trade_date":"2026-09-03",
"summary":{"limit_up_count":1,"limit_down_count":1,"open_limit_count":0,"seal_success_rate":1},
"limit_up_pool":[{"code":"000001","board_count":3,"status":"sealed","pct_chg":10,"reason_type":"test","first_limit_up_time":"09:30:01","theme_tags":["AI","AI"," "]}],
"broken":[],
"limit_down":[{"code":"600001","board_count":0,"status":"limit_down","pct_chg":-10}],
"relay":{"trade_date":"2026-09-04","prev_trade_date":"2026-09-03","height_groups":[{"height":2,"stocks":[{"code":"000001","prev_board_count":2,"today_status":"连板","today_board_count":3,"today_pct_chg":0.1}]}]},
"theme_overview":[]
}`
const correctionFixture = `{"trade_date":"2016-01-04","mode":"upsert","reason":"restore original reason","audit_ref":"workbench:1","events":[{"code":"000001","event_type":"limit_up","close_status":"sealed","board_count":1,"reason_text":"bank"}]}`

func reviewFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestReviewSnapshotNormalizationAndExplicitUnits(t *testing.T) {
	path := reviewFile(t, "day.json", reviewFixture)
	loc, _ := time.LoadLocation("Asia/Shanghai")
	for _, tc := range []struct {
		unit string
		want float64
	}{{"percent", 0.1}, {"ratio", 10}} {
		bundle, skipped, issues, err := parseLimitReviewSnapshotFile(path, "run", loc, time.Now, tc.unit, "generic")
		if err != nil {
			t.Fatal(err)
		}
		if skipped != 0 || len(issues) != 0 || len(bundle.Events) != 2 || len(bundle.RelayEvents) != 2 {
			t.Fatalf("%+v %d %+v", bundle, skipped, issues)
		}
		if bundle.Events[0].Market != "sz" || len(bundle.Events[0].ThemeTags) != 1 || *bundle.Events[0].FirstLimitMinute != "09:30" {
			t.Fatalf("%+v", bundle.Events[0])
		}
		if *bundle.RelayEvents[0].TodayPctChg != tc.want || bundle.RelayEvents[1].SampleGroup != "prev_ladder" {
			t.Fatalf("%+v", bundle.RelayEvents)
		}
	}
}

func TestReviewLegacySealedStatusAndUnknownMinute(t *testing.T) {
	raw := strings.ReplaceAll(reviewFixture, "连板", "平板")
	raw = strings.ReplaceAll(raw, "09:30:01", "-")
	loc, _ := time.LoadLocation("Asia/Shanghai")
	bundle, skipped, issues, err := parseLimitReviewSnapshotFile(reviewFile(t, "day.json", raw), "run", loc, time.Now, "percent", "generic")
	if err != nil || skipped != 0 || len(issues) != 0 || len(bundle.Events) != 2 || len(bundle.RelayEvents) != 2 {
		t.Fatalf("%+v %d %+v %v", bundle, skipped, issues, err)
	}
	if bundle.Events[0].FirstLimitMinute != nil || bundle.RelayEvents[0].TodayStatus != "sealed" {
		t.Fatalf("%+v", bundle)
	}
	issue := limitReviewIssue("run", "/tmp/day.json", "invalid_snapshot", "", "", "bad", "error", time.Now)
	if issue.Market != "" || issue.LogicalKey != "day.json" {
		t.Fatalf("%+v", issue)
	}
}
func TestReviewImportLifecycleAndDryRun(t *testing.T) {
	path := reviewFile(t, "day.json", reviewFixture)
	store := &reviewMemoryStore{}
	result, err := ImportLimitReviewSnapshots(context.Background(), LimitReviewImportOptions{File: path, Store: store, DryRun: true})
	if err != nil || result.Events != 2 || len(store.runs) != 0 || len(store.events) != 0 {
		t.Fatalf("%+v %v", result, err)
	}
	result, err = ImportLimitReviewSnapshots(context.Background(), LimitReviewImportOptions{File: path, Store: store, LoadEvents: emptyLimitEvents})
	if err != nil || len(store.runs) != 1 || len(store.watermarks) != 1 || len(store.events) != 2 {
		t.Fatalf("%+v %v %+v", result, err, store)
	}
	store = &reviewMemoryStore{fail: errors.New("write failed")}
	result, err = ImportLimitReviewSnapshots(context.Background(), LimitReviewImportOptions{File: path, Store: store, LoadEvents: emptyLimitEvents})
	if err == nil || result.RowsWritten != 0 || len(store.watermarks) != 0 || len(store.runs) != 1 || store.runs[0].Status != "failed" {
		t.Fatalf("%+v %v %+v", result, err, store)
	}
}
func TestReviewMalformedSnapshotIsNotSuccessfulEmptyDay(t *testing.T) {
	for _, raw := range []string{"{}", strings.Replace(reviewFixture, `"prev_trade_date":"2026-09-03"`, `"prev_trade_date":"2026-09-05"`, 1), strings.Replace(reviewFixture, "09:30:01", "bad-minute", 1)} {
		store := &reviewMemoryStore{}
		result, err := ImportLimitReviewSnapshots(context.Background(), LimitReviewImportOptions{File: reviewFile(t, "day.json", raw), Store: store})
		if err == nil || len(store.events) != 0 || len(store.issues) == 0 || store.runs[0].Status != "failed" {
			t.Fatalf("%+v %v %+v", result, err, store)
		}
	}
}
func TestReviewConflictingRowsAbortWholeImport(t *testing.T) {
	raw := strings.Replace(reviewFixture, `"broken":[]`, `"broken":[]`, 1)
	raw = strings.Replace(raw, `"limit_down":[`, `"limit_down":[{"code":"600001","status":"limit_down","reason_type":"conflict"},`, 1)
	store := &reviewMemoryStore{}
	result, err := ImportLimitReviewSnapshots(context.Background(), LimitReviewImportOptions{File: reviewFile(t, "day.json", raw), Store: store})
	if err == nil || len(store.events) != 0 || result.RowsWritten != 0 {
		t.Fatalf("%+v %v", result, err)
	}
}
func TestReviewCorrectionAuditAndFailure(t *testing.T) {
	store := &reviewMemoryStore{}
	path := reviewFile(t, "corrections.jsonl", correctionFixture+"\n"+correctionFixture)
	result, err := ImportLimitReviewCorrections(context.Background(), LimitReviewImportOptions{File: path, Store: store, AllowFactReplacement: true})
	if err != nil || result.RowsWritten != 1 || result.RowsSkipped != 1 || len(store.events) != 1 {
		t.Fatalf("%+v %v", result, err)
	}
	if !strings.Contains(store.runs[0].Params, "restore original reason") || !strings.Contains(store.runs[0].Params, "workbench:1") {
		t.Fatal(store.runs[0].Params)
	}
	for _, raw := range []string{strings.Replace(correctionFixture, "upsert", "patch_missing_fields", 1), correctionFixture + "\n" + strings.Replace(correctionFixture, "bank", "changed", 1), strings.Replace(correctionFixture, `"reason_text":"bank"`, `"pct_chg":10`, 1)} {
		store = &reviewMemoryStore{}
		_, err = ImportLimitReviewCorrections(context.Background(), LimitReviewImportOptions{File: reviewFile(t, "bad.jsonl", raw), Store: store, AllowFactReplacement: true})
		if err == nil || len(store.events) != 0 || len(store.runs) != 1 || store.runs[0].Status != "failed" {
			t.Fatalf("%v %+v", err, store)
		}
	}
}
func TestAuxReviewJSONValidation(t *testing.T) {
	path := reviewFile(t, "breadth.json", `[{"trade_date":"2026-09-04","up_count":3000,"down_count":2000,"total_count":5000,"up_gt_3_count":30,"up_gt_5_count":20,"up_gt_7_count":10}]`)
	result, err := ImportMarketBreadthJSON(context.Background(), LimitReviewImportOptions{File: path, DryRun: true})
	if err != nil || result.RowsWritten != 1 {
		t.Fatalf("%+v %v", result, err)
	}
	bad := reviewFile(t, "bad.json", `[{"trade_date":"2026-09-04","total_count":5000,"up_count":100,"down_count":0,"up_gt_7_count":101}]`)
	if _, err := ImportMarketBreadthJSON(context.Background(), LimitReviewImportOptions{File: bad, DryRun: true}); err == nil {
		t.Fatal("invalid nested counts accepted")
	}
	missing := reviewFile(t, "missing.json", `[{"trade_date":"2026-09-04","total_count":5000}]`)
	if _, err := ImportMarketBreadthJSON(context.Background(), LimitReviewImportOptions{File: missing, DryRun: true}); err == nil {
		t.Fatal("missing directional counts accepted as zero")
	}
	idx := reviewFile(t, "idx.json", `{"bars":[{"index_code":"prev_non_st_limit_up_perf","trade_date":"2026-09-04","open":100,"high":103,"low":99,"close":102}]}`)
	result, err = ImportLimitPerformanceJSON(context.Background(), LimitReviewImportOptions{File: idx, DryRun: true})
	if err != nil || result.RowsWritten != 1 {
		t.Fatalf("%+v %v", result, err)
	}
}

func TestAuxReviewRejectsEmptyAndUnknownFields(t *testing.T) {
	for _, raw := range []string{`{"rows":null}`, `[]`, `[{"trade_date":"2026-09-04","up_count":1,"down_count":1,"total_count":2,"up_gt_7_cout":1}]`} {
		store := &reviewMemoryStore{}
		result, err := ImportMarketBreadthJSON(context.Background(), LimitReviewImportOptions{File: reviewFile(t, "bad.json", raw), Store: store})
		if err == nil || result.RowsWritten != 0 || len(store.watermarks) != 0 || len(store.runs) != 1 || store.runs[0].Status != "failed" || result.RunID == "" {
			t.Fatalf("%+v %v %+v", result, err, store)
		}
	}
}
