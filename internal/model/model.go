package model

import "time"

type DailyBar struct {
	Market    string
	Symbol    string
	TradeDate time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    uint64
	Amount    float64
}

type MinuteBar struct {
	Market    string
	Symbol    string
	BarTime   time.Time
	TradeDate time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    uint64
	Amount    float64
}

type QualityIssue struct {
	RunID             string
	Dataset           string
	Severity          string
	IssueType         string
	Market            string
	Symbol            string
	LogicalKey        string
	InputPath         string
	InputRecordOffset *uint64
	ObservedAt        time.Time
	Message           string
	Details           string
}

type TaskRun struct {
	RunID       string
	Dataset     string
	TaskType    string
	Status      string
	TargetTable string
	InputPath   string
	InputFormat string
	Params      string
	StartedAt   time.Time
	FinishedAt  *time.Time
	DurationMS  *uint64
	RowsWritten uint64
	RowsSkipped uint64
	Error       string
	UpdatedAt   time.Time
}

type Watermark struct {
	Dataset      string
	Asset        string
	Status       string
	MinWatermark *time.Time
	MaxWatermark *time.Time
	RowsWritten  uint64
	Message      string
	UpdatedAt    time.Time
}

// QuoteServiceRun is one durable realtime quote sweep run in the ops plane.
type QuoteServiceRun struct {
	RunID            string
	Status           string
	Markets          []string
	SymbolSource     string
	BatchSize        uint32
	PlannedSymbols   uint32
	PlannedBatches   uint32
	SucceededBatches uint32
	FailedBatches    uint32
	SkippedBatches   uint32
	RowsFetched      uint64
	StartedAt        time.Time
	FinishedAt       *time.Time
	DurationMS       *uint64
	Error            string
	UpdatedAt        time.Time
}

// QuoteServiceBatch is one durable batch progress record within a run.
type QuoteServiceBatch struct {
	RunID       string
	BatchNo     uint32
	Status      string
	SymbolCount uint32
	FirstSymbol string
	LastSymbol  string
	Attempts    uint32
	RowsFetched uint64
	StartedAt   *time.Time
	FinishedAt  *time.Time
	DurationMS  *uint64
	FailureKind string
	Error       string
	UpdatedAt   time.Time
}
