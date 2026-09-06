// Package marketdata defines online data products, independent of source protocols
// and storage. Instrument identities are meaningful only within a provider.
package marketdata

import "context"

const Timezone = "Asia/Shanghai"

type Instrument struct {
	Kind   string `json:"kind"` // index or security
	Market string `json:"market"`
	Symbol string `json:"symbol"`
}

type BarsQuery struct {
	Instrument Instrument `json:"instrument"`
	Period     string     `json:"period"`
	Adjust     string     `json:"adjust"`
	Since      string     `json:"since"`
	Until      string     `json:"until"`
}

type Bar struct {
	Time   string  `json:"time"` // YYYY-MM-DD for 1d; RFC3339 for intraday
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
	Amount float64 `json:"amount"`
}

type BarsResult struct {
	Provider   string    `json:"provider"`
	Query      BarsQuery `json:"query"`
	Timezone   string    `json:"timezone"`
	VolumeUnit string    `json:"volume_unit"`
	AmountUnit string    `json:"amount_unit"`
	Bars       []Bar     `json:"bars"`
	Warnings   []string  `json:"warnings"`
}

type Board struct {
	Kind       string      `json:"kind"`
	Code       string      `json:"code"` // page/catalog ID; not necessarily a quote symbol
	Name       string      `json:"name"`
	Instrument *Instrument `json:"instrument,omitempty"` // populated by ResolveBoard
}

type BoardsResult struct {
	Provider string  `json:"provider"`
	Kind     string  `json:"kind"`
	Scope    string  `json:"scope"`
	Boards   []Board `json:"boards"`
}

type BoardResult struct {
	Provider string `json:"provider"`
	Board    Board  `json:"board"`
}

type BarsCapability struct {
	Kind    string   `json:"kind"`
	Markets []string `json:"markets"`
	Periods []string `json:"periods"`
}

// Providers implement only the data products that they actually support.
type Provider interface{ ID() string }

type BarsProvider interface {
	Provider
	BarsCapabilities() []BarsCapability
	Bars(context.Context, BarsQuery) (BarsResult, error)
}

type BoardProvider interface {
	Provider
	BoardKinds() []string
	Boards(context.Context, string) (BoardsResult, error)
	ResolveBoard(context.Context, string, string) (BoardResult, error)
}

type ProviderInfo struct {
	ID         string           `json:"id"`
	Bars       []BarsCapability `json:"bars"`
	BoardKinds []string         `json:"board_kinds"`
}
