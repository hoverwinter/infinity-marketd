package clickhouse

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/hoverwinter/infinity-marketd/internal/model"
)

type fakeConn struct {
	queries []string
	batches []*fakeBatch
}

func (c *fakeConn) Contributors() []string                                       { return nil }
func (c *fakeConn) ServerVersion() (*chdriver.ServerVersion, error)              { return nil, nil }
func (c *fakeConn) Select(context.Context, any, string, ...any) error            { return nil }
func (c *fakeConn) Query(context.Context, string, ...any) (chdriver.Rows, error) { return nil, nil }
func (c *fakeConn) QueryRow(context.Context, string, ...any) chdriver.Row        { return nil }
func (c *fakeConn) Exec(context.Context, string, ...any) error                   { return nil }
func (c *fakeConn) AsyncInsert(context.Context, string, bool, ...any) error      { return nil }
func (c *fakeConn) Ping(context.Context) error                                   { return nil }
func (c *fakeConn) Stats() chdriver.Stats                                        { return chdriver.Stats{} }
func (c *fakeConn) Close() error                                                 { return nil }

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
	if len(conn.queries) != 0 {
		t.Fatalf("queries=%d", len(conn.queries))
	}
}
