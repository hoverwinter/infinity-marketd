# cmd/infinity/

Entry point for the **querier (read plane)** binary. `main.go` only calls `internal/infinitycli.Run(ctx, args, stdout, stderr)` and exits with its status code.

Commands (routed in `internal/infinitycli`): `querier serve | health | bars | resolve-symbol`.

Add or change command behavior in `internal/infinitycli`, not here.
