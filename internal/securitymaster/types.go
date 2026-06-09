package securitymaster

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	SourceTDX  = "tdx"
	SourceFile = "file"

	StatusUnknown   = "unknown"
	StatusListed    = "listed"
	StatusSuspended = "suspended"
	StatusDelisted  = "delisted"

	RefreshStatusRunning   = "running"
	RefreshStatusSucceeded = "succeeded"
	RefreshStatusFailed    = "failed"
	RefreshStatusDryRun    = "dry_run"
)

var ErrNotFound = errors.New("security not found")

type UnavailableError struct {
	Err error
}

func (e UnavailableError) Error() string {
	if e.Err == nil {
		return "securities master unavailable"
	}
	return "securities master unavailable: " + e.Err.Error()
}

func (e UnavailableError) Unwrap() error {
	return e.Err
}

type Security struct {
	Market          string    `json:"market"`
	Symbol          string    `json:"symbol"`
	Exchange        string    `json:"exchange"`
	CurrentName     string    `json:"current_name"`
	CurrentNameNorm string    `json:"current_name_norm,omitempty"`
	Board           string    `json:"board"`
	Status          string    `json:"status"`
	ListingDate     string    `json:"listing_date,omitempty"`
	DelistingDate   string    `json:"delisting_date,omitempty"`
	LotSize         int       `json:"lot_size,omitempty"`
	PricePrecision  int       `json:"price_precision,omitempty"`
	Source          string    `json:"source"`
	ManualLocked    bool      `json:"manual_locked"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
}

type NameHistory struct {
	ID             int64     `json:"id,omitempty"`
	Market         string    `json:"market"`
	Symbol         string    `json:"symbol"`
	Name           string    `json:"name"`
	NameNorm       string    `json:"name_norm,omitempty"`
	ValidFrom      string    `json:"valid_from"`
	ValidTo        string    `json:"valid_to,omitempty"`
	Source         string    `json:"source"`
	ManualOverride bool      `json:"manual_override"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

type Alias struct {
	ID        int64     `json:"id,omitempty"`
	Market    string    `json:"market"`
	Symbol    string    `json:"symbol"`
	Alias     string    `json:"alias"`
	AliasNorm string    `json:"alias_norm,omitempty"`
	AliasType string    `json:"alias_type"`
	Priority  int       `json:"priority"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type RefreshRun struct {
	ID              int64      `json:"id,omitempty"`
	Source          string     `json:"source"`
	Markets         []string   `json:"markets"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	Status          string     `json:"status"`
	RowsSeen        int        `json:"rows_seen"`
	RowsUpserted    int        `json:"rows_upserted"`
	RowsSkipped     int        `json:"rows_skipped"`
	AliasesUpserted int        `json:"aliases_upserted"`
	HistoryUpserted int        `json:"history_upserted"`
	Error           string     `json:"error,omitempty"`
}

type ResolveCandidate struct {
	Security    Security `json:"security"`
	MatchType   string   `json:"match_type"`
	MatchedText string   `json:"matched_text"`
	Score       int      `json:"score"`
}

type Reader interface {
	Security(ctx context.Context, market string, symbol string) (Security, error)
	Resolve(ctx context.Context, query string, limit int) ([]ResolveCandidate, error)
}

type Writer interface {
	BeginRefreshRun(ctx context.Context, run RefreshRun) (int64, error)
	FinishRefreshRun(ctx context.Context, id int64, run RefreshRun) error
	UpsertSecurity(ctx context.Context, security Security) error
	UpsertAliases(ctx context.Context, aliases []Alias) (int, error)
	UpsertNameHistory(ctx context.Context, history []NameHistory) (int, error)
}

type Repository interface {
	Reader
	Writer
}

type unavailableReader struct {
	err error
}

func NewUnavailableReader(err error) Reader {
	if err == nil {
		err = fmt.Errorf("mysql is not configured")
	}
	return unavailableReader{err: err}
}

func (r unavailableReader) Security(context.Context, string, string) (Security, error) {
	return Security{}, UnavailableError{Err: r.err}
}

func (r unavailableReader) Resolve(context.Context, string, int) ([]ResolveCandidate, error) {
	return nil, UnavailableError{Err: r.err}
}

func marketsCSV(markets []string) string {
	return strings.Join(markets, ",")
}
