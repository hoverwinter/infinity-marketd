# cmd/marketd/

Entry point for the **ingest (write plane)** binary. `main.go` only calls `internal/cli.Run(ctx, args, stdout, stderr)` and exits with its status code.

Commands (routed in `internal/cli`): `bootstrap`, `status`, `import-tdx-day`, `import-tdx-1m`, `import-tdx-5m`.

Add or change command behavior in `internal/cli`, not here.
