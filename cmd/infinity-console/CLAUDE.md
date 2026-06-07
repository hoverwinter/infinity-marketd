# cmd/infinity-console/

Entry point for the standalone Infinity Console binary. `main.go` only calls `internal/consolecli.Run(ctx, args, stdout, stderr)` and exits with its status code.

Change console serving behavior in `internal/consolecli` or `internal/querier`, not here.
