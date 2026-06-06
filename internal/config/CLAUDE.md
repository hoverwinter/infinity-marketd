# internal/config/

Configuration loading and precedence.

- `Default()` — built-in defaults.
- `Load(Overrides) (Config, error)` — merges with precedence **CLI flags > env vars > config file > defaults**.
- `RegisterCommonFlags(fs, *Overrides)` — registers shared flags (`--config`, ClickHouse host/port/etc.) on a `flag.FlagSet`.

Config types: `Config`, `ClickHouseConfig`, `DatabaseConfig`, `TDXConfig`, `RuntimeConfig`. Runtime time handling assumes `Asia/Shanghai`. Sample/template files live in `configs/` and `examples/`.
