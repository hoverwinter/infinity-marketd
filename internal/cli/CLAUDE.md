# internal/cli/

Command routing for the **marketd** binary. `Run(ctx, args, stdout, stderr) int` dispatches subcommands, parses flags via `config.RegisterCommonFlags`, and orchestrates the actual work through `internal/ingest` and `internal/clickhouse`.

Subcommands: `bootstrap` (create schema), `status` (watermarks/health), `import-tdx-day` / `import-tdx-1m` / `import-tdx-5m` (with `--root`+`--code`, `--file`, and `--dry-run`), `import-tdx-vipdoc-zip`, `import-tdx-hq-day` (explicit online HQ daily import), `import-tdx-g4-day` (validated post-close market-wide daily import), `quote`, `quote-probe`, `quote-sweep`, `exquote-markets`, `exquote-count`, `exquote-instruments`, `exquote`, `exquote-bars`, `exquote-minute`, `exquote-history-minute`, `exquote-transactions`, `exquote-history-transactions`, and `exquote-history-bars`.

Keep this layer thin: flag parsing and wiring only; parsing logic belongs in `tdx`, write logic in `ingest`/`clickhouse`.
