package securitymaster

import (
	"strings"
	"testing"
)

func TestBootstrapDDL(t *testing.T) {
	ddl, err := BootstrapDDL("infinity_market")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(ddl, "\n")
	for _, want := range []string{
		"CREATE DATABASE IF NOT EXISTS `infinity_market`",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.securities",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.security_name_history",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.security_aliases",
		"CREATE TABLE IF NOT EXISTS `infinity_market`.security_refresh_runs",
		"PRIMARY KEY (market, symbol)",
		"UNIQUE KEY uk_name_history_segment",
		"UNIQUE KEY uk_security_alias",
		"KEY idx_refresh_source_status",
		"DEFAULT CHARSET=utf8mb4",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("DDL missing %q\n%s", want, joined)
		}
	}
}

func TestBootstrapDDLRejectsInvalidDatabase(t *testing.T) {
	if _, err := BootstrapDDL("bad-name"); err == nil {
		t.Fatal("expected invalid identifier error")
	}
}
