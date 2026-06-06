package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEnvAndOverridePrecedence(t *testing.T) {
	t.Setenv("MARKETD_CLICKHOUSE_ADDR", "env:9000")
	t.Setenv("MARKETD_CLICKHOUSE_MARKET_DB", "env_market")

	cfg, err := Load(Overrides{ClickHouseAddr: "flag:9000"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClickHouse.Addr != "flag:9000" {
		t.Fatalf("addr = %q", cfg.ClickHouse.Addr)
	}
	if cfg.ClickHouse.Databases.Market != "env_market" {
		t.Fatalf("market db = %q", cfg.ClickHouse.Databases.Market)
	}
}

func TestLoadConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := []byte(`clickhouse:
  addr: "192.168.28.210:9000"
  user: "marketd"
  password: "secret"
  databases:
    market: "infinity_market"
    ops: "infinity_ops"
tdx:
  root: "/tmp/tdx"
runtime:
  timezone: "Asia/Shanghai"
  batch_size: 5000
`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(Overrides{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClickHouse.Addr != "192.168.28.210:9000" {
		t.Fatalf("addr = %q", cfg.ClickHouse.Addr)
	}
	if cfg.ClickHouse.User != "marketd" {
		t.Fatalf("user = %q", cfg.ClickHouse.User)
	}
	if cfg.TDX.Root != "/tmp/tdx" {
		t.Fatalf("tdx root = %q", cfg.TDX.Root)
	}
	if cfg.Runtime.BatchSize != 5000 {
		t.Fatalf("batch size = %d", cfg.Runtime.BatchSize)
	}
}

func TestLoadClickHouseHostPortPasswdDatabaseConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := []byte(`clickhouse:
  user: "default"
  host: "192.168.28.210"
  port: 9000
  passwd: 123456
  database: "infinity"
`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(Overrides{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClickHouse.Addr != "192.168.28.210:9000" {
		t.Fatalf("addr = %q", cfg.ClickHouse.Addr)
	}
	if cfg.ClickHouse.Password != "123456" {
		t.Fatalf("password = %q", cfg.ClickHouse.Password)
	}
	if cfg.ClickHouse.Databases.Market != "infinity" {
		t.Fatalf("market db = %q", cfg.ClickHouse.Databases.Market)
	}
	if cfg.ClickHouse.Databases.Ops != "infinity_ops" {
		t.Fatalf("ops db = %q", cfg.ClickHouse.Databases.Ops)
	}
}

func TestLoadEmptyConfigFileFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("  \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(Overrides{ConfigPath: path})
	if err == nil {
		t.Fatal("expected empty config file error")
	}
	if !strings.Contains(err.Error(), "is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}
