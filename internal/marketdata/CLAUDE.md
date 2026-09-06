# internal/marketdata/

Online data product contracts only: optional `BarsProvider` / `BoardProvider`, source-scoped DTOs, validation, typed errors and startup registry. Do not import source packages, querier, ingest or storage here. No default provider or implicit fallback.

Adapters belong to their source package (`internal/ths`, `internal/tdx`, `internal/eastmoney`). Querier composes all three by default; imports may consume the same interfaces with explicit normalization and writes. Adding a provider must not require implementing unrelated capabilities. `Registry.WithProvider` replaces/adds one source immutably, preserving other instances.
