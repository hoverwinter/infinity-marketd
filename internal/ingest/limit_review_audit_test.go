package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/querier"
)

func TestLegacySnapshotPlaceholderProfiles(t *testing.T) {
	raw := `{"trade_date":"2026-09-04","summary":{"big_noodle_count":0,"high_level_break_count":2,"strong_theme_count":0,"seal_success_rate":0},"limit_up_pool":[{"code":"000001","board_count":1,"status":"sealed","amount":100,"order_amount":0,"turnover_rate":0,"open_num":0}],"broken":[],"limit_down":[],"warnings":["missing_field:turnover_rate"]}`
	path := reviewFile(t, "day.json", raw)
	loc, _ := time.LoadLocation("Asia/Shanghai")
	for _, kind := range []string{"generic", "historical-replay", "ths"} {
		bundle, _, issues, err := parseLimitReviewSnapshotFile(path, "test", loc, time.Now, "percent", kind)
		if err != nil {
			t.Fatal(err)
		}
		row := bundle.Events[0]
		if row.Amount == nil || *row.Amount != 100 || len(issues) == 0 || issues[0].IssueType != "snapshot_warning" {
			t.Fatalf("lost value/warnings: %+v", bundle)
		}
		if kind == "generic" {
			if row.TurnoverRate == nil || row.OpenCount == nil || row.SealOrderAmount == nil {
				t.Fatal("generic zeros changed")
			}
		} else if row.TurnoverRate != nil || row.SealOrderAmount != nil {
			t.Fatal("placeholder persisted")
		}
		if kind == "historical-replay" {
			if row.OpenCount != nil || bundle.Summaries[0].BigNoodleCount != nil || bundle.Summaries[0].SealSuccessRate != nil {
				t.Fatal("historical defaults persisted")
			}
			if bundle.Summaries[0].HighLevelBreakCount == nil || *bundle.Summaries[0].HighLevelBreakCount != 2 {
				t.Fatal("nonzero correction lost")
			}
		}
	}
	if _, _, _, err := parseLimitReviewSnapshotFile(path, "test", loc, time.Now, "percent", "unknown"); err == nil {
		t.Fatal("invalid profile accepted")
	}
}

// Opt-in read-only audit: the existing Go parser is compared with HTTP FINAL reads.
func TestLimitReviewMigrationAudit(t *testing.T) {
	root, url := os.Getenv("MARKETD_REVIEW_AUDIT_ROOT"), os.Getenv("MARKETD_REVIEW_AUDIT_URL")
	if root == "" || url == "" {
		t.Skip("set MARKETD_REVIEW_AUDIT_ROOT and MARKETD_REVIEW_AUDIT_URL")
	}
	var manifest struct {
		Selected []struct {
			Path         string `json:"path"`
			TradeDate    string `json:"trade_date"`
			SHA256       string `json:"sha256"`
			SnapshotKind string `json:"snapshot_kind"`
			PercentUnit  string `json:"percent_unit"`
		} `json:"selected"`
	}
	body, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Selected) == 0 {
		t.Fatal("empty manifest")
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	expected := map[string][]string{}
	months := map[string]bool{}
	for _, file := range manifest.Selected {
		body, err := os.ReadFile(file.Path)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(body)
		if hex.EncodeToString(hash[:]) != file.SHA256 {
			t.Fatalf("frozen input changed: %s", file.Path)
		}
		bundle, skipped, issues, err := parseLimitReviewSnapshotFile(file.Path, "audit", loc, time.Now, file.PercentUnit, file.SnapshotKind)
		if err != nil || skipped != 0 {
			t.Fatalf("invalid frozen snapshot: %s: %v", file.Path, err)
		}
		for _, issue := range issues {
			if issue.Severity == "error" {
				t.Fatal(issue)
			}
		}
		if _, found := expected[file.TradeDate]; found {
			t.Fatal("duplicate manifest date")
		}
		expected[file.TradeDate] = []string{}
		for _, row := range bundle.Events {
			expected[file.TradeDate] = append(expected[file.TradeDate], auditEventJSON(t, row))
		}
		months[file.TradeDate[:7]] = true
	}
	client := querier.NewHTTPClient(url, &http.Client{Timeout: 60 * time.Second})
	keys := make([]string, 0, len(months))
	for month := range months {
		keys = append(keys, month)
	}
	sort.Strings(keys)
	type dayAudit struct {
		Date   string `json:"trade_date"`
		Rows   int    `json:"rows"`
		SHA256 string `json:"sha256"`
	}
	var report struct {
		Days       []dayAudit `json:"days"`
		Events     int        `json:"events"`
		VerifiedAt time.Time  `json:"verified_at"`
	}
	for _, month := range keys {
		start, _ := time.Parse("2006-01-02", month+"-01")
		q := querier.LimitQuery{Since: start.Format("2006-01-02"), Until: start.AddDate(0, 1, -1).Format("2006-01-02")}
		rows, err := querier.ReadCompleteLimitRows(context.Background(), q, client.LimitEvents)
		if err != nil {
			t.Fatal(err)
		}
		actual := map[string][]string{}
		for _, row := range rows {
			if _, exists := expected[row.TradeDate]; !exists {
				t.Fatalf("unexpected persisted date %s", row.TradeDate)
			}
			actual[row.TradeDate] = append(actual[row.TradeDate], auditEventJSON(t, row))
		}
		for date, want := range expected {
			if !strings.HasPrefix(date, month) {
				continue
			}
			got := actual[date]
			if got == nil {
				got = []string{}
			}
			sort.Strings(want)
			sort.Strings(got)
			if !reflect.DeepEqual(want, got) {
				for i := 0; i < len(want) && i < len(got); i++ {
					if want[i] != got[i] {
						t.Logf("first mismatch expected=%s actual=%s", want[i], got[i])
						break
					}
				}
				t.Fatalf("event mismatch %s: expected=%d actual=%d", date, len(want), len(got))
			}
			digest := sha256.Sum256([]byte(strings.Join(got, "\n")))
			report.Days = append(report.Days, dayAudit{date, len(got), hex.EncodeToString(digest[:])})
			report.Events += len(got)
		}
		t.Logf("verified %s: %d event rows", month, len(rows))
	}
	sort.Slice(report.Days, func(i, j int) bool { return report.Days[i].Date < report.Days[j].Date })
	report.VerifiedAt = time.Now()
	body, err = json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "event-audit.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("verified %d days, %d complete event rows", len(report.Days), report.Events)
}

func auditEventJSON(t *testing.T, row any) string {
	t.Helper()
	body, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	date := fmt.Sprint(fields["trade_date"])
	if len(date) < 10 {
		t.Fatal("invalid event date")
	}
	fields["trade_date"] = date[:10]
	if fields["theme_tags"] == nil {
		fields["theme_tags"] = []any{}
	}
	body, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
