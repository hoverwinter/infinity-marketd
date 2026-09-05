package consoleops

import (
	"context"
	"strings"
	"testing"

	"github.com/hoverwinter/infinity-marketd/internal/ingest"
	"github.com/hoverwinter/infinity-marketd/internal/querier"
)

type correctionReadStore struct{ ingest.LimitReviewWriter }

func (s correctionReadStore) LimitEvents(context.Context, querier.LimitQuery) (querier.LimitResult[querier.LimitEvent], error) {
	return querier.LimitResult[querier.LimitEvent]{Rows: []querier.LimitEvent{{TradeDate: "2016-01-04", Market: "sz", Symbol: "000001", EventType: "limit_up", CloseStatus: "sealed", BoardCount: 1}}}, nil
}

func TestLimitCorrectionSharedValidation(t *testing.T) {
	importer := LimitCorrectionImporter(correctionReadStore{}, "Asia/Shanghai")
	valid := `{"trade_date":"2016-01-04","mode":"enrich_existing","reason":"verified","events":[{"code":"000001","event_type":"limit_up","close_status":"sealed","board_count":1,"reason_text":"bank"}]}`
	result, err := importer(context.Background(), []byte(valid), true)
	if err != nil || !result.DryRun || result.Events != 1 {
		t.Fatalf("%+v %v", result, err)
	}
	for _, payload := range []string{`{}`, strings.Replace(valid, `"mode":"enrich_existing"`, `"mode":"upsert"`, 1), strings.Replace(valid, `"board_count":1`, `"board_count":1,"pct_chg":10`, 1), strings.Replace(valid, `"board_count":1`, `"board_count":2`, 1)} {
		if _, err := importer(context.Background(), []byte(payload), false); !querier.IsValidationError(err) {
			t.Fatalf("expected validation failure: %v", err)
		}
	}
	if _, err := LimitCorrectionImporter(nil, "Asia/Shanghai")(context.Background(), []byte(valid), true); !querier.IsValidationError(err) {
		t.Fatalf("missing reader must fail validation: %v", err)
	}
}
