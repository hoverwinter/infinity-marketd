package consoleops

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/clickhouse"
	"github.com/hoverwinter/infinity-marketd/internal/config"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/querier"
)

func TestLimitCorrectionHTTPClickHouseIntegration(t *testing.T) {
	path := os.Getenv("MARKETD_REVIEW_INTEGRATION_CONFIG")
	if path == "" {
		t.Skip("set MARKETD_REVIEW_INTEGRATION_CONFIG for retained isolated databases")
	}
	cfg, err := config.Load(config.Overrides{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	prefix := fmt.Sprintf("review_http_test_%d", time.Now().UnixNano())
	cfg.ClickHouse.Databases = config.DatabaseConfig{Market: prefix + "_market", Ops: prefix + "_ops"}
	t.Logf("isolated databases retained: %s, %s", cfg.ClickHouse.Databases.Market, cfg.ClickHouse.Databases.Ops)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	store, err := clickhouse.Open(ctx, cfg.ClickHouse)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	h := querier.NewServer(store).WithLimitCorrectionImporter("integration-secret", LimitCorrectionImporter(store, cfg.Runtime.Timezone)).Handler()
	post := func(reason, query string, status int) {
		raw := fmt.Sprintf(`{"trade_date":"2016-01-04","mode":"enrich_existing","reason":"verified backfill","events":[{"code":"000001","event_type":"limit_up","close_status":"sealed","board_count":1,"reason_text":%q}]}`, reason)
		r := httptest.NewRequest("POST", "/api/console/imports/limit-review-corrections"+query, strings.NewReader(raw))
		r.Header.Set("Authorization", "Bearer integration-secret")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	}
	post("original", "", 400)
	q := querier.LimitQuery{TradeDate: "2016-01-04"}
	rows, err := store.LimitEvents(ctx, q)
	if err != nil || len(rows.Rows) != 0 {
		t.Fatalf("dry run wrote facts: %+v %v", rows, err)
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if err := store.InsertLimitEvents(ctx, []model.LimitEvent{{TradeDate: time.Date(2016, 1, 4, 0, 0, 0, 0, loc), Market: "sz", Symbol: "000001", EventType: "limit_up", CloseStatus: "sealed", BoardCount: 1}}); err != nil {
		t.Fatal(err)
	}
	post("original", "", 200)
	post("original", "?dry_run=false", 200)
	post("corrected", "?dry_run=false", 400)
	rows, err = store.LimitEvents(ctx, q)
	if err != nil || len(rows.Rows) != 1 || rows.Rows[0].ReasonText != "original" {
		t.Fatalf("%+v %v", rows, err)
	}
	runs, err := store.ConsoleTaskRuns(ctx, 10)
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs=%+v err=%v", runs, err)
	}
}
