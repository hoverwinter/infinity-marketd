package securitymaster

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestStoreSecurityLookup(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	now := time.Date(2026, 6, 10, 9, 30, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(securitySelectSQL()+" WHERE market = ? AND symbol = ?")).
		WithArgs("sh", "600519").
		WillReturnRows(securityRows().AddRow("sh", "600519", "SSE", "贵州茅台", "贵州茅台", "main", StatusListed, now, nil, 100, 2, SourceTDX, false, now, now))

	security, err := store.Security(context.Background(), "sh", "600519")
	if err != nil {
		t.Fatal(err)
	}
	if security.CurrentName != "贵州茅台" || security.ListingDate != "2026-06-10" {
		t.Fatalf("security = %+v", security)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreResolveReturnsMultipleCandidates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	now := time.Date(2026, 6, 10, 9, 30, 0, 0, time.UTC)
	mock.ExpectQuery("FROM securities WHERE current_name_norm").
		WithArgs("同名", "%同名%", "同名", 20).
		WillReturnRows(securityRows().
			AddRow("sh", "600001", "SSE", "同名股份", "同名股份", "main", StatusListed, nil, nil, 100, 2, SourceFile, false, now, now).
			AddRow("sz", "000001", "SZSE", "同名银行", "同名银行", "main", StatusListed, nil, nil, 100, 2, SourceFile, false, now, now))
	mock.ExpectQuery("FROM security_name_history").
		WithArgs("同名", "%同名%", "同名", 20).
		WillReturnRows(securityRowsWithMatched("name"))
	mock.ExpectQuery("FROM security_aliases").
		WithArgs("同名", "%同名%", "同名", 20).
		WillReturnRows(securityRowsWithMatched("alias"))

	candidates, err := store.Resolve(context.Background(), "同名", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidates = %+v", candidates)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreUpsertSecurityPreservesManualLockedFieldsInSQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	mock.ExpectExec("ON DUPLICATE KEY UPDATE\\s+exchange = IF\\(manual_locked").
		WithArgs(
			"bj", "920001", "BSE", "北证测试", "北证测试", "bse", StatusListed,
			nil, nil, 100, 2, SourceTDX, false, sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = store.UpsertSecurity(context.Background(), Security{
		Market:          "bj",
		Symbol:          "920001",
		Exchange:        "BSE",
		CurrentName:     "北证测试",
		CurrentNameNorm: "北证测试",
		Board:           "bse",
		Status:          StatusListed,
		LotSize:         100,
		PricePrecision:  2,
		Source:          SourceTDX,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreUpsertNameHistoryPreservesManualOverrideInSQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	mock.ExpectExec("ON DUPLICATE KEY UPDATE\\s+name = IF\\(manual_override").
		WithArgs("sh", "600519", "旧名", "旧名", "2020-01-01", nil, SourceFile, false, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	count, err := store.UpsertNameHistory(context.Background(), []NameHistory{{
		Market:    "sh",
		Symbol:    "600519",
		Name:      "旧名",
		NameNorm:  "旧名",
		ValidFrom: "2020-01-01",
		Source:    SourceFile,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d", count)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func securityRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"market",
		"symbol",
		"exchange",
		"current_name",
		"current_name_norm",
		"board",
		"status",
		"listing_date",
		"delisting_date",
		"lot_size",
		"price_precision",
		"source",
		"manual_locked",
		"created_at",
		"updated_at",
	})
}

func securityRowsWithMatched(name string) *sqlmock.Rows {
	columns := []string{
		"market",
		"symbol",
		"exchange",
		"current_name",
		"current_name_norm",
		"board",
		"status",
		"listing_date",
		"delisting_date",
		"lot_size",
		"price_precision",
		"source",
		"manual_locked",
		"created_at",
		"updated_at",
		name,
	}
	return sqlmock.NewRows(columns)
}
