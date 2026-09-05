package querier

import (
	"context"
	"time"
)

const (
	Version       = "0.1.0"
	SchemaVersion = "2026-09-05"
)

type Health struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	SchemaVersion string `json:"schema_version"`
}

type BarQuery struct {
	Market string `json:"market"`
	Symbol string `json:"symbol"`
	Period string `json:"period"`
	Adjust string `json:"adjust"`
	Since  string `json:"since,omitempty"`
	Until  string `json:"until,omitempty"`
	Limit  int    `json:"limit"`
}

type Bar struct {
	Market    string     `json:"market"`
	Symbol    string     `json:"symbol"`
	Period    string     `json:"period"`
	TradeDate string     `json:"trade_date"`
	BarTime   *time.Time `json:"bar_time,omitempty"`
	Open      float64    `json:"open"`
	High      float64    `json:"high"`
	Low       float64    `json:"low"`
	Close     float64    `json:"close"`
	Volume    uint64     `json:"volume"`
	Amount    float64    `json:"amount"`
}

type BarResult struct {
	Query BarQuery `json:"query"`
	Bars  []Bar    `json:"bars"`
}

type IntradayPointQuery struct {
	Market string `json:"market"`
	Symbol string `json:"symbol"`
	Date   string `json:"date,omitempty"`
	Since  string `json:"since,omitempty"`
	Until  string `json:"until,omitempty"`
	Limit  int    `json:"limit"`
}

type IntradayPoint struct {
	Market     string    `json:"market"`
	Symbol     string    `json:"symbol"`
	TradeDate  string    `json:"trade_date"`
	PointTime  time.Time `json:"point_time"`
	PointIndex uint16    `json:"point_index"`
	Price      float64   `json:"price"`
	Volume     uint64    `json:"volume"`
}

type IntradayPointResult struct {
	Query  IntradayPointQuery `json:"query"`
	Points []IntradayPoint    `json:"points"`
}

type SymbolResolution struct {
	Symbol string `json:"symbol"`
	Market string `json:"market"`
}

type Repository interface {
	LimitReviewRepository
	ConsoleRepository
	Health(ctx context.Context) error
	Bars(ctx context.Context, query BarQuery) (BarResult, error)
	IntradayPoints(ctx context.Context, query IntradayPointQuery) (IntradayPointResult, error)
}
