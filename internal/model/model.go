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

type Symbol struct {
	Market string
	Symbol string
}

type IntradayPoint struct {
	Market     string
	Symbol     string
	TradeDate  time.Time
	PointTime  time.Time
	PointIndex uint16
	Price      float64
	Volume     uint64
}

type XDXREvent struct {
	Market         string
	Symbol         string
	EventDate      time.Time
	Category       uint8
	CategoryName   string
	FenHong        *float64
	PeiGuJia       *float64
	SongZhuanGu    *float64
	PeiGu          *float64
	SuoGu          *float64
	PanQianLiuTong *float64
	PanHouLiuTong  *float64
	QianZongGuBen  *float64
	HouZongGuBen   *float64
	FenShu         *float64
	XingQuanJia    *float64
}

type AdjustFactor struct {
	Market     string
	Symbol     string
	TradeDate  time.Time
	QFQFactor  *float64
	HFQFactor  *float64
	ComputedAt time.Time
}

type DailyDerived struct {
	Market     string
	Symbol     string
	TradeDate  time.Time
	PrevClose  *float64
	PctChg     *float64
	ComputedAt time.Time
}

type CapitalChangeEvent struct {
	Market           string
	Symbol           string
	EventDate        time.Time
	Category         uint8
	EventSeq         uint16
	EventName        string
	CashDividend     *float64
	AllotmentPrice   *float64
	BonusShares      *float64
	AllotmentShares  *float64
	ShrinkShares     *float64
	PreFloatShares   *float64
	PostFloatShares  *float64
	PreTotalShares   *float64
	PostTotalShares  *float64
	RatioDenominator *float64
	ExercisePrice    *float64
}

type TDXBlockSnapshot struct {
	SnapshotID   string
	BlockScope   string
	SnapshotTime time.Time
	ContentHash  string
	BlockCount   uint32
	MemberCount  uint32
}

type TDXBlockDefinition struct {
	SnapshotID   string
	BlockScope   string
	BlockKind    string
	BlockID      string
	BlockName    string
	BlockType    uint16
	DisplayOrder uint32
	MemberCount  uint32
}

type TDXBlockMembership struct {
	SnapshotID  string
	BlockScope  string
	BlockID     string
	MemberOrder uint32
	Code        string
	Market      string
	Symbol      string
}

type ExDailyBar struct {
	ExMarket        uint16
	Code            string
	TradeDate       time.Time
	Open            float64
	High            float64
	Low             float64
	Close           float64
	Position        int64
	Trade           int64
	Price           *float64
	Amount          *float64
	SettlementPrice *float64
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
