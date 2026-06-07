package clickhouse

import (
	"strings"
	"testing"

	"github.com/hoverwinter/infinity-marketd/internal/querier"
)

func TestSafeConsoleLimit(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{name: "default", input: 0, want: querier.DefaultConsoleLimit},
		{name: "negative", input: -1, want: querier.DefaultConsoleLimit},
		{name: "explicit", input: 50, want: 50},
		{name: "max", input: querier.MaxConsoleLimit + 1, want: querier.MaxConsoleLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := safeConsoleLimit(tt.input); got != tt.want {
				t.Fatalf("limit=%d want=%d", got, tt.want)
			}
		})
	}
}

func TestConsoleQuerySQLUsesRecentOrderingAndLimit(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want []string
	}{
		{
			name: "watermarks",
			sql:  consoleWatermarksSQL("ops.watermarks", 7),
			want: []string{"FROM ops.watermarks FINAL", "ORDER BY updated_at DESC LIMIT 7"},
		},
		{
			name: "task runs",
			sql:  consoleTaskRunsSQL("ops.task_runs", 8),
			want: []string{"FROM ops.task_runs FINAL", "ORDER BY started_at DESC LIMIT 8"},
		},
		{
			name: "data quality issues",
			sql:  consoleDataQualityIssuesSQL("ops.data_quality_issues", 9),
			want: []string{"FROM ops.data_quality_issues", "ORDER BY observed_at DESC LIMIT 9"},
		},
		{
			name: "quote service runs",
			sql:  consoleQuoteServiceRunsSQL("ops.quote_service_runs", 10),
			want: []string{"FROM ops.quote_service_runs FINAL", "ORDER BY started_at DESC LIMIT 10"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, want := range tc.want {
				if !strings.Contains(tc.sql, want) {
					t.Fatalf("sql=%s missing %s", tc.sql, want)
				}
			}
		})
	}
}

func TestConsoleQualityStatsSQLGroupsAndLimits(t *testing.T) {
	stmt := consoleDataQualityIssueStatsSQL("ops.data_quality_issues", 5)
	for _, want := range []string{
		"GROUP BY dataset, severity, issue_type",
		"ORDER BY issue_count DESC LIMIT 5",
	} {
		if !strings.Contains(stmt, want) {
			t.Fatalf("sql=%s missing %s", stmt, want)
		}
	}
}
