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
	Logging    LoggingConfig    `yaml:"logging"`
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

type LoggingConfig struct {
	Level            string            `yaml:"level"`
	Encoding         string            `yaml:"encoding"`
	OutputPaths      []string          `yaml:"output_paths"`
	ErrorOutputPaths []string          `yaml:"error_output_paths"`
	File             LoggingFileConfig `yaml:"file"`
}

type LoggingFileConfig struct {
	Path       string `yaml:"path"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAgeDays int    `yaml:"max_age_days"`
	Compress   bool   `yaml:"compress"`
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
	LogLevel           string
	LogEncoding        string
	LogOutputPaths     string
	LogErrorPaths      string
	LogFilePath        string
	LogFileMaxSizeMB   int
	LogFileMaxBackups  int
	LogFileMaxAgeDays  int
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
		Logging: LoggingConfig{
			Level:            "info",
			Encoding:         "console",
			OutputPaths:      []string{"stderr"},
			ErrorOutputPaths: []string{"stderr"},
			File: LoggingFileConfig{
				MaxSizeMB:  100,
				MaxBackups: 7,
				MaxAgeDays: 30,
				Compress:   true,
			},
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
	if err := validateLoggingConfig(cfg.Logging); err != nil {
		return cfg, err
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
	fs.StringVar(&overrides.LogLevel, "log-level", "", "log level: debug, info, warn, error, dpanic, panic, or fatal")
	fs.StringVar(&overrides.LogEncoding, "log-encoding", "", "log encoding: console or json")
	fs.StringVar(&overrides.LogOutputPaths, "log-output-paths", "", "comma-separated log outputs: stdout, stderr, or file")
	fs.StringVar(&overrides.LogErrorPaths, "log-error-output-paths", "", "comma-separated zap internal error outputs")
	fs.StringVar(&overrides.LogFilePath, "log-file", "", "rotating log file path for file output")
	fs.IntVar(&overrides.LogFileMaxSizeMB, "log-file-max-size-mb", 0, "rotating log file max size in MB")
	fs.IntVar(&overrides.LogFileMaxBackups, "log-file-max-backups", 0, "rotating log file max backup count")
	fs.IntVar(&overrides.LogFileMaxAgeDays, "log-file-max-age-days", 0, "rotating log file max age in days")
}

func applyEnv(cfg *Config) {
	setString(&cfg.ClickHouse.Addr, "MARKETD_CLICKHOUSE_ADDR")
	setString(&cfg.ClickHouse.Databases.Market, "MARKETD_CLICKHOUSE_MARKET_DB")
	setString(&cfg.ClickHouse.Databases.Ops, "MARKETD_CLICKHOUSE_OPS_DB")
	setString(&cfg.ClickHouse.User, "MARKETD_CLICKHOUSE_USER")
	setString(&cfg.ClickHouse.Password, "MARKETD_CLICKHOUSE_PASSWORD")
	setString(&cfg.TDX.Root, "MARKETD_TDX_ROOT")
	setString(&cfg.Runtime.Timezone, "MARKETD_TIMEZONE")
	setString(&cfg.Logging.Level, "MARKETD_LOG_LEVEL")
	setString(&cfg.Logging.Encoding, "MARKETD_LOG_ENCODING")
	setString(&cfg.Logging.File.Path, "MARKETD_LOG_FILE")
	setStringList(&cfg.Logging.OutputPaths, "MARKETD_LOG_OUTPUT_PATHS")
	setStringList(&cfg.Logging.ErrorOutputPaths, "MARKETD_LOG_ERROR_OUTPUT_PATHS")
	if value := os.Getenv("MARKETD_BATCH_SIZE"); value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			cfg.Runtime.BatchSize = n
		}
	}
	if value := os.Getenv("MARKETD_LOG_FILE_MAX_SIZE_MB"); value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			cfg.Logging.File.MaxSizeMB = n
		}
	}
	if value := os.Getenv("MARKETD_LOG_FILE_MAX_BACKUPS"); value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			cfg.Logging.File.MaxBackups = n
		}
	}
	if value := os.Getenv("MARKETD_LOG_FILE_MAX_AGE_DAYS"); value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			cfg.Logging.File.MaxAgeDays = n
		}
	}
	if value := os.Getenv("MARKETD_LOG_FILE_COMPRESS"); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			cfg.Logging.File.Compress = parsed
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
	if overrides.LogLevel != "" {
		cfg.Logging.Level = overrides.LogLevel
	}
	if overrides.LogEncoding != "" {
		cfg.Logging.Encoding = overrides.LogEncoding
	}
	if overrides.LogOutputPaths != "" {
		cfg.Logging.OutputPaths = splitCSV(overrides.LogOutputPaths)
	}
	if overrides.LogErrorPaths != "" {
		cfg.Logging.ErrorOutputPaths = splitCSV(overrides.LogErrorPaths)
	}
	if overrides.LogFilePath != "" {
		cfg.Logging.File.Path = overrides.LogFilePath
	}
	if overrides.LogFileMaxSizeMB > 0 {
		cfg.Logging.File.MaxSizeMB = overrides.LogFileMaxSizeMB
	}
	if overrides.LogFileMaxBackups > 0 {
		cfg.Logging.File.MaxBackups = overrides.LogFileMaxBackups
	}
	if overrides.LogFileMaxAgeDays > 0 {
		cfg.Logging.File.MaxAgeDays = overrides.LogFileMaxAgeDays
	}
}

func setString(target *string, env string) {
	if value := os.Getenv(env); value != "" {
		*target = value
	}
}

func setStringList(target *[]string, env string) {
	if value := os.Getenv(env); value != "" {
		*target = splitCSV(value)
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func validateLoggingConfig(cfg LoggingConfig) error {
	switch strings.ToLower(cfg.Level) {
	case "debug", "info", "warn", "error", "dpanic", "panic", "fatal":
	default:
		return fmt.Errorf("logging.level must be one of debug, info, warn, error, dpanic, panic, or fatal")
	}
	switch strings.ToLower(cfg.Encoding) {
	case "console", "json":
	default:
		return fmt.Errorf("logging.encoding must be console or json")
	}
	if len(cfg.OutputPaths) == 0 {
		return fmt.Errorf("logging.output_paths must not be empty")
	}
	if len(cfg.ErrorOutputPaths) == 0 {
		return fmt.Errorf("logging.error_output_paths must not be empty")
	}
	if err := validateLogOutputPaths("logging.output_paths", cfg.OutputPaths, cfg.File.Path); err != nil {
		return err
	}
	if err := validateLogOutputPaths("logging.error_output_paths", cfg.ErrorOutputPaths, cfg.File.Path); err != nil {
		return err
	}
	if cfg.File.MaxSizeMB <= 0 {
		return fmt.Errorf("logging.file.max_size_mb must be positive")
	}
	if cfg.File.MaxBackups < 0 {
		return fmt.Errorf("logging.file.max_backups must be non-negative")
	}
	if cfg.File.MaxAgeDays < 0 {
		return fmt.Errorf("logging.file.max_age_days must be non-negative")
	}
	return nil
}

func validateLogOutputPaths(field string, paths []string, filePath string) error {
	for _, path := range paths {
		switch strings.ToLower(path) {
		case "stdout", "stderr":
		case "file":
			if strings.TrimSpace(filePath) == "" {
				return fmt.Errorf("%s includes file but logging.file.path is empty", field)
			}
		default:
			return fmt.Errorf("%s contains unsupported output %q", field, path)
		}
	}
	return nil
}
