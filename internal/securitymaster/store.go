package securitymaster

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/hoverwinter/infinity-marketd/internal/config"
)

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, cfg config.MySQLConfig) (*Store, error) {
	if err := cfg.RequiredError(); err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", mysqlDSN(cfg, true))
	if err != nil {
		return nil, err
	}
	configureDB(db, cfg)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func Bootstrap(ctx context.Context, cfg config.MySQLConfig) error {
	if err := cfg.RequiredError(); err != nil {
		return err
	}
	db, err := sql.Open("mysql", mysqlDSN(cfg, false))
	if err != nil {
		return err
	}
	defer db.Close()
	configureDB(db, cfg)
	if err := db.PingContext(ctx); err != nil {
		return err
	}
	ddl, err := BootstrapDDL(cfg.Database)
	if err != nil {
		return err
	}
	for _, stmt := range ddl {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func mysqlDSN(cfg config.MySQLConfig, withDatabase bool) string {
	dbName := ""
	if withDatabase {
		dbName = cfg.Database
	}
	mysqlCfg := mysql.Config{
		User:                 cfg.User,
		Passwd:               cfg.Password,
		Net:                  "tcp",
		Addr:                 cfg.Addr,
		DBName:               dbName,
		ParseTime:            true,
		AllowNativePasswords: true,
		Params: map[string]string{
			"charset": "utf8mb4",
			"loc":     "UTC",
		},
	}
	return mysqlCfg.FormatDSN()
}

func configureDB(db *sql.DB, cfg config.MySQLConfig) {
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime.Duration())
}

func (s *Store) Security(ctx context.Context, market string, symbol string) (Security, error) {
	market, err := NormalizeMarket(market)
	if err != nil {
		return Security{}, err
	}
	symbol, err = NormalizeSymbol(symbol)
	if err != nil {
		return Security{}, err
	}
	row := s.db.QueryRowContext(ctx, securitySelectSQL()+" WHERE market = ? AND symbol = ?", market, symbol)
	return scanSecurity(row)
}

func (s *Store) Resolve(ctx context.Context, query string, limit int) ([]ResolveCandidate, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("q is required")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	norm := NormalizeText(query)
	candidates := make(map[string]ResolveCandidate)
	add := func(candidate ResolveCandidate) {
		key := candidate.Security.Market + ":" + candidate.Security.Symbol
		if existing, ok := candidates[key]; ok && existing.Score >= candidate.Score {
			return
		}
		candidates[key] = candidate
	}
	if symbolPattern.MatchString(query) {
		rows, err := s.db.QueryContext(ctx, securitySelectSQL()+" WHERE symbol = ? ORDER BY market LIMIT ?", query, limit)
		if err != nil {
			return nil, err
		}
		if err := scanCandidateRows(rows, "symbol", query, 100, add); err != nil {
			return nil, err
		}
	}
	if norm != "" {
		pattern := "%" + escapeLike(norm) + "%"
		rows, err := s.db.QueryContext(ctx, securitySelectSQL()+" WHERE current_name_norm = ? OR current_name_norm LIKE ? ESCAPE '\\\\' ORDER BY (current_name_norm = ?) DESC, current_name LIMIT ?", norm, pattern, norm, limit)
		if err != nil {
			return nil, err
		}
		if err := scanCandidateRows(rows, "current_name", query, 90, add); err != nil {
			return nil, err
		}
		historySQL := "SELECT s.market, s.symbol, s.exchange, s.current_name, s.current_name_norm, s.board, s.status, s.listing_date, s.delisting_date, s.lot_size, s.price_precision, s.source, s.manual_locked, s.created_at, s.updated_at, h.name FROM security_name_history h JOIN securities s ON s.market = h.market AND s.symbol = h.symbol WHERE h.name_norm = ? OR h.name_norm LIKE ? ESCAPE '\\\\' ORDER BY (h.name_norm = ?) DESC, h.valid_from DESC LIMIT ?"
		rows, err = s.db.QueryContext(ctx, historySQL, norm, pattern, norm, limit)
		if err != nil {
			return nil, err
		}
		if err := scanCandidateRowsWithMatched(rows, "history_name", 80, add); err != nil {
			return nil, err
		}
		aliasSQL := "SELECT s.market, s.symbol, s.exchange, s.current_name, s.current_name_norm, s.board, s.status, s.listing_date, s.delisting_date, s.lot_size, s.price_precision, s.source, s.manual_locked, s.created_at, s.updated_at, a.alias FROM security_aliases a JOIN securities s ON s.market = a.market AND s.symbol = a.symbol WHERE a.alias_norm = ? OR a.alias_norm LIKE ? ESCAPE '\\\\' ORDER BY (a.alias_norm = ?) DESC, a.priority DESC LIMIT ?"
		rows, err = s.db.QueryContext(ctx, aliasSQL, norm, pattern, norm, limit)
		if err != nil {
			return nil, err
		}
		if err := scanCandidateRowsWithMatched(rows, "alias", 70, add); err != nil {
			return nil, err
		}
	}
	out := make([]ResolveCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, candidate)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].Security.Market != out[j].Security.Market {
			return out[i].Security.Market < out[j].Security.Market
		}
		return out[i].Security.Symbol < out[j].Security.Symbol
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) BeginRefreshRun(ctx context.Context, run RefreshRun) (int64, error) {
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	}
	if run.Status == "" {
		run.Status = RefreshStatusRunning
	}
	res, err := s.db.ExecContext(ctx, "INSERT INTO security_refresh_runs (source, markets, started_at, status, rows_seen, rows_upserted, rows_skipped, aliases_upserted, history_upserted, error) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		run.Source, marketsCSV(run.Markets), run.StartedAt, run.Status, run.RowsSeen, run.RowsUpserted, run.RowsSkipped, run.AliasesUpserted, run.HistoryUpserted, nullableString(run.Error))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) FinishRefreshRun(ctx context.Context, id int64, run RefreshRun) error {
	_, err := s.db.ExecContext(ctx, "UPDATE security_refresh_runs SET finished_at = ?, status = ?, rows_seen = ?, rows_upserted = ?, rows_skipped = ?, aliases_upserted = ?, history_upserted = ?, error = ? WHERE id = ?",
		nullableTime(run.FinishedAt), run.Status, run.RowsSeen, run.RowsUpserted, run.RowsSkipped, run.AliasesUpserted, run.HistoryUpserted, nullableString(run.Error), id)
	return err
}

func (s *Store) UpsertSecurity(ctx context.Context, security Security) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `INSERT INTO securities
(market, symbol, exchange, current_name, current_name_norm, board, status, listing_date, delisting_date, lot_size, price_precision, source, manual_locked, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    exchange = IF(manual_locked, exchange, VALUES(exchange)),
    current_name = IF(manual_locked, current_name, VALUES(current_name)),
    current_name_norm = IF(manual_locked, current_name_norm, VALUES(current_name_norm)),
    board = IF(manual_locked, board, VALUES(board)),
    status = IF(manual_locked, status, VALUES(status)),
    listing_date = IF(manual_locked, listing_date, VALUES(listing_date)),
    delisting_date = IF(manual_locked, delisting_date, VALUES(delisting_date)),
    lot_size = IF(manual_locked, lot_size, VALUES(lot_size)),
    price_precision = IF(manual_locked, price_precision, VALUES(price_precision)),
    source = IF(manual_locked, source, VALUES(source)),
    updated_at = VALUES(updated_at)`,
		security.Market, security.Symbol, security.Exchange, security.CurrentName, security.CurrentNameNorm, security.Board, security.Status,
		nullableDate(security.ListingDate), nullableDate(security.DelistingDate), nullablePositiveInt(security.LotSize), nullableNonNegativeInt(security.PricePrecision),
		security.Source, security.ManualLocked, now, now)
	return err
}

func (s *Store) UpsertAliases(ctx context.Context, aliases []Alias) (int, error) {
	count := 0
	now := time.Now().UTC()
	for _, alias := range aliases {
		_, err := s.db.ExecContext(ctx, `INSERT INTO security_aliases
(market, symbol, alias, alias_norm, alias_type, priority, source, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE alias = VALUES(alias), priority = VALUES(priority), source = VALUES(source), updated_at = VALUES(updated_at)`,
			alias.Market, alias.Symbol, alias.Alias, alias.AliasNorm, alias.AliasType, alias.Priority, alias.Source, now, now)
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *Store) UpsertNameHistory(ctx context.Context, history []NameHistory) (int, error) {
	count := 0
	now := time.Now().UTC()
	for _, item := range history {
		_, err := s.db.ExecContext(ctx, `INSERT INTO security_name_history
(market, symbol, name, name_norm, valid_from, valid_to, source, manual_override, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    name = IF(manual_override, name, VALUES(name)),
    valid_to = IF(manual_override, valid_to, VALUES(valid_to)),
    source = IF(manual_override, source, VALUES(source)),
    updated_at = VALUES(updated_at)`,
			item.Market, item.Symbol, item.Name, item.NameNorm, nullableDate(item.ValidFrom), nullableDate(item.ValidTo), item.Source, item.ManualOverride, now, now)
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func securitySelectSQL() string {
	return "SELECT market, symbol, exchange, current_name, current_name_norm, board, status, listing_date, delisting_date, lot_size, price_precision, source, manual_locked, created_at, updated_at FROM securities"
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSecurity(row scanner) (Security, error) {
	var security Security
	var listingDate sql.NullTime
	var delistingDate sql.NullTime
	var lotSize sql.NullInt64
	var pricePrecision sql.NullInt64
	err := row.Scan(
		&security.Market,
		&security.Symbol,
		&security.Exchange,
		&security.CurrentName,
		&security.CurrentNameNorm,
		&security.Board,
		&security.Status,
		&listingDate,
		&delistingDate,
		&lotSize,
		&pricePrecision,
		&security.Source,
		&security.ManualLocked,
		&security.CreatedAt,
		&security.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Security{}, ErrNotFound
	}
	if err != nil {
		return Security{}, err
	}
	security.ListingDate = formatNullDate(listingDate)
	security.DelistingDate = formatNullDate(delistingDate)
	if lotSize.Valid {
		security.LotSize = int(lotSize.Int64)
	}
	if pricePrecision.Valid {
		security.PricePrecision = int(pricePrecision.Int64)
	}
	return security, nil
}

func scanCandidateRows(rows *sql.Rows, matchType string, matchedText string, score int, add func(ResolveCandidate)) error {
	defer rows.Close()
	for rows.Next() {
		security, err := scanSecurity(rows)
		if err != nil {
			return err
		}
		add(ResolveCandidate{Security: security, MatchType: matchType, MatchedText: matchedText, Score: score})
	}
	return rows.Err()
}

func scanCandidateRowsWithMatched(rows *sql.Rows, matchType string, score int, add func(ResolveCandidate)) error {
	defer rows.Close()
	for rows.Next() {
		var matchedText string
		wrapped := rowWithTrailingText{rows: rows, trailing: &matchedText}
		security, err := scanSecurity(wrapped)
		if err != nil {
			return err
		}
		add(ResolveCandidate{Security: security, MatchType: matchType, MatchedText: matchedText, Score: score})
	}
	return rows.Err()
}

type rowWithTrailingText struct {
	rows     *sql.Rows
	trailing *string
}

func (r rowWithTrailingText) Scan(dest ...any) error {
	dest = append(dest, r.trailing)
	return r.rows.Scan(dest...)
}

func nullableDate(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullablePositiveInt(value int) any {
	if value <= 0 {
		return nil
	}
	return value
}

func nullableNonNegativeInt(value int) any {
	if value < 0 {
		return nil
	}
	return value
}

func formatNullDate(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02")
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

var _ Repository = (*Store)(nil)
