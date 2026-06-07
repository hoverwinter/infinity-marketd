package querier

import (
	"context"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

const (
	DefaultConsoleLimit = 20
	MaxConsoleLimit     = 200
)

type ConsoleSummary struct {
	Health                 Health                    `json:"health"`
	Watermarks             []ConsoleWatermark        `json:"watermarks"`
	TaskRuns               []ConsoleTaskRun          `json:"task_runs"`
	DataQualityIssueCounts []ConsoleQualityIssueStat `json:"data_quality_issue_counts"`
	QuoteServiceRuns       []ConsoleQuoteServiceRun  `json:"quote_service_runs"`
}

type ConsoleWatermark struct {
	Dataset      string     `json:"dataset"`
	Asset        string     `json:"asset"`
	Status       string     `json:"status"`
	MinWatermark *time.Time `json:"min_watermark,omitempty"`
	MaxWatermark *time.Time `json:"max_watermark,omitempty"`
	RowsWritten  uint64     `json:"rows_written"`
	Message      string     `json:"message"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ConsoleTaskRun struct {
	RunID       string     `json:"run_id"`
	Dataset     string     `json:"dataset"`
	TaskType    string     `json:"task_type"`
	Status      string     `json:"status"`
	TargetTable string     `json:"target_table"`
	InputPath   string     `json:"input_path"`
	InputFormat string     `json:"input_format"`
	Params      string     `json:"params"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	DurationMS  *uint64    `json:"duration_ms,omitempty"`
	RowsWritten uint64     `json:"rows_written"`
	RowsSkipped uint64     `json:"rows_skipped"`
	Error       string     `json:"error,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type ConsoleDataQualityIssue struct {
	IssueID           string    `json:"issue_id"`
	RunID             string    `json:"run_id"`
	Dataset           string    `json:"dataset"`
	Severity          string    `json:"severity"`
	IssueType         string    `json:"issue_type"`
	Market            string    `json:"market,omitempty"`
	Symbol            string    `json:"symbol,omitempty"`
	LogicalKey        string    `json:"logical_key"`
	InputPath         string    `json:"input_path"`
	InputRecordOffset *uint64   `json:"input_record_offset,omitempty"`
	ObservedAt        time.Time `json:"observed_at"`
	Message           string    `json:"message"`
	Details           string    `json:"details,omitempty"`
}

type ConsoleQualityIssueStat struct {
	Dataset   string `json:"dataset"`
	Severity  string `json:"severity"`
	IssueType string `json:"issue_type"`
	Count     uint64 `json:"count"`
}

type ConsoleQuoteServiceRun struct {
	RunID            string     `json:"run_id"`
	Status           string     `json:"status"`
	Markets          []string   `json:"markets"`
	SymbolSource     string     `json:"symbol_source"`
	BatchSize        uint32     `json:"batch_size"`
	PlannedSymbols   uint32     `json:"planned_symbols"`
	PlannedBatches   uint32     `json:"planned_batches"`
	SucceededBatches uint32     `json:"succeeded_batches"`
	FailedBatches    uint32     `json:"failed_batches"`
	SkippedBatches   uint32     `json:"skipped_batches"`
	RowsFetched      uint64     `json:"rows_fetched"`
	StartedAt        time.Time  `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	DurationMS       *uint64    `json:"duration_ms,omitempty"`
	Error            string     `json:"error,omitempty"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type ConsoleBestIPStatus struct {
	CachePath   string                  `json:"cache_path"`
	GeneratedAt *time.Time              `json:"generated_at,omitempty"`
	ExpiresAt   *time.Time              `json:"expires_at,omitempty"`
	Preferred   string                  `json:"preferred,omitempty"`
	Usable      bool                    `json:"usable"`
	Results     []tdx.ServerProbeResult `json:"results"`
	Error       string                  `json:"error,omitempty"`
}

type ConsoleProbeResult struct {
	Results []tdx.ServerProbeResult `json:"results"`
}

type ConsoleQuoteSmokeResult struct {
	Quotes []tdx.Quote `json:"quotes"`
}

type ConsoleRepository interface {
	Health(ctx context.Context) error
	ConsoleWatermarks(ctx context.Context, limit int) ([]ConsoleWatermark, error)
	ConsoleTaskRuns(ctx context.Context, limit int) ([]ConsoleTaskRun, error)
	ConsoleDataQualityIssues(ctx context.Context, limit int) ([]ConsoleDataQualityIssue, error)
	ConsoleDataQualityIssueStats(ctx context.Context, limit int) ([]ConsoleQualityIssueStat, error)
	ConsoleQuoteServiceRuns(ctx context.Context, limit int) ([]ConsoleQuoteServiceRun, error)
}
