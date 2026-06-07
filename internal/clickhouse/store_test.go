package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/hoverwinter/infinity-marketd/internal/model"
)

type fakeConn struct {
	queries      []string
	queryArgs    [][]any
	queryRows    []string
	queryRowArgs [][]any
	execs        []string
	execArgs     [][]any
	batches      []*fakeBatch
	row          chdriver.Row
}

func (c *fakeConn) Contributors() []string                            { return nil }
func (c *fakeConn) ServerVersion() (*chdriver.ServerVersion, error)   { return nil, nil }
func (c *fakeConn) Select(context.Context, any, string, ...any) error { return nil }
func (c *fakeConn) Query(_ context.Context, query string, args ...any) (chdriver.Rows, error) {
	c.queries = append(c.queries, query)
	c.queryArgs = append(c.queryArgs, append([]any(nil), args...))
	return nil, nil
}
func (c *fakeConn) QueryRow(_ context.Context, query string, args ...any) chdriver.Row {
	c.queryRows = append(c.queryRows, query)
	c.queryRowArgs = append(c.queryRowArgs, append([]any(nil), args...))
	if c.row != nil {
		return c.row
	}
	return fakeRow{}
}
func (c *fakeConn) Exec(_ context.Context, query string, args ...any) error {
	c.execs = append(c.execs, query)
	c.execArgs = append(c.execArgs, append([]any(nil), args...))
	return nil
}
func (c *fakeConn) AsyncInsert(context.Context, string, bool, ...any) error { return nil }
func (c *fakeConn) Ping(context.Context) error                              { return nil }
func (c *fakeConn) Stats() chdriver.Stats                                   { return chdriver.Stats{} }
func (c *fakeConn) Close() error                                            { return nil }

func (c *fakeConn) PrepareBatch(_ context.Context, query string, _ ...chdriver.PrepareBatchOption) (chdriver.Batch, error) {
	batch := &fakeBatch{}
	c.queries = append(c.queries, query)
	c.batches = append(c.batches, batch)
	return batch, nil
}

type fakeBatch struct {
	rows [][]any
	sent bool
}

func (b *fakeBatch) Abort() error { return nil }
func (b *fakeBatch) Append(v ...any) error {
	b.rows = append(b.rows, append([]any(nil), v...))
	return nil
}
func (b *fakeBatch) AppendStruct(any) error          { return nil }
func (b *fakeBatch) Column(int) chdriver.BatchColumn { return nil }
func (b *fakeBatch) Flush() error                    { return nil }
func (b *fakeBatch) Send() error {
	b.sent = true
	return nil
}
func (b *fakeBatch) IsSent() bool                { return b.sent }
func (b *fakeBatch) Rows() int                   { return len(b.rows) }
func (b *fakeBatch) Columns() []column.Interface { return nil }
func (b *fakeBatch) Close() error                { return nil }

type fakeRow struct {
	values []any
	err    error
}

func (r fakeRow) Err() error { return r.err }
func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) > len(r.values) {
		return fmt.Errorf("scan destination count %d exceeds values %d", len(dest), len(r.values))
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *uint64:
			v, ok := r.values[i].(uint64)
			if !ok {
				return fmt.Errorf("value %d is %T, not uint64", i, r.values[i])
			}
			*d = v
		default:
			return fmt.Errorf("unsupported scan destination %T", dest[i])
		}
	}
	return nil
}
func (r fakeRow) ScanStruct(any) error { return r.err }

func TestInsertIntradayPoints(t *testing.T) {
	conn := &fakeConn{}
	store := &Store{conn: conn, marketDB: "infinity_market", opsDB: "infinity_ops"}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	tradeDate := time.Date(2026, 6, 5, 0, 0, 0, 0, loc)
	pointTime := time.Date(2026, 6, 5, 9, 30, 0, 0, loc)

	err := store.InsertIntradayPoints(context.Background(), []model.IntradayPoint{
		{Market: "sh", Symbol: "600519", TradeDate: tradeDate, PointTime: pointTime, PointIndex: 0, Price: 12.34, Volume: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(conn.queries) != 1 {
		t.Fatalf("queries=%d", len(conn.queries))
	}
	for _, want := range []string{
		"INSERT INTO `infinity_market`.`a_share_intraday_points`",
		"(market, symbol, trade_date, point_time, point_index, price, volume)",
	} {
		if !strings.Contains(conn.queries[0], want) {
			t.Fatalf("query missing %q: %s", want, conn.queries[0])
		}
	}
	if len(conn.batches) != 1 || !conn.batches[0].sent {
		t.Fatalf("batch not sent: %#v", conn.batches)
	}
	row := conn.batches[0].rows[0]
	if row[0] != "sh" || row[1] != "600519" || row[4] != uint16(0) || row[5] != 12.34 || row[6] != uint64(100) {
		t.Fatalf("row=%#v", row)
	}
}

func TestInsertIntradayPointsEmptyBatch(t *testing.T) {
	conn := &fakeConn{}
	store := &Store{conn: conn, marketDB: "infinity_market", opsDB: "infinity_ops"}
	if err := store.InsertIntradayPoints(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(conn.queries) != 0 {
		t.Fatalf("queries=%d", len(conn.queries))
	}
}

func TestInsertDailyDerived(t *testing.T) {
	conn := &fakeConn{}
	store := &Store{conn: conn, marketDB: "infinity_market", opsDB: "infinity_ops"}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	tradeDate := time.Date(2026, 6, 5, 0, 0, 0, 0, loc)
	prevClose := 10.0
	pctChg := 12.3

	err := store.InsertDailyDerived(context.Background(), []model.DailyDerived{
		{Market: "sh", Symbol: "600519", TradeDate: tradeDate, PrevClose: &prevClose, PctChg: &pctChg},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(conn.queries) != 1 {
		t.Fatalf("queries=%d", len(conn.queries))
	}
	for _, want := range []string{
		"INSERT INTO `infinity_market`.`a_share_daily_derived`",
		"(market, symbol, trade_date, prev_close, pct_chg, computed_at)",
	} {
		if !strings.Contains(conn.queries[0], want) {
			t.Fatalf("query missing %q: %s", want, conn.queries[0])
		}
	}
	row := conn.batches[0].rows[0]
	if row[0] != "sh" || row[1] != "600519" || row[3] != &prevClose || row[4] != &pctChg {
		t.Fatalf("row=%#v", row)
	}
}

func TestRefreshMinuteScanUsesBoundedInsertSelect(t *testing.T) {
	conn := &fakeConn{row: fakeRow{values: []any{uint64(3)}}}
	store := &Store{conn: conn, marketDB: "infinity_market", opsDB: "infinity_ops"}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	since := time.Date(2026, 6, 1, 0, 0, 0, 0, loc)
	until := time.Date(2026, 6, 7, 0, 0, 0, 0, loc)

	rows, err := store.RefreshMinuteScan(context.Background(), MinuteScanRefresh{Period: "1m", Since: since, Until: until})
	if err != nil {
		t.Fatal(err)
	}
	if rows != 3 {
		t.Fatalf("rows=%d", rows)
	}
	if len(conn.queryRows) != 1 || len(conn.execs) != 1 {
		t.Fatalf("queryRows=%d execs=%d", len(conn.queryRows), len(conn.execs))
	}
	for _, want := range []string{
		"SELECT count() FROM `infinity_market`.`a_share_bars_1m` FINAL",
		"WHERE trade_date >= ? AND trade_date <= ?",
	} {
		if !strings.Contains(conn.queryRows[0], want) {
			t.Fatalf("count SQL missing %q:\n%s", want, conn.queryRows[0])
		}
	}
	for _, want := range []string{
		"INSERT INTO `infinity_market`.`a_share_bars_1m_scan`",
		"FROM `infinity_market`.`a_share_bars_1m` FINAL",
		"WHERE trade_date >= ? AND trade_date <= ?",
		"lagInFrame(toNullable(close), 1)",
		"minute_ret",
		"CAST(NULL, 'Nullable(Float64)') AS volume_ratio",
		"ORDER BY trade_date, bar_time, market, symbol",
	} {
		if !strings.Contains(conn.execs[0], want) {
			t.Fatalf("refresh SQL missing %q:\n%s", want, conn.execs[0])
		}
	}
	for _, forbidden := range []string{"DROP ", "TRUNCATE ", "DETACH ", "DELETE ", "ALTER TABLE"} {
		if strings.Contains(strings.ToUpper(conn.execs[0]), forbidden) {
			t.Fatalf("refresh SQL contains destructive operation %q:\n%s", forbidden, conn.execs[0])
		}
	}
	if len(conn.queryRowArgs[0]) != 2 || len(conn.execArgs[0]) != 2 {
		t.Fatalf("args query=%#v exec=%#v", conn.queryRowArgs[0], conn.execArgs[0])
	}
}

func TestRefreshMinuteScanSupportsFiveMinuteSource(t *testing.T) {
	conn := &fakeConn{row: fakeRow{values: []any{uint64(1)}}}
	store := &Store{conn: conn, marketDB: "infinity_market", opsDB: "infinity_ops"}
	day := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)

	if _, err := store.RefreshMinuteScan(context.Background(), MinuteScanRefresh{Period: "5m", Since: day, Until: day}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(conn.execs[0], "`infinity_market`.`a_share_bars_5m_scan`") {
		t.Fatalf("refresh SQL missing 5m scan target:\n%s", conn.execs[0])
	}
	if !strings.Contains(conn.execs[0], "`infinity_market`.`a_share_bars_5m`") {
		t.Fatalf("refresh SQL missing 5m source:\n%s", conn.execs[0])
	}
}

func TestRefreshMinuteScanRejectsUnsupportedPeriod(t *testing.T) {
	conn := &fakeConn{}
	store := &Store{conn: conn, marketDB: "infinity_market", opsDB: "infinity_ops"}
	day := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)

	if _, err := store.RefreshMinuteScan(context.Background(), MinuteScanRefresh{Period: "1d", Since: day, Until: day}); err == nil {
		t.Fatal("expected unsupported period error")
	}
	if len(conn.queryRows) != 0 || len(conn.execs) != 0 {
		t.Fatalf("unexpected queries=%d execs=%d", len(conn.queryRows), len(conn.execs))
	}
}

func TestInsertLocalReferenceTables(t *testing.T) {
	ctx := context.Background()
	loc, _ := time.LoadLocation("Asia/Shanghai")
	tradeDate := time.Date(2026, 6, 5, 0, 0, 0, 0, loc)
	v := 1.2

	tests := []struct {
		name      string
		insert    func(*Store) error
		wantTable string
		wantCols  string
	}{
		{
			name: "capital change events",
			insert: func(store *Store) error {
				return store.InsertCapitalChangeEvents(ctx, []model.CapitalChangeEvent{
					{Market: "sh", Symbol: "600519", EventDate: tradeDate, Category: 1, EventSeq: 0, EventName: "除权除息", CashDividend: &v},
				})
			},
			wantTable: "a_share_capital_change_events",
			wantCols:  "(market, symbol, event_date, category, event_seq, event_name, cash_dividend",
		},
		{
			name: "block snapshots",
			insert: func(store *Store) error {
				return store.InsertTDXBlockSnapshots(ctx, []model.TDXBlockSnapshot{
					{SnapshotID: "snapshot", BlockScope: "system", SnapshotTime: tradeDate, ContentHash: "snapshot", BlockCount: 1, MemberCount: 1},
				})
			},
			wantTable: "tdx_block_snapshots",
			wantCols:  "(snapshot_id, block_scope, snapshot_time, content_hash, block_count, member_count)",
		},
		{
			name: "block definitions",
			insert: func(store *Store) error {
				return store.InsertTDXBlockDefinitions(ctx, []model.TDXBlockDefinition{
					{SnapshotID: "snapshot", BlockScope: "system", BlockKind: "block_gn", BlockID: "test", BlockName: "Test", BlockType: 7, DisplayOrder: 0, MemberCount: 1},
				})
			},
			wantTable: "tdx_block_definitions",
			wantCols:  "(snapshot_id, block_scope, block_kind, block_id, block_name, block_type, display_order, member_count)",
		},
		{
			name: "block memberships",
			insert: func(store *Store) error {
				return store.InsertTDXBlockMemberships(ctx, []model.TDXBlockMembership{
					{SnapshotID: "snapshot", BlockScope: "system", BlockID: "test", MemberOrder: 0, Code: "1600519", Market: "sh", Symbol: "600519"},
				})
			},
			wantTable: "tdx_block_memberships",
			wantCols:  "(snapshot_id, block_scope, block_id, member_order, code, market, symbol)",
		},
		{
			name: "extension daily bars",
			insert: func(store *Store) error {
				return store.InsertExDailyBars(ctx, []model.ExDailyBar{
					{ExMarket: 29, Code: "A1801", TradeDate: tradeDate, Open: 1, High: 2, Low: 0.5, Close: 1.5, Position: 100, Trade: 10, Amount: &v},
				})
			},
			wantTable: "tdx_ex_bars_1d",
			wantCols:  "(ex_market, code, trade_date, open, high, low, close, position, trade, price, amount, settlement_price)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &fakeConn{}
			store := &Store{conn: conn, marketDB: "infinity_market", opsDB: "infinity_ops"}
			if err := tt.insert(store); err != nil {
				t.Fatal(err)
			}
			if len(conn.queries) != 1 {
				t.Fatalf("queries=%d", len(conn.queries))
			}
			if !strings.Contains(conn.queries[0], "INSERT INTO `infinity_market`.`"+tt.wantTable+"`") {
				t.Fatalf("query targets wrong table: %s", conn.queries[0])
			}
			if !strings.Contains(conn.queries[0], tt.wantCols) {
				t.Fatalf("query missing columns %q: %s", tt.wantCols, conn.queries[0])
			}
			if len(conn.batches) != 1 || !conn.batches[0].sent || len(conn.batches[0].rows) != 1 {
				t.Fatalf("batch not sent: %#v", conn.batches)
			}
		})
	}
}

func TestInsertLocalReferenceTablesEmptyBatch(t *testing.T) {
	ctx := context.Background()
	conn := &fakeConn{}
	store := &Store{conn: conn, marketDB: "infinity_market", opsDB: "infinity_ops"}
	if err := store.InsertCapitalChangeEvents(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertTDXBlockSnapshots(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertTDXBlockDefinitions(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertTDXBlockMemberships(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertExDailyBars(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertDailyDerived(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if len(conn.queries) != 0 {
		t.Fatalf("queries=%d", len(conn.queries))
	}
}
