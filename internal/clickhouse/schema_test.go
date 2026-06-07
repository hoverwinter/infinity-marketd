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
		"CREATE TABLE IF NOT EXISTS `infinity_market`.a_share_bars_1m_scan",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.a_share_bars_5m_scan",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.a_share_financial_raw_items",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.a_share_gp_metric_values",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.tdx_financial_item_dictionary",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.tdx_gp_metric_dictionary",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.a_share_intraday_points",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.a_share_daily_derived",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.a_share_xdxr_events",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.a_share_adjust_factors_1d",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.a_share_capital_change_events",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.tdx_block_snapshots",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.tdx_block_definitions",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.tdx_block_memberships",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.tdx_ex_bars_1d",
		"CREATE TABLE IF NOT EXISTS `infinity_ops`.watermarks",
		"point_time DateTime('Asia/Shanghai')",
		"point_index UInt16",
		"ENGINE = ReplacingMergeTree",
		"PARTITION BY toYear(trade_date)",
		"PARTITION BY toYear(event_date)",
		"PARTITION BY toYYYYMM(snapshot_time)",
		"PARTITION BY toYYYYMM(trade_date)",
		"ORDER BY (market, symbol, trade_date)",
		"ORDER BY (market, symbol, bar_time)",
		"ORDER BY (trade_date, bar_time, market, symbol)",
		"ORDER BY (market, symbol, report_date, item_id)",
		"ORDER BY (market, symbol, metric_type, event_date)",
		"ORDER BY (item_id)",
		"ORDER BY (metric_type)",
		"ORDER BY (market, symbol, trade_date, point_time)",
		"ORDER BY (market, symbol, event_date, category)",
		"ORDER BY (market, symbol, event_date, category, event_seq)",
		"ORDER BY (block_scope, snapshot_time, snapshot_id)",
		"ORDER BY (snapshot_id, block_scope, block_id)",
		"ORDER BY (snapshot_id, block_scope, block_id, market, symbol, member_order)",
		"ORDER BY (ex_market, code, trade_date)",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("DDL missing %q\n%s", want, joined)
		}
	}
	for _, forbiddenTable := range []string{"a_share_bars_1d_qfq", "a_share_bars_1d_hfq", "a_share_bars_1m_qfq", "a_share_bars_5m_hfq"} {
		if strings.Contains(joined, forbiddenTable) {
			t.Fatalf("DDL creates adjusted K-line table %q\n%s", forbiddenTable, joined)
		}
	}
	marketDDL := ddl[2] + ddl[3] + ddl[4]
	for _, forbidden := range []string{"source_key", "source_file", "version UInt64", "updated_at", "pct_chg", "fenhong", "peigu", "songzhuangu", "suogu"} {
		if strings.Contains(marketDDL, forbidden) {
			t.Fatalf("market DDL contains %q\n%s", forbidden, marketDDL)
		}
	}
	for _, table := range []string{"a_share_bars_1m_scan", "a_share_bars_5m_scan"} {
		tableDDL := ""
		for _, stmt := range ddl {
			if strings.Contains(stmt, table) {
				tableDDL = stmt
				break
			}
		}
		if tableDDL == "" {
			t.Fatalf("missing %s DDL", table)
		}
		for _, want := range []string{
			"trade_date Date",
			"bar_time DateTime('Asia/Shanghai')",
			"market LowCardinality(String)",
			"symbol String",
			"close Float64",
			"volume UInt64",
			"amount Float64",
			"prev_close Nullable(Float64)",
			"minute_ret Nullable(Float64)",
			"volume_ratio Nullable(Float64)",
			"computed_at DateTime64(3) DEFAULT now64(3)",
			"ENGINE = ReplacingMergeTree(computed_at)",
			"PARTITION BY toYYYYMM(trade_date)",
			"ORDER BY (trade_date, bar_time, market, symbol)",
			"TTL trade_date + INTERVAL 12 MONTH DELETE",
		} {
			if !strings.Contains(tableDDL, want) {
				t.Fatalf("%s DDL missing %q\n%s", table, want, tableDDL)
			}
		}
		for _, forbidden := range []string{"open ", "high ", "low ", "source_key", "source_file", "version UInt64", "updated_at"} {
			if strings.Contains(tableDDL, forbidden) {
				t.Fatalf("%s DDL contains %q\n%s", table, forbidden, tableDDL)
			}
		}
	}
	intradayDDL := ""
	for _, stmt := range ddl {
		if strings.Contains(stmt, "a_share_intraday_points") {
			intradayDDL = stmt
			break
		}
	}
	if intradayDDL == "" {
		t.Fatal("missing intraday DDL")
	}
	for _, forbidden := range []string{"open ", "high ", "low ", "close ", "amount", "period", "bar_interval", "source_key", "source_file", "version UInt64", "updated_at"} {
		if strings.Contains(intradayDDL, forbidden) {
			t.Fatalf("intraday DDL contains %q\n%s", forbidden, intradayDDL)
		}
	}
	for _, table := range []string{"a_share_capital_change_events", "tdx_ex_bars_1d", "a_share_financial_raw_items", "a_share_gp_metric_values"} {
		tableDDL := ""
		for _, stmt := range ddl {
			if strings.Contains(stmt, table) {
				tableDDL = stmt
				break
			}
		}
		if tableDDL == "" {
			t.Fatalf("missing %s DDL", table)
		}
		for _, forbidden := range []string{"source_key", "source_file", "version UInt64", "updated_at"} {
			if strings.Contains(tableDDL, forbidden) {
				t.Fatalf("%s DDL contains %q\n%s", table, forbidden, tableDDL)
			}
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
