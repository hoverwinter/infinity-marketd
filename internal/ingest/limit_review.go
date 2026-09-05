package ingest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

const (
	limitReviewDataset = "a_share_limit_review"
	limitReviewTarget  = "a_share_limit_events"
)

type LimitReviewWriter interface {
	OnlineOps
	InsertLimitEvents(context.Context, []model.LimitEvent) error
	InsertLimitDailySummaries(context.Context, []model.LimitDailySummary) error
	InsertLimitRelayEvents(context.Context, []model.LimitRelayEvent) error
	InsertLimitThemeDaily(context.Context, []model.LimitThemeDaily) error
	InsertLimitPerformanceIndexBars(context.Context, []model.LimitPerformanceIndexBar) error
	InsertMarketBreadthDaily(context.Context, []model.MarketBreadthDaily) error
}

type LimitReviewImportOptions struct {
	LoadEvents           func(context.Context, string) ([]model.LimitEvent, error)
	AllowFactReplacement bool
	PercentUnit          string
	SnapshotKind         string
	File                 string
	Root                 string
	Since                string
	Until                string
	DryRun               bool
	Store                LimitReviewWriter
	Timezone             string
	Now                  func() time.Time
}

type LimitReviewImportSummary struct {
	RunID          string               `json:"run_id"`
	Dataset        string               `json:"dataset"`
	TargetTable    string               `json:"target_table"`
	FilesRead      uint64               `json:"files_read"`
	Events         uint64               `json:"events"`
	DailySummaries uint64               `json:"daily_summaries"`
	RelayEvents    uint64               `json:"relay_events"`
	Themes         uint64               `json:"themes"`
	RowsWritten    uint64               `json:"rows_written"`
	RowsSkipped    uint64               `json:"rows_skipped"`
	Issues         []model.QualityIssue `json:"issues"`
	DryRun         bool                 `json:"dry_run"`
}

type limitReviewBundle struct {
	Events      []model.LimitEvent
	Summaries   []model.LimitDailySummary
	RelayEvents []model.LimitRelayEvent
	Themes      []model.LimitThemeDaily
}

type rawLimitReviewSnapshot struct {
	Warnings      []string        `json:"warnings"`
	TradeDate     string          `json:"trade_date"`
	PrevTradeDate string          `json:"prev_trade_date"`
	Summary       rawLimitSummary `json:"summary"`
	LimitUpPool   []rawLimitStock `json:"limit_up_pool"`
	Broken        []rawLimitStock `json:"broken"`
	LimitDown     []rawLimitStock `json:"limit_down"`
	Relay         rawLimitRelay   `json:"relay"`
	ThemeOverview []rawLimitTheme `json:"theme_overview"`
}

type rawLimitSummary struct {
	LimitUpCount             uint32   `json:"limit_up_count"`
	LimitDownCount           uint32   `json:"limit_down_count"`
	OpenLimitCount           uint32   `json:"open_limit_count"`
	SealSuccessRate          *float64 `json:"seal_success_rate"`
	MaxBoardHeight           uint16   `json:"max_board_height"`
	FirstBoardCount          uint32   `json:"first_board_count"`
	ContinuousBoardCount     uint32   `json:"continuous_board_count"`
	PrevLimitUpPromotionRate *float64 `json:"prev_limit_up_promotion_rate"`
	PrevLadderPromotionRate  *float64 `json:"prev_ladder_promotion_rate"`
	BigNoodleCount           *uint32  `json:"big_noodle_count"`
	HighLevelBreakCount      *uint32  `json:"high_level_break_count"`
	StrongThemeCount         *uint32  `json:"strong_theme_count"`
}

type rawLimitStock struct {
	Code             string   `json:"code"`
	BoardCount       uint16   `json:"board_count"`
	ReasonType       string   `json:"reason_type"`
	ThemePrimary     string   `json:"theme_primary"`
	ThemeTags        []string `json:"theme_tags"`
	OrderAmount      *float64 `json:"order_amount"`
	Amount           *float64 `json:"amount"`
	PctChg           *float64 `json:"pct_chg"`
	FirstLimitUpTime *string  `json:"first_limit_up_time"`
	LastLimitUpTime  *string  `json:"last_limit_up_time"`
	TurnoverRate     *float64 `json:"turnover_rate"`
	OpenNum          *uint16  `json:"open_num"`
	Status           string   `json:"status"`
	MarketValue      *float64 `json:"market_value"`
}

type rawLimitRelay struct {
	TradeDate     string                `json:"trade_date"`
	PrevTradeDate string                `json:"prev_trade_date"`
	HeightGroups  []rawRelayHeightGroup `json:"height_groups"`
}

type rawRelayHeightGroup struct {
	Height uint16          `json:"height"`
	Stocks []rawRelayStock `json:"stocks"`
}

type rawRelayStock struct {
	Code                 string   `json:"code"`
	PrevBoardCount       uint16   `json:"prev_board_count"`
	PrevFirstLimitUpTime *string  `json:"prev_first_limit_up_time"`
	PrevThemePrimary     string   `json:"prev_theme_primary"`
	PrevReason           string   `json:"prev_reason"`
	TodayStatus          string   `json:"today_status"`
	TodayBoardCount      uint16   `json:"today_board_count"`
	TodayPctChg          *float64 `json:"today_pct_chg"`
}

type rawLimitTheme struct {
	ThemeName        string `json:"theme_name"`
	LimitUpCount     uint32 `json:"limit_up_count"`
	LadderCount      uint32 `json:"ladder_count"`
	BrokenCount      uint32 `json:"broken_count"`
	LimitDownCount   uint32 `json:"limit_down_count"`
	LeaderCode       string `json:"leader_code"`
	LeaderBoardCount uint16 `json:"leader_board_count"`
	StrengthRank     uint16 `json:"strength_rank"`
}

func ImportLimitReviewSnapshots(ctx context.Context, opts LimitReviewImportOptions) (LimitReviewImportSummary, error) {
	loc, now, since, until, err := normalizeLimitReviewImportOptions(opts)
	if err != nil {
		return LimitReviewImportSummary{}, err
	}
	paths, err := discoverLimitReviewFiles(opts.File, opts.Root)
	if err != nil {
		return LimitReviewImportSummary{}, err
	}
	if len(paths) == 0 {
		return LimitReviewImportSummary{}, fmt.Errorf("no limit review JSON files found")
	}
	if !opts.DryRun && opts.Store == nil {
		return LimitReviewImportSummary{}, fmt.Errorf("store is required when dry-run is false")
	}

	runID := newRunID()
	started := now()
	summary := LimitReviewImportSummary{RunID: runID, Dataset: limitReviewDataset, TargetTable: limitReviewTarget, DryRun: opts.DryRun}
	bundle := limitReviewBundle{}
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		var pathDate time.Time
		if opts.Root != "" {
			rel, relErr := filepath.Rel(expandHome(opts.Root), path)
			if relErr != nil {
				return summary, relErr
			}
			dateText := strings.TrimSuffix(filepath.ToSlash(rel), ".json")
			date, dateErr := parseReviewDate(strings.ReplaceAll(dateText, "/", "-"), loc)
			if dateErr != nil {
				continue
			}
			if (since != nil && date.Before(*since)) || (until != nil && date.After(*until)) {
				continue
			}
			pathDate = date
		}
		parsed, skipped, issues, parseErr := parseLimitReviewSnapshotFile(path, runID, loc, now, opts.PercentUnit, opts.SnapshotKind)
		if parseErr == nil && !pathDate.IsZero() && !pathDate.Equal(parsed.Summaries[0].TradeDate) {
			parseErr = fmt.Errorf("trade_date does not match YYYY/MM/DD.json path")
		}
		if parseErr != nil {
			summary.RowsSkipped++
			summary.Issues = append(summary.Issues, limitReviewIssue(runID, path, "invalid_snapshot", "", "", parseErr.Error(), "error", now))
			continue
		}
		if (since != nil && parsed.Summaries[0].TradeDate.Before(*since)) || (until != nil && parsed.Summaries[0].TradeDate.After(*until)) {
			continue
		}
		summary.FilesRead++
		summary.RowsSkipped += skipped
		summary.Issues = append(summary.Issues, issues...)
		bundle.Events = append(bundle.Events, parsed.Events...)
		bundle.Summaries = append(bundle.Summaries, parsed.Summaries...)
		bundle.RelayEvents = append(bundle.RelayEvents, parsed.RelayEvents...)
		bundle.Themes = append(bundle.Themes, parsed.Themes...)
	}

	bundle, skipped, issues := dedupeLimitReviewBundle(bundle, runID, now)
	summary.RowsSkipped += skipped
	summary.Issues = append(summary.Issues, issues...)
	summary.Events = uint64(len(bundle.Events))
	summary.DailySummaries = uint64(len(bundle.Summaries))
	summary.RelayEvents = uint64(len(bundle.RelayEvents))
	summary.Themes = uint64(len(bundle.Themes))
	summary.RowsWritten = summary.Events + summary.DailySummaries + summary.RelayEvents + summary.Themes
	var validationErr error
	for _, issue := range summary.Issues {
		if issue.Severity == "error" {
			validationErr = fmt.Errorf("import validation failed: %s (%s)", issue.IssueType, issue.InputPath)
			break
		}
	}
	if summary.FilesRead == 0 {
		validationErr = fmt.Errorf("no valid snapshots within requested date range")
	}
	if validationErr == nil && !opts.AllowFactReplacement && !opts.DryRun {
		if opts.LoadEvents == nil {
			validationErr = fmt.Errorf("current-event reader required before snapshot writes")
		} else {
			for _, day := range bundle.Summaries {
				rows, err := opts.LoadEvents(ctx, day.TradeDate.Format("2006-01-02"))
				if err != nil {
					validationErr = err
					break
				}
				if len(rows) != 0 {
					validationErr = fmt.Errorf("snapshot date %s already contains events; explicit operator replacement required", day.TradeDate.Format("2006-01-02"))
					break
				}
			}
		}
	}
	if opts.DryRun {
		return summary, validationErr
	}

	writeErr := validationErr
	summary.RowsWritten = 0
	if writeErr == nil {
		summary.RowsWritten, writeErr = writeLimitReviewBundle(ctx, opts.Store, bundle)
	}
	if len(summary.Issues) > 0 {
		if issueErr := opts.Store.InsertQualityIssues(ctx, summary.Issues); writeErr == nil {
			writeErr = issueErr
		}
	}
	status := "success"
	if len(summary.Issues) > 0 {
		status = "degraded"
	}
	if writeErr != nil {
		status = "failed"
	}
	if writeErr == nil && len(bundle.Summaries) > 0 {
		minWM, maxWM := summaryWatermarks(bundle.Summaries)
		writeErr = opts.Store.InsertWatermark(ctx, model.Watermark{Dataset: limitReviewDataset, Asset: "all", Status: status, MinWatermark: minWM, MaxWatermark: maxWM, RowsWritten: summary.RowsWritten, Message: status, UpdatedAt: now()})
	}
	if writeErr != nil {
		status = "failed"
	}
	params, _ := json.Marshal(map[string]any{"file": opts.File, "root": opts.Root, "since": opts.Since, "until": opts.Until, "files_read": summary.FilesRead, "percent_unit": opts.PercentUnit, "snapshot_kind": opts.SnapshotKind, "target_tables": []string{"a_share_limit_events", "a_share_limit_daily_summary", "a_share_limit_relay_events", "a_share_limit_theme_daily"}})
	if runErr := recordLimitReviewTaskRun(ctx, opts.Store, runID, "limit_review_json_import", limitReviewTarget, firstNonEmpty(opts.File, opts.Root), "quman.limit_review.snapshot.v1", string(params), started, now, summary.RowsWritten, summary.RowsSkipped, status, writeErr); writeErr == nil {
		writeErr = runErr
	}
	if writeErr != nil {
		return summary, writeErr
	}
	return summary, nil
}

func parseLimitReviewSnapshotFile(path, runID string, loc *time.Location, now func() time.Time, percentUnit, snapshotKind string) (limitReviewBundle, uint64, []model.QualityIssue, error) {
	if snapshotKind != "" && snapshotKind != "generic" && snapshotKind != "historical-replay" && snapshotKind != "ths" {
		return limitReviewBundle{}, 0, nil, fmt.Errorf("unsupported snapshot kind %q", snapshotKind)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return limitReviewBundle{}, 0, nil, err
	}
	var snapshot rawLimitReviewSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return limitReviewBundle{}, 0, nil, err
	}
	normalizeSnapshotPlaceholders(&snapshot, snapshotKind)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return limitReviewBundle{}, 0, nil, err
	}
	for _, key := range []string{"summary", "limit_up_pool", "broken", "limit_down"} {
		if value, ok := fields[key]; !ok || string(value) == "null" {
			return limitReviewBundle{}, 0, nil, fmt.Errorf("missing required snapshot field %s", key)
		}
	}
	tradeDate, err := parseReviewDate(snapshot.TradeDate, loc)
	if err != nil {
		return limitReviewBundle{}, 0, nil, fmt.Errorf("trade_date: %w", err)
	}
	prevTradeDate, err := parseOptionalReviewDate(snapshot.PrevTradeDate, loc)
	if err != nil {
		return limitReviewBundle{}, 0, nil, fmt.Errorf("prev_trade_date: %w", err)
	}
	if prevTradeDate != nil && !prevTradeDate.Before(tradeDate) {
		return limitReviewBundle{}, 0, nil, fmt.Errorf("prev_trade_date must precede trade_date")
	}
	for _, rate := range []*float64{snapshot.Summary.SealSuccessRate, snapshot.Summary.PrevLimitUpPromotionRate, snapshot.Summary.PrevLadderPromotionRate} {
		if rate != nil && (!finiteNumbers(*rate) || *rate < 0 || *rate > 1) {
			return limitReviewBundle{}, 0, nil, fmt.Errorf("summary rate must be in [0,1]")
		}
	}

	bundle := limitReviewBundle{}
	var skipped uint64
	var issues []model.QualityIssue
	if len(snapshot.Warnings) > 0 {
		issues = append(issues, limitReviewIssue(runID, path, "snapshot_warning", snapshot.TradeDate, "", strings.Join(snapshot.Warnings, "; "), "warning", now))
	}
	pools := []struct {
		eventType string
		rows      []rawLimitStock
	}{
		{eventType: "limit_up", rows: snapshot.LimitUpPool},
		{eventType: "open_limit", rows: snapshot.Broken},
		{eventType: "limit_down", rows: snapshot.LimitDown},
	}
	for _, pool := range pools {
		for _, rawRow := range pool.rows {
			row, err := normalizeLimitStock(tradeDate, pool.eventType, rawRow)
			if err != nil {
				skipped++
				issues = append(issues, limitReviewIssue(runID, path, "invalid_event", snapshot.TradeDate, rawRow.Code, err.Error(), "error", now))
				continue
			}
			bundle.Events = append(bundle.Events, row)
		}
	}

	boardCounts := countBoardHeights(bundle.Events)
	bundle.Summaries = append(bundle.Summaries, model.LimitDailySummary{
		TradeDate: tradeDate, PrevTradeDate: prevTradeDate,
		LimitUpCount: snapshot.Summary.LimitUpCount, LimitDownCount: snapshot.Summary.LimitDownCount, OpenLimitCount: snapshot.Summary.OpenLimitCount,
		SealSuccessRate: snapshot.Summary.SealSuccessRate, MaxBoardHeight: snapshot.Summary.MaxBoardHeight,
		FirstBoardCount: snapshot.Summary.FirstBoardCount, ContinuousBoardCount: snapshot.Summary.ContinuousBoardCount,
		PrevLimitUpPromotionRate: snapshot.Summary.PrevLimitUpPromotionRate, PrevLadderPromotionRate: snapshot.Summary.PrevLadderPromotionRate,
		BigNoodleCount: snapshot.Summary.BigNoodleCount, HighLevelBreakCount: snapshot.Summary.HighLevelBreakCount, StrongThemeCount: snapshot.Summary.StrongThemeCount,
		TwoBoardCount: boardCounts[2], ThreeBoardCount: boardCounts[3], FourBoardCount: boardCounts[4], FivePlusBoardCount: boardCounts[5],
	})
	issues = append(issues, snapshotCountIssues(runID, path, snapshot, bundle.Events, now)...)

	percentScale := 1.0
	if percentUnit == "ratio" {
		percentScale = 100
	}
	relayTradeDate := tradeDate
	if snapshot.Relay.TradeDate != "" {
		if snapshot.Relay.TradeDate != snapshot.TradeDate {
			return limitReviewBundle{}, 0, nil, fmt.Errorf("relay trade_date mismatch")
		}
	}
	relayPrevDate := prevTradeDate
	if snapshot.Relay.PrevTradeDate != "" {
		relayPrevDate, err = parseOptionalReviewDate(snapshot.Relay.PrevTradeDate, loc)
		if err != nil {
			return limitReviewBundle{}, 0, nil, fmt.Errorf("relay.prev_trade_date: %w", err)
		}
		if prevTradeDate != nil && !relayPrevDate.Equal(*prevTradeDate) {
			return limitReviewBundle{}, 0, nil, fmt.Errorf("relay prev_trade_date mismatch")
		}
	}
	if relayPrevDate == nil && len(snapshot.Relay.HeightGroups) > 0 {
		return limitReviewBundle{}, 0, nil, fmt.Errorf("relay prev_trade_date is required")
	}
	if relayPrevDate != nil {
		if !relayPrevDate.Before(tradeDate) {
			return limitReviewBundle{}, 0, nil, fmt.Errorf("relay prev_trade_date must precede trade_date")
		}
		for _, group := range snapshot.Relay.HeightGroups {
			for _, rawRow := range group.Stocks {
				rows, normalizeErr := normalizeRelayRows(relayTradeDate, *relayPrevDate, rawRow, percentScale)
				if normalizeErr != nil {
					skipped++
					issues = append(issues, limitReviewIssue(runID, path, "invalid_relay_event", snapshot.TradeDate, rawRow.Code, normalizeErr.Error(), "error", now))
					continue
				}
				bundle.RelayEvents = append(bundle.RelayEvents, rows...)
			}
		}
	}

	for _, rawTheme := range snapshot.ThemeOverview {
		name := strings.TrimSpace(rawTheme.ThemeName)
		if name == "" {
			skipped++
			issues = append(issues, limitReviewIssue(runID, path, "invalid_theme", snapshot.TradeDate, "", "theme_name is required", "error", now))
			continue
		}
		leaderMarket := ""
		leaderSymbol := strings.TrimSpace(rawTheme.LeaderCode)
		if validReviewSymbol(leaderSymbol) {
			leaderMarket = tdx.InferMarketFromCode(leaderSymbol)
		} else {
			leaderSymbol = ""
		}
		bundle.Themes = append(bundle.Themes, model.LimitThemeDaily{TradeDate: tradeDate, ThemeName: name, LimitUpCount: rawTheme.LimitUpCount, LadderCount: rawTheme.LadderCount, BrokenCount: rawTheme.BrokenCount, LimitDownCount: rawTheme.LimitDownCount, LeaderMarket: leaderMarket, LeaderSymbol: leaderSymbol, LeaderBoardCount: rawTheme.LeaderBoardCount, StrengthRank: rawTheme.StrengthRank})
	}
	return bundle, skipped, issues, nil
}

// These profiles describe verified legacy writers, not a numeric-unit heuristic.
func normalizeSnapshotPlaceholders(snapshot *rawLimitReviewSnapshot, kind string) {
	zeroFloat := func(value **float64) {
		if *value != nil && **value == 0 {
			*value = nil
		}
	}
	if kind == "historical-replay" {
		for _, pool := range [][]rawLimitStock{snapshot.LimitUpPool, snapshot.Broken, snapshot.LimitDown} {
			for i := range pool {
				zeroFloat(&pool[i].OrderAmount)
				zeroFloat(&pool[i].TurnoverRate)
				if pool[i].OpenNum != nil && *pool[i].OpenNum == 0 {
					pool[i].OpenNum = nil
				}
			}
		}
		for _, value := range []**float64{&snapshot.Summary.SealSuccessRate, &snapshot.Summary.PrevLimitUpPromotionRate, &snapshot.Summary.PrevLadderPromotionRate} {
			zeroFloat(value)
		}
		for _, value := range []**uint32{&snapshot.Summary.BigNoodleCount, &snapshot.Summary.HighLevelBreakCount, &snapshot.Summary.StrongThemeCount} {
			if *value != nil && **value == 0 {
				*value = nil
			}
		}
	}
	if kind == "ths" {
		missingTurnover := containsString(snapshot.Warnings, "missing_field:turnover_rate")
		for _, pool := range [][]rawLimitStock{snapshot.LimitUpPool, snapshot.Broken, snapshot.LimitDown} {
			for i := range pool {
				zeroFloat(&pool[i].Amount)
				zeroFloat(&pool[i].OrderAmount)
				if missingTurnover || pool[i].Status == "broken" || pool[i].Status == "limit_down" {
					zeroFloat(&pool[i].TurnoverRate)
				}
				if pool[i].Status == "limit_down" && pool[i].OpenNum != nil && *pool[i].OpenNum == 0 {
					pool[i].OpenNum = nil
				}
			}
		}
	}
}

func normalizeLimitStock(tradeDate time.Time, eventType string, raw rawLimitStock) (model.LimitEvent, error) {
	if !containsString([]string{"limit_up", "open_limit", "limit_down"}, eventType) {
		return model.LimitEvent{}, fmt.Errorf("unsupported event_type %q", eventType)
	}
	symbol := strings.TrimSpace(raw.Code)
	if !validReviewSymbol(symbol) {
		return model.LimitEvent{}, fmt.Errorf("invalid six-digit symbol %q", raw.Code)
	}
	status := strings.TrimSpace(raw.Status)
	if status == "" {
		switch eventType {
		case "limit_up":
			status = "sealed"
		case "open_limit":
			status = "broken"
		case "limit_down":
			status = "limit_down"
		}
	}
	if !containsString([]string{"sealed", "broken", "broken_reseal", "limit_down"}, status) {
		return model.LimitEvent{}, fmt.Errorf("unsupported close status %q", status)
	}
	if eventType == "limit_up" && (raw.BoardCount == 0 || (status != "sealed" && status != "broken_reseal")) {
		return model.LimitEvent{}, fmt.Errorf("limit_up requires positive board_count and sealed/resealed status")
	}
	if eventType == "limit_down" && status != "limit_down" {
		return model.LimitEvent{}, fmt.Errorf("limit_down requires limit_down status")
	}
	if eventType == "open_limit" && status != "broken" && status != "broken_reseal" && status != "limit_down" {
		return model.LimitEvent{}, fmt.Errorf("open_limit requires broken, broken_reseal, or limit_down status")
	}
	first, err := normalizeReviewMinute(raw.FirstLimitUpTime)
	if err != nil {
		return model.LimitEvent{}, fmt.Errorf("first_limit_up_time: %w", err)
	}
	last, err := normalizeReviewMinute(raw.LastLimitUpTime)
	if err != nil {
		return model.LimitEvent{}, fmt.Errorf("last_limit_up_time: %w", err)
	}
	if first != nil && last != nil && *first > *last {
		return model.LimitEvent{}, fmt.Errorf("first limit time exceeds last limit time")
	}
	if err := validateOptionalNonNegative("order_amount", raw.OrderAmount); err != nil {
		return model.LimitEvent{}, err
	}
	if err := validateOptionalNonNegative("amount", raw.Amount); err != nil {
		return model.LimitEvent{}, err
	}
	if err := validateOptionalNonNegative("turnover_rate", raw.TurnoverRate); err != nil {
		return model.LimitEvent{}, err
	}
	if err := validateOptionalNonNegative("market_value", raw.MarketValue); err != nil {
		return model.LimitEvent{}, err
	}
	return model.LimitEvent{
		TradeDate: tradeDate, Market: tdx.InferMarketFromCode(symbol), Symbol: symbol,
		EventType: eventType, CloseStatus: status, BoardCount: raw.BoardCount,
		ReasonText: strings.TrimSpace(raw.ReasonType), ThemePrimary: strings.TrimSpace(raw.ThemePrimary), ThemeTags: normalizeTags(raw.ThemeTags),
		FirstLimitMinute: first, LastLimitMinute: last, OpenCount: raw.OpenNum, SealOrderAmount: raw.OrderAmount,
		Amount: raw.Amount, TurnoverRate: raw.TurnoverRate, MarketValue: raw.MarketValue,
	}, nil
}

func normalizeRelayRows(tradeDate, prevTradeDate time.Time, raw rawRelayStock, percentScale float64) ([]model.LimitRelayEvent, error) {
	symbol := strings.TrimSpace(raw.Code)
	if !validReviewSymbol(symbol) {
		return nil, fmt.Errorf("invalid six-digit symbol %q", raw.Code)
	}
	minute, err := normalizeReviewMinute(raw.PrevFirstLimitUpTime)
	if err != nil {
		return nil, err
	}
	status, err := normalizeRelayStatus(raw.TodayStatus)
	if err != nil {
		return nil, err
	}
	pct := raw.TodayPctChg
	if pct != nil {
		value := *pct * percentScale
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1000 {
			return nil, fmt.Errorf("invalid today_pct_chg")
		}
		pct = &value
	}
	base := model.LimitRelayEvent{TradeDate: tradeDate, PrevTradeDate: prevTradeDate, Market: tdx.InferMarketFromCode(symbol), Symbol: symbol, PrevBoardCount: raw.PrevBoardCount, PrevReasonText: strings.TrimSpace(raw.PrevReason), PrevThemePrimary: strings.TrimSpace(raw.PrevThemePrimary), PrevFirstLimitMinute: minute, TodayStatus: status, TodayBoardCount: raw.TodayBoardCount, TodayPctChg: pct}
	base.SampleGroup = "prev_limit_up"
	rows := []model.LimitRelayEvent{base}
	if raw.PrevBoardCount >= 2 {
		ladder := base
		ladder.SampleGroup = "prev_ladder"
		rows = append(rows, ladder)
	}
	return rows, nil
}

func normalizeRelayStatus(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "连板", "promoted":
		return "promoted", nil
	case "平板", "sealed":
		return "sealed", nil
	case "断板", "broken":
		return "broken", nil
	case "炸板", "open_limit":
		return "open_limit", nil
	case "跌停", "limit_down":
		return "limit_down", nil
	case "停牌", "suspended":
		return "suspended", nil
	case "", "未知", "unknown":
		return "unknown", nil
	default:
		return "", fmt.Errorf("unsupported today_status %q", value)
	}
}

func countBoardHeights(events []model.LimitEvent) map[uint16]uint32 {
	counts := map[uint16]uint32{}
	for _, row := range events {
		if row.EventType != "limit_up" {
			continue
		}
		if row.BoardCount >= 5 {
			counts[5]++
		} else {
			counts[row.BoardCount]++
		}
	}
	return counts
}

func snapshotCountIssues(runID, path string, snapshot rawLimitReviewSnapshot, events []model.LimitEvent, now func() time.Time) []model.QualityIssue {
	actual := map[string]uint32{}
	for _, row := range events {
		actual[row.EventType]++
	}
	expected := map[string]uint32{"limit_up": snapshot.Summary.LimitUpCount, "open_limit": snapshot.Summary.OpenLimitCount, "limit_down": snapshot.Summary.LimitDownCount}
	var issues []model.QualityIssue
	for eventType, count := range expected {
		if actual[eventType] != count {
			message := fmt.Sprintf("summary %s count=%d but normalized events=%d", eventType, count, actual[eventType])
			issues = append(issues, limitReviewIssue(runID, path, "summary_event_count_mismatch", snapshot.TradeDate, "", message, "warning", now))
		}
	}
	return issues
}

func dedupeLimitReviewBundle(bundle limitReviewBundle, runID string, now func() time.Time) (limitReviewBundle, uint64, []model.QualityIssue) {
	var skipped uint64
	var issues []model.QualityIssue
	events := map[string]model.LimitEvent{}
	for _, row := range bundle.Events {
		key := limitEventKey(row)
		if prev, ok := events[key]; ok {
			skipped++
			if !reflect.DeepEqual(prev, row) {
				issues = append(issues, limitReviewIssue(runID, "", "conflicting_logical_key", row.TradeDate.Format("2006-01-02"), row.Symbol, key, "error", now))
			}
			continue
		}
		events[key] = row
	}
	bundle.Events = bundle.Events[:0]
	for _, row := range events {
		bundle.Events = append(bundle.Events, row)
	}
	sort.Slice(bundle.Events, func(i, j int) bool { return limitEventKey(bundle.Events[i]) < limitEventKey(bundle.Events[j]) })

	onConflict := func(key string) {
		issues = append(issues, limitReviewIssue(runID, "", "conflicting_logical_key", key, "", "conflicting review rows", "error", now))
	}
	var skippedSummary, skippedRelay, skippedThemes uint64
	bundle.Summaries, skippedSummary = dedupeReviewRows(bundle.Summaries, func(row model.LimitDailySummary) string { return row.TradeDate.Format("2006-01-02") }, onConflict)
	bundle.RelayEvents, skippedRelay = dedupeReviewRows(bundle.RelayEvents, func(row model.LimitRelayEvent) string {
		return fmt.Sprintf("%s|%s|%s|%s", row.TradeDate.Format("2006-01-02"), row.SampleGroup, row.Market, row.Symbol)
	}, onConflict)
	bundle.Themes, skippedThemes = dedupeReviewRows(bundle.Themes, func(row model.LimitThemeDaily) string {
		return fmt.Sprintf("%s|%s", row.TradeDate.Format("2006-01-02"), row.ThemeName)
	}, onConflict)
	skipped += skippedSummary + skippedRelay + skippedThemes
	return bundle, skipped, issues
}

func dedupeReviewRows[T any](rows []T, keyOf func(T) string, onConflict func(string)) ([]T, uint64) {
	seen := make(map[string]T, len(rows))
	var skipped uint64
	for _, row := range rows {
		key := keyOf(row)
		if prev, ok := seen[key]; ok {
			skipped++
			if !reflect.DeepEqual(prev, row) {
				onConflict(key)
			}
			continue
		}
		seen[key] = row
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]T, 0, len(keys))
	for _, key := range keys {
		out = append(out, seen[key])
	}
	return out, skipped
}

func writeLimitReviewBundle(ctx context.Context, store LimitReviewWriter, bundle limitReviewBundle) (uint64, error) {
	var written uint64
	if err := store.InsertLimitEvents(ctx, bundle.Events); err != nil {
		return written, err
	}
	written += uint64(len(bundle.Events))
	if err := store.InsertLimitDailySummaries(ctx, bundle.Summaries); err != nil {
		return written, err
	}
	written += uint64(len(bundle.Summaries))
	if err := store.InsertLimitRelayEvents(ctx, bundle.RelayEvents); err != nil {
		return written, err
	}
	written += uint64(len(bundle.RelayEvents))
	if err := store.InsertLimitThemeDaily(ctx, bundle.Themes); err != nil {
		return written, err
	}
	return written + uint64(len(bundle.Themes)), nil
}

func discoverLimitReviewFiles(file, root string) ([]string, error) {
	if strings.TrimSpace(file) != "" {
		return []string{expandHome(strings.TrimSpace(file))}, nil
	}
	root = expandHome(strings.TrimSpace(root))
	if root == "" {
		return nil, fmt.Errorf("either --file or --root is required")
	}
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".json") && !strings.Contains(path, string(filepath.Separator)+"evidence"+string(filepath.Separator)) {
			paths = append(paths, path)
		}
		return nil
	})
	sort.Strings(paths)
	return paths, err
}

func normalizeLimitReviewImportOptions(opts LimitReviewImportOptions) (*time.Location, func() time.Time, *time.Time, *time.Time, error) {
	if opts.PercentUnit != "" && opts.PercentUnit != "percent" && opts.PercentUnit != "ratio" {
		return nil, nil, nil, nil, fmt.Errorf("percent-unit must be percent or ratio")
	}
	timezone := opts.Timezone
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	since, err := parseOptionalReviewDate(opts.Since, loc)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("--since: %w", err)
	}
	until, err := parseOptionalReviewDate(opts.Until, loc)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("--until: %w", err)
	}
	if since != nil && until != nil && since.After(*until) {
		return nil, nil, nil, nil, fmt.Errorf("--since must be <= --until")
	}
	return loc, now, since, until, nil
}

func parseReviewDate(value string, loc *time.Location) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(value), loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q, expected YYYY-MM-DD", value)
	}
	return t, nil
}

func parseOptionalReviewDate(value string, loc *time.Location) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	t, err := parseReviewDate(value, loc)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func normalizeReviewMinute(value *string) (*string, error) {
	if value == nil || strings.TrimSpace(*value) == "" || strings.TrimSpace(*value) == "-" {
		return nil, nil
	}
	raw := strings.TrimSpace(*value)
	for _, layout := range []string{"15:04", "15:04:05", "150405"} {
		if t, err := time.Parse(layout, raw); err == nil {
			normalized := t.Format("15:04")
			return &normalized, nil
		}
	}
	return nil, fmt.Errorf("invalid minute %q", raw)
}

func normalizeTags(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func validReviewSymbol(symbol string) bool {
	if len(symbol) != 6 {
		return false
	}
	for _, r := range symbol {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func validateOptionalNonNegative(name string, value *float64) error {
	if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0) {
		return fmt.Errorf("%s must be finite and non-negative", name)
	}
	return nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func limitEventKey(row model.LimitEvent) string {
	return fmt.Sprintf("%s|%s|%s|%s", row.TradeDate.Format("2006-01-02"), row.EventType, row.Market, row.Symbol)
}

func summaryWatermarks(rows []model.LimitDailySummary) (*time.Time, *time.Time) {
	if len(rows) == 0 {
		return nil, nil
	}
	min, max := rows[0].TradeDate, rows[0].TradeDate
	for _, row := range rows[1:] {
		if row.TradeDate.Before(min) {
			min = row.TradeDate
		}
		if row.TradeDate.After(max) {
			max = row.TradeDate
		}
	}
	return &min, &max
}

func limitReviewIssue(runID, path, issueType, tradeDate, symbol, message, severity string, now func() time.Time) model.QualityIssue {
	market := ""
	if symbol != "" {
		market = tdx.InferMarketFromCode(symbol)
	}
	key := strings.Trim(tradeDate+"|"+symbol, "|")
	if key == "" && path != "" {
		key = filepath.Base(path)
	}
	return model.QualityIssue{RunID: runID, Dataset: limitReviewDataset, Severity: severity, IssueType: issueType, Market: market, Symbol: symbol, LogicalKey: key, InputPath: path, ObservedAt: now(), Message: message}
}

func recordLimitReviewTaskRun(ctx context.Context, store LimitReviewWriter, runID, taskType, target, inputPath, inputFormat, params string, started time.Time, now func() time.Time, rowsWritten, rowsSkipped uint64, status string, failure error) error {
	return recordDatasetTaskRun(ctx, store, limitReviewDataset, runID, taskType, target, inputPath, inputFormat, params, started, now, rowsWritten, rowsSkipped, status, failure)
}

func recordDatasetTaskRun(ctx context.Context, store LimitReviewWriter, dataset, runID, taskType, target, inputPath, inputFormat, params string, started time.Time, now func() time.Time, rowsWritten, rowsSkipped uint64, status string, failure error) error {
	finished := now()
	duration := uint64(finished.Sub(started).Milliseconds())
	errText := ""
	if failure != nil {
		errText = failure.Error()
	}
	return store.InsertTaskRun(ctx, model.TaskRun{RunID: runID, Dataset: dataset, TaskType: taskType, Status: status, TargetTable: target, InputPath: inputPath, InputFormat: inputFormat, Params: params, StartedAt: started, FinishedAt: &finished, DurationMS: &duration, RowsWritten: rowsWritten, RowsSkipped: rowsSkipped, Error: errText, UpdatedAt: finished})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type rawLimitCorrection struct {
	TradeDate string                    `json:"trade_date"`
	Mode      string                    `json:"mode"`
	Reason    string                    `json:"reason"`
	AuditRef  string                    `json:"audit_ref"`
	Events    []rawLimitCorrectionEvent `json:"events"`
}

type rawLimitCorrectionEvent struct {
	Code             string   `json:"code"`
	EventType        string   `json:"event_type"`
	CloseStatus      string   `json:"close_status"`
	BoardCount       uint16   `json:"board_count"`
	ReasonText       string   `json:"reason_text"`
	ReasonType       string   `json:"reason_type"`
	ThemePrimary     string   `json:"theme_primary"`
	ThemeTags        []string `json:"theme_tags"`
	FirstLimitMinute *string  `json:"first_limit_minute"`
	LastLimitMinute  *string  `json:"last_limit_minute"`
	OpenCount        *uint16  `json:"open_count"`
	SealOrderAmount  *float64 `json:"seal_order_amount"`
	Amount           *float64 `json:"amount"`
	TurnoverRate     *float64 `json:"turnover_rate"`
	MarketValue      *float64 `json:"market_value"`
}

func ImportLimitReviewCorrections(ctx context.Context, opts LimitReviewImportOptions) (summary LimitReviewImportSummary, retErr error) {
	return importLimitReviewCorrections(ctx, opts, nil)
}

// ImportLimitReviewCorrectionsReader shares file validation without staging HTTP data on disk.
func ImportLimitReviewCorrectionsReader(ctx context.Context, input io.Reader, opts LimitReviewImportOptions) (LimitReviewImportSummary, error) {
	if input == nil {
		return LimitReviewImportSummary{}, fmt.Errorf("correction reader is required")
	}
	opts.File = "http:limit-review-corrections"
	return importLimitReviewCorrections(ctx, opts, input)
}

func importLimitReviewCorrections(ctx context.Context, opts LimitReviewImportOptions, input io.Reader) (summary LimitReviewImportSummary, retErr error) {
	loc, now, since, until, err := normalizeLimitReviewImportOptions(opts)
	if err != nil {
		return summary, err
	}
	path := expandHome(strings.TrimSpace(opts.File))
	if path == "" {
		return summary, fmt.Errorf("--file is required")
	}
	if !opts.DryRun && opts.Store == nil {
		return summary, fmt.Errorf("store is required when dry-run is false")
	}
	runID := newRunID()
	started := now()
	summary = LimitReviewImportSummary{RunID: runID, Dataset: limitReviewDataset, TargetTable: limitReviewTarget, DryRun: opts.DryRun}
	audits := []map[string]string{}
	defer func() {
		if opts.DryRun {
			return
		}
		status := "success"
		if retErr != nil {
			status = "failed"
		} else if len(summary.Issues) > 0 {
			status = "degraded"
		}
		if retErr != nil {
			summary.Issues = append(summary.Issues, limitReviewIssue(runID, path, "correction_failed", "", "", retErr.Error(), "error", now))
		}
		if len(summary.Issues) > 0 {
			if err := opts.Store.InsertQualityIssues(ctx, summary.Issues); err != nil {
				status = "failed"
				if retErr == nil {
					retErr = err
				}
			}
		}
		params, _ := json.Marshal(map[string]any{"allow_fact_replacement": opts.AllowFactReplacement, "since": opts.Since, "until": opts.Until, "corrections": audits})
		if err := recordLimitReviewTaskRun(ctx, opts.Store, runID, "limit_review_correction_import", limitReviewTarget, path, "jsonl.limit_review_corrections.v1", string(params), started, now, summary.RowsWritten, summary.RowsSkipped, status, retErr); retErr == nil {
			retErr = err
		}
	}()
	if input == nil {
		file, err := os.Open(path)
		if err != nil {
			return summary, err
		}
		defer file.Close()
		input = file
		summary.FilesRead = 1
	}
	var events []model.LimitEvent
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	line := 0
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		line++
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var correction rawLimitCorrection
		if err := json.Unmarshal(scanner.Bytes(), &correction); err != nil {
			return summary, fmt.Errorf("line %d: %w", line, err)
		}
		decoder := json.NewDecoder(strings.NewReader(scanner.Text()))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&correction); err != nil {
			return summary, fmt.Errorf("line %d: %w", line, err)
		}
		if correction.Mode != "enrich_existing" && (correction.Mode != "upsert" || !opts.AllowFactReplacement) {
			return summary, fmt.Errorf("line %d: mode enrich_existing required; upsert needs explicit operator fact replacement", line)
		}
		date, err := parseReviewDate(correction.TradeDate, loc)
		if err != nil {
			return summary, fmt.Errorf("line %d: %w", line, err)
		}
		if (since != nil && date.Before(*since)) || (until != nil && date.After(*until)) {
			summary.RowsSkipped += uint64(len(correction.Events))
			continue
		}
		if strings.TrimSpace(correction.Reason) == "" {
			return summary, fmt.Errorf("line %d: reason is required", line)
		}
		if len(correction.Events) == 0 {
			return summary, fmt.Errorf("line %d: events must not be empty", line)
		}
		audits = append(audits, map[string]string{"trade_date": correction.TradeDate, "mode": correction.Mode, "reason": correction.Reason, "audit_ref": correction.AuditRef})
		for _, r := range correction.Events {
			if r.CloseStatus == "" {
				return summary, fmt.Errorf("line %d: close_status is required", line)
			}
			event, err := normalizeLimitStock(date, r.EventType, rawLimitStock{Code: r.Code, BoardCount: r.BoardCount, ReasonType: firstNonEmpty(r.ReasonText, r.ReasonType), ThemePrimary: r.ThemePrimary, ThemeTags: r.ThemeTags, OrderAmount: r.SealOrderAmount, Amount: r.Amount, FirstLimitUpTime: r.FirstLimitMinute, LastLimitUpTime: r.LastLimitMinute, TurnoverRate: r.TurnoverRate, OpenNum: r.OpenCount, Status: r.CloseStatus, MarketValue: r.MarketValue})
			if err != nil {
				return summary, fmt.Errorf("line %d symbol %q: %w", line, r.Code, err)
			}
			events = append(events, event)
		}
	}
	if err := scanner.Err(); err != nil {
		return summary, err
	}
	if len(events) == 0 {
		return summary, fmt.Errorf("no corrections within requested date range")
	}
	bundle, skipped, issues := dedupeLimitReviewBundle(limitReviewBundle{Events: events}, runID, now)
	summary.Events = uint64(len(bundle.Events))
	summary.RowsSkipped += skipped
	summary.Issues = issues
	if len(issues) > 0 {
		return summary, fmt.Errorf("conflicting corrections in one payload")
	}
	for _, audit := range audits {
		if audit["mode"] != "enrich_existing" {
			continue
		}
		if opts.LoadEvents == nil {
			return summary, fmt.Errorf("current-event reader required for enrichment")
		}
		current, err := opts.LoadEvents(ctx, audit["trade_date"])
		if err != nil {
			return summary, err
		}
		byKey := make(map[string]model.LimitEvent, len(current))
		for _, row := range current {
			byKey[row.Market+":"+row.Symbol+":"+row.EventType] = row
		}
		for _, row := range bundle.Events {
			if row.TradeDate.Format("2006-01-02") != audit["trade_date"] {
				continue
			}
			before, ok := byKey[row.Market+":"+row.Symbol+":"+row.EventType]
			if !ok {
				return summary, fmt.Errorf("enrichment requires existing event: %s %s", audit["trade_date"], row.Symbol)
			}
			if err := validateLimitEnrichment(before, row); err != nil {
				return summary, err
			}
		}
	}
	if opts.DryRun {
		summary.RowsWritten = summary.Events
		return summary, nil
	}
	if err := opts.Store.InsertLimitEvents(ctx, bundle.Events); err != nil {
		return summary, err
	}
	summary.RowsWritten = summary.Events
	dates := make([]time.Time, 0, len(bundle.Events))
	for _, r := range bundle.Events {
		dates = append(dates, r.TradeDate)
	}
	min, max := timeBounds(dates)
	if err := opts.Store.InsertWatermark(ctx, model.Watermark{Dataset: "a_share_limit_review_corrections", Asset: "all", Status: "success", MinWatermark: min, MaxWatermark: max, RowsWritten: summary.RowsWritten, UpdatedAt: now()}); err != nil {
		return summary, err
	}
	return summary, nil
}

func validateLimitEnrichment(before, after model.LimitEvent) error {
	if before.ReasonText != "" && before.ReasonText != after.ReasonText {
		return fmt.Errorf("existing reason conflict: %s", before.Symbol)
	}
	if before.ThemePrimary != "" && before.ThemePrimary != "未分类" && before.ThemePrimary != after.ThemePrimary {
		return fmt.Errorf("existing theme conflict: %s", before.Symbol)
	}
	before.ReasonText, before.ThemePrimary = after.ReasonText, after.ThemePrimary
	// Compare calendar identity rather than time.Location pointer identity.
	if !before.TradeDate.Equal(after.TradeDate) {
		return fmt.Errorf("event date changed")
	}
	before.TradeDate = after.TradeDate
	if len(before.ThemeTags) == 0 && len(after.ThemeTags) == 0 {
		before.ThemeTags = after.ThemeTags
	}
	if !reflect.DeepEqual(before, after) {
		return fmt.Errorf("enrichment cannot change market facts or tags: %s", before.Symbol)
	}
	return nil
}
