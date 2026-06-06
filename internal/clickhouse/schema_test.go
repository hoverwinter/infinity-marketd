package clickhouse

import (
	"strings"
	"testing"
	"time"
)

func TestBootstrapDDL(t *testing.T) {
	ddl, err := BootstrapDDL(SchemaConfig{MarketDB: "infinity_market", OpsDB: "infinity_ops"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(ddl, "\n")
	for _, want := range []string{
		"CREATE DATABASE IF NOT EXISTS `infinity_market`",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.a_share_bars_1d",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.a_share_bars_1m",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.a_share_bars_5m",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.a_share_daily_derived",
		"CREATE TABLE IF NOT EXISTS `infinity_ops`.watermarks",
		"ENGINE = ReplacingMergeTree",
		"PARTITION BY toYear(trade_date)",
		"PARTITION BY toYYYYMM(trade_date)",
		"ORDER BY (market, symbol, trade_date)",
		"ORDER BY (market, symbol, bar_time)",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("DDL missing %q\n%s", want, joined)
		}
	}
	marketDDL := ddl[2] + ddl[3] + ddl[4]
	for _, forbidden := range []string{"source_key", "source_file", "version UInt64", "updated_at", "pct_chg"} {
		if strings.Contains(marketDDL, forbidden) {
			t.Fatalf("market DDL contains %q\n%s", forbidden, marketDDL)
		}
	}
}

func TestBootstrapDDLRejectsInvalidIdentifier(t *testing.T) {
	if _, err := BootstrapDDL(SchemaConfig{MarketDB: "bad-name", OpsDB: "ops"}); err == nil {
		t.Fatal("expected invalid identifier error")
	}
}

func TestPartitionKeys(t *testing.T) {
	got := dailyPartitionKey(time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC))
	if got != "2026" {
		t.Fatalf("daily partition key = %q", got)
	}
	got = minutePartitionKey(time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC))
	if got != "202606" {
		t.Fatalf("minute partition key = %q", got)
	}
}
