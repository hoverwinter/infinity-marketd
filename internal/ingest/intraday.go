package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	chstore "github.com/hoverwinter/infinity-marketd/internal/clickhouse"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

const intradayDataset = "a_share_intraday_points"

type FetchHQMinuteTimeFunc func(context.Context, tdx.HQMinuteRequest, tdx.QuoteClientOptions) ([]tdx.HQMinutePoint, error)
type FetchHQHistoryMinuteTimeFunc func(context.Context, tdx.HQMinuteRequest, int, tdx.QuoteClientOptions) ([]tdx.HQMinutePoint, error)

type IntradayImportOptions struct {
	Market             string
	Symbol             string
	Date               string
	Since              string
	Until              string
	Today              bool
	DryRun             bool
	Store              *chstore.Store
	Timezone           string
	ClientOptions      tdx.QuoteClientOptions
	FetchMinuteTime    FetchHQMinuteTimeFunc
	FetchHistoryMinute FetchHQHistoryMinuteTimeFunc
	Now                func() time.Time
}

type IntradaySummary struct {
	RunID        string
	Dataset      string
	TargetTable  string
	Market       string
	Symbol       string
	Date         string
	Since        string
	Until        string
	Today        bool
	RowsWritten  uint64
	RowsSkipped  uint64
	DatesFetched uint64
	EmptyDates   uint64
	Issues       []model.QualityIssue
	DryRun       bool
}

func ImportIntradayPoints(ctx context.Context, opts IntradayImportOptions) (IntradaySummary, error) {
	if opts.Timezone == "" {
		opts.Timezone = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(opts.Timezone)
	if err != nil {
		return IntradaySummary{}, err
	}
	if opts.FetchMinuteTime == nil {
		opts.FetchMinuteTime = tdx.FetchHQMinuteTime
	}
	if opts.FetchHistoryMinute == nil {
		opts.FetchHistoryMinute = tdx.FetchHQHistoryMinuteTime
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	req, err := tdx.ParseHQMinuteRequest(opts.Market, opts.Symbol)
	if err != nil {
		return IntradaySummary{}, err
	}
	mode, dates, err := intradayImportDates(opts, loc)
	if err != nil {
		return IntradaySummary{}, err
	}
	if !opts.DryRun && opts.Store == nil {
		return IntradaySummary{}, fmt.Errorf("store is required when dry-run is false")
	}

	summary := IntradaySummary{
		Dataset:     intradayDataset,
		TargetTable: intradayDataset,
		Market:      req.Market,
		Symbol:      req.Symbol,
		Date:        opts.Date,
		Since:       opts.Since,
		Until:       opts.Until,
		Today:       opts.Today,
		DryRun:      opts.DryRun,
	}

	// Product-owned counters tracked while producing rows.
	var datesFetched, emptyDates uint64
	produce := func(ctx context.Context, runID string) ([]model.IntradayPoint, uint64, []model.QualityIssue, error) {
		var allPoints []model.IntradayPoint
		var issues []model.QualityIssue
		var rowsSkipped uint64
		switch mode {
		case "today":
			tradeDate := dateOnly(opts.Now().In(loc), loc)
			points, err := opts.FetchMinuteTime(ctx, req, opts.ClientOptions)
			if err != nil {
				return nil, 0, nil, err
			}
			normalized, skipped, pointIssues := normalizeIntradayPoints(runID, req, tradeDate, points, loc)
			allPoints = append(allPoints, normalized...)
			rowsSkipped += skipped
			issues = append(issues, pointIssues...)
			datesFetched = 1
			if len(points) == 0 {
				emptyDates = 1
			}
		case "history":
			for _, tradeDate := range dates {
				points, err := opts.FetchHistoryMinute(ctx, req, intDate(tradeDate), opts.ClientOptions)
				if err != nil {
					return nil, 0, nil, err
				}
				datesFetched++
				if len(points) == 0 {
					emptyDates++
					continue
				}
				normalized, skipped, pointIssues := normalizeIntradayPoints(runID, req, tradeDate, points, loc)
				allPoints = append(allPoints, normalized...)
				rowsSkipped += skipped
				issues = append(issues, pointIssues...)
			}
		default:
			return nil, 0, nil, fmt.Errorf("unsupported intraday import mode %q", mode)
		}
		return allPoints, rowsSkipped, issues, nil
	}

	params, _ := json.Marshal(map[string]any{
		"market": req.Market,
		"symbol": req.Symbol,
		"date":   opts.Date,
		"since":  opts.Since,
		"until":  opts.Until,
		"today":  opts.Today,
	})

	var ops OnlineOps
	if opts.Store != nil {
		ops = opts.Store
	}

	result, err := RunOnlineJob(ctx, OnlineJob[model.IntradayPoint]{
		Dataset:     intradayDataset,
		TargetTable: intradayDataset,
		TaskType:    "tdx_intraday_import",
		InputFormat: "tdx.hq.minute_time",
		Asset:       fmt.Sprintf("%s:%s", req.Market, req.Symbol),
		Params:      string(params),
		DryRun:      opts.DryRun,
		Ops:         ops,
		Now:         opts.Now,
		Produce:     produce,
		Write: func(ctx context.Context, rows []model.IntradayPoint) error {
			return opts.Store.InsertIntradayPoints(ctx, rows)
		},
		Bounds: intradayWatermarks,
	})
	summary.RunID = result.RunID
	summary.RowsWritten = result.RowsWritten
	summary.RowsSkipped = result.RowsSkipped
	summary.Issues = result.Issues
	summary.DatesFetched = datesFetched
	summary.EmptyDates = emptyDates
	if err != nil {
		return summary, err
	}
	return summary, nil
}

func intradayImportDates(opts IntradayImportOptions, loc *time.Location) (string, []time.Time, error) {
	hasDate := strings.TrimSpace(opts.Date) != ""
	hasSince := strings.TrimSpace(opts.Since) != ""
	hasUntil := strings.TrimSpace(opts.Until) != ""
	if opts.Today {
		if hasDate || hasSince || hasUntil {
			return "", nil, fmt.Errorf("--today cannot be combined with --date, --since, or --until")
		}
		return "today", nil, nil
	}
	if hasDate {
		if hasSince || hasUntil {
			return "", nil, fmt.Errorf("--date cannot be combined with --since or --until")
		}
		d, err := parseIntradayDate(opts.Date, loc)
		if err != nil {
			return "", nil, fmt.Errorf("parse --date: %w", err)
		}
		return "history", []time.Time{d}, nil
	}
	if hasSince || hasUntil {
		if !hasSince || !hasUntil {
			return "", nil, fmt.Errorf("--since and --until must be provided together")
		}
		since, err := parseIntradayDate(opts.Since, loc)
		if err != nil {
			return "", nil, fmt.Errorf("parse --since: %w", err)
		}
		until, err := parseIntradayDate(opts.Until, loc)
		if err != nil {
			return "", nil, fmt.Errorf("parse --until: %w", err)
		}
		if since.After(until) {
			return "", nil, fmt.Errorf("--since must be <= --until")
		}
		var dates []time.Time
		for d := since; !d.After(until); d = d.AddDate(0, 0, 1) {
			dates = append(dates, d)
		}
		return "history", dates, nil
	}
	return "", nil, fmt.Errorf("one of --date, --since/--until, or --today is required")
}

func normalizeIntradayPoints(runID string, req tdx.HQMinuteRequest, tradeDate time.Time, points []tdx.HQMinutePoint, loc *time.Location) ([]model.IntradayPoint, uint64, []model.QualityIssue) {
	seen := map[string]model.IntradayPoint{}
	out := make([]model.IntradayPoint, 0, len(points))
	var skipped uint64
	var issues []model.QualityIssue
	for i, point := range points {
		pointTime, err := parsePointTime(tradeDate, point.Time, loc)
		if err != nil {
			skipped++
			issues = append(issues, intradayIssue(runID, "invalid_point_time", req.Market, req.Symbol, logicalPointKey(req.Market, req.Symbol, tradeDate, point.Time), err.Error()))
			continue
		}
		if point.Volume < 0 {
			skipped++
			issues = append(issues, intradayIssue(runID, "invalid_volume", req.Market, req.Symbol, logicalPointKey(req.Market, req.Symbol, tradeDate, point.Time), "negative intraday volume"))
			continue
		}
		pointIndex := point.Index
		if pointIndex < 0 {
			pointIndex = i
		}
		if pointIndex > 65535 {
			skipped++
			issues = append(issues, intradayIssue(runID, "invalid_point_index", req.Market, req.Symbol, logicalPointKey(req.Market, req.Symbol, tradeDate, point.Time), "intraday point index exceeds UInt16"))
			continue
		}
		normalized := model.IntradayPoint{
			Market:     req.Market,
			Symbol:     req.Symbol,
			TradeDate:  tradeDate,
			PointTime:  pointTime,
			PointIndex: uint16(pointIndex),
			Price:      point.Price,
			Volume:     uint64(point.Volume),
		}
		key := logicalPointKey(req.Market, req.Symbol, tradeDate, pointTime.Format("15:04"))
		if prev, ok := seen[key]; ok {
			skipped++
			if sameIntradayPoint(prev, normalized) {
				continue
			}
			issues = append(issues, intradayIssue(runID, "conflicting_logical_key", req.Market, req.Symbol, key, "duplicate intraday point has conflicting values"))
			continue
		}
		seen[key] = normalized
		out = append(out, normalized)
	}
	return out, skipped, issues
}

func parseIntradayDate(value string, loc *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02", "20060102"} {
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			return dateOnly(t, loc), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date %q, expected YYYY-MM-DD or YYYYMMDD", value)
}

func parsePointTime(tradeDate time.Time, value string, loc *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"15:04", "15:04:05"} {
		t, err := time.ParseInLocation(layout, value, loc)
		if err == nil {
			return time.Date(tradeDate.Year(), tradeDate.Month(), tradeDate.Day(), t.Hour(), t.Minute(), t.Second(), 0, loc), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid point time %q", value)
}

func sameIntradayPoint(a, b model.IntradayPoint) bool {
	return a.Market == b.Market &&
		a.Symbol == b.Symbol &&
		a.TradeDate.Equal(b.TradeDate) &&
		a.PointTime.Equal(b.PointTime) &&
		a.PointIndex == b.PointIndex &&
		a.Price == b.Price &&
		a.Volume == b.Volume
}

func intradayWatermarks(points []model.IntradayPoint) (*time.Time, *time.Time) {
	if len(points) == 0 {
		return nil, nil
	}
	min := points[0].PointTime
	max := points[0].PointTime
	for _, point := range points[1:] {
		if point.PointTime.Before(min) {
			min = point.PointTime
		}
		if point.PointTime.After(max) {
			max = point.PointTime
		}
	}
	return &min, &max
}

func intradayIssue(runID string, issueType string, market string, symbol string, logicalKey string, message string) model.QualityIssue {
	return model.QualityIssue{
		RunID:      runID,
		Dataset:    intradayDataset,
		Severity:   severityFor(issueType),
		IssueType:  issueType,
		Market:     market,
		Symbol:     symbol,
		LogicalKey: logicalKey,
		ObservedAt: time.Now(),
		Message:    message,
	}
}

func logicalPointKey(market, symbol string, tradeDate time.Time, timeText string) string {
	return fmt.Sprintf("%s:%s:%s:%s", market, symbol, tradeDate.Format("2006-01-02"), timeText)
}

func dateOnly(t time.Time, loc *time.Location) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

func intDate(t time.Time) int {
	return t.Year()*10000 + int(t.Month())*100 + t.Day()
}
