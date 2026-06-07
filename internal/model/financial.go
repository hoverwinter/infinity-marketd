package model

import "time"

type FinancialRawItem struct {
	Market     string
	Symbol     string
	ReportDate time.Time
	ItemID     uint16
	Value      float64
}

type GPMetricValue struct {
	Market     string
	Symbol     string
	MetricType uint16
	EventDate  time.Time
	Value1     float64
	Value2     float64
}

type FinancialItemDictionaryEntry struct {
	ItemID    uint16
	Name      string
	Title     string
	Category  string
	Unit      string
	ValueKind string
	Status    string
	SourceRef string
}

type GPMetricDictionaryEntry struct {
	MetricType    uint16
	Name          string
	Title         string
	Value1Meaning string
	Value2Meaning string
	Status        string
	SourceRef     string
}
