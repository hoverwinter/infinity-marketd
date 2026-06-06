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
