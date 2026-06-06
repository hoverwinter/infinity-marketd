package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ClickHouse ClickHouseConfig `yaml:"clickhouse"`
	TDX        TDXConfig        `yaml:"tdx"`
	Runtime    RuntimeConfig    `yaml:"runtime"`
}

type ClickHouseConfig struct {
	Addr      string         `yaml:"addr"`
	Host      string         `yaml:"host"`
	Port      int            `yaml:"port"`
	User      string         `yaml:"user"`
	Password  string         `yaml:"password"`
	Passwd    scalarString   `yaml:"passwd"`
	Database  string         `yaml:"database"`
	Databases DatabaseConfig `yaml:"databases"`
}

type DatabaseConfig struct {
	Market string `yaml:"market"`
	Ops    string `yaml:"ops"`
}

type TDXConfig struct {
	Root string `yaml:"root"`
}

type RuntimeConfig struct {
	Timezone  string `yaml:"timezone"`
	BatchSize int    `yaml:"batch_size"`
}

type Overrides struct {
	ConfigPath         string
	ClickHouseAddr     string
	ClickHouseMarketDB string
	ClickHouseOpsDB    string
	ClickHouseUser     string
	ClickHousePassword string
	TDXRoot            string
	BatchSize          int
	Timezone           string
}

func Default() Config {
	return Config{
		ClickHouse: ClickHouseConfig{
			Addr:     "127.0.0.1:9000",
			User:     "default",
			Password: "",
			Databases: DatabaseConfig{
				Market: "infinity_market",
				Ops:    "infinity_ops",
			},
		},
		Runtime: RuntimeConfig{
			Timezone:  "Asia/Shanghai",
			BatchSize: 10000,
		},
	}
}

func Load(overrides Overrides) (Config, error) {
	cfg := Default()
	if overrides.ConfigPath != "" {
		raw, err := os.ReadFile(overrides.ConfigPath)
		if err != nil {
			return cfg, fmt.Errorf("read config: %w", err)
		}
		if strings.TrimSpace(string(raw)) == "" {
			return cfg, fmt.Errorf("config file %s is empty", overrides.ConfigPath)
		}
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return cfg, fmt.Errorf("parse config: %w", err)
		}
	}
	normalizeClickHouseConfig(&cfg.ClickHouse)
	applyEnv(&cfg)
	applyOverrides(&cfg, overrides)
	if cfg.Runtime.BatchSize <= 0 {
		return cfg, fmt.Errorf("runtime.batch_size must be positive")
	}
	if cfg.ClickHouse.Databases.Market == "" || cfg.ClickHouse.Databases.Ops == "" {
		return cfg, fmt.Errorf("clickhouse database names are required")
	}
	return cfg, nil
}

type scalarString string

func (s *scalarString) UnmarshalYAML(value *yaml.Node) error {
	*s = scalarString(value.Value)
	return nil
}

func normalizeClickHouseConfig(cfg *ClickHouseConfig) {
	if cfg.Host != "" {
		port := cfg.Port
		if port == 0 {
			port = 9000
		}
		cfg.Addr = fmt.Sprintf("%s:%d", cfg.Host, port)
	}
	if cfg.Password == "" && cfg.Passwd != "" {
		cfg.Password = string(cfg.Passwd)
	}
	if cfg.Database != "" {
		cfg.Databases.Market = cfg.Database
	}
}

func RegisterCommonFlags(fs *flag.FlagSet, overrides *Overrides) {
	fs.StringVar(&overrides.ConfigPath, "config", "", "config file path")
	fs.StringVar(&overrides.ClickHouseAddr, "clickhouse-addr", "", "ClickHouse native address host:port")
	fs.StringVar(&overrides.ClickHouseAddr, "clickhouse-url", "", "alias for --clickhouse-addr")
	fs.StringVar(&overrides.ClickHouseMarketDB, "clickhouse-market-db", "", "ClickHouse market database")
	fs.StringVar(&overrides.ClickHouseOpsDB, "clickhouse-ops-db", "", "ClickHouse ops database")
	fs.StringVar(&overrides.ClickHouseUser, "clickhouse-user", "", "ClickHouse user")
	fs.StringVar(&overrides.ClickHousePassword, "clickhouse-password", "", "ClickHouse password")
	fs.StringVar(&overrides.TDXRoot, "root", "", "TDX root path")
	fs.StringVar(&overrides.Timezone, "timezone", "", "runtime timezone")
	fs.IntVar(&overrides.BatchSize, "batch-size", 0, "batch insert size")
}

func applyEnv(cfg *Config) {
	setString(&cfg.ClickHouse.Addr, "MARKETD_CLICKHOUSE_ADDR")
	setString(&cfg.ClickHouse.Databases.Market, "MARKETD_CLICKHOUSE_MARKET_DB")
	setString(&cfg.ClickHouse.Databases.Ops, "MARKETD_CLICKHOUSE_OPS_DB")
	setString(&cfg.ClickHouse.User, "MARKETD_CLICKHOUSE_USER")
	setString(&cfg.ClickHouse.Password, "MARKETD_CLICKHOUSE_PASSWORD")
	setString(&cfg.TDX.Root, "MARKETD_TDX_ROOT")
	setString(&cfg.Runtime.Timezone, "MARKETD_TIMEZONE")
	if value := os.Getenv("MARKETD_BATCH_SIZE"); value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			cfg.Runtime.BatchSize = n
		}
	}
}

func applyOverrides(cfg *Config, overrides Overrides) {
	if overrides.ClickHouseAddr != "" {
		cfg.ClickHouse.Addr = overrides.ClickHouseAddr
	}
	if overrides.ClickHouseMarketDB != "" {
		cfg.ClickHouse.Databases.Market = overrides.ClickHouseMarketDB
	}
	if overrides.ClickHouseOpsDB != "" {
		cfg.ClickHouse.Databases.Ops = overrides.ClickHouseOpsDB
	}
	if overrides.ClickHouseUser != "" {
		cfg.ClickHouse.User = overrides.ClickHouseUser
	}
	if overrides.ClickHousePassword != "" {
		cfg.ClickHouse.Password = overrides.ClickHousePassword
	}
	if overrides.TDXRoot != "" {
		cfg.TDX.Root = overrides.TDXRoot
	}
	if overrides.Timezone != "" {
		cfg.Runtime.Timezone = overrides.Timezone
	}
	if overrides.BatchSize > 0 {
		cfg.Runtime.BatchSize = overrides.BatchSize
	}
}

func setString(target *string, env string) {
	if value := os.Getenv(env); value != "" {
		*target = value
	}
}
