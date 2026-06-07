## Why

`marketd` can read TDX xdxr corporate-action data online, but persisted OHLCV queries only return unadjusted bars. This leaves long-horizon returns, technical indicators, and backtests exposed to mechanical price jumps from dividends, bonus shares, rights issues, and splits.

## What Changes

- Persist TDX xdxr corporate-action events as normalized market facts.
- Add a rebuildable daily adjustment factor table derived from raw daily bars and xdxr events.
- Support `adjust=none|qfq|hfq` on `/api/v1/bars`.
- Return adjusted OHLC values by joining raw bars to daily factors at query time.
- Keep canonical OHLCV fact tables unadjusted.
- Do not materialize full adjusted daily, 1-minute, or 5-minute K-line tables in this change.
- Do not add a runtime dependency on Python, pandas, mootdx, or external factor providers.

## Capabilities

### New Capabilities

- `marketd-adjusted-bars`: TDX xdxr persistence, daily adjustment factor generation, and adjusted bar query semantics.

### Modified Capabilities

- `marketd-clickhouse-data-plane`: Bootstrap creates the corporate-action and adjustment factor tables required by adjusted bar queries.
- `marketd-tdx-local-import`: Canonical local OHLCV imports remain raw-only and do not implicitly refresh adjustment factors.

## Impact

- New ClickHouse market tables for xdxr events and daily adjustment factors.
- New `marketd` refresh/import commands for xdxr and adjustment factors.
- `/api/v1/bars` gains an optional `adjust` query parameter and includes that normalized value in the response query echo.
- `internal/querier` and `internal/clickhouse/query.go` must join adjustment factors only for adjusted queries.
- Documentation updates for storage schema, API semantics, and operator refresh workflow.
- Tests for xdxr event normalization, factor calculations, date-range rebuilds, adjusted daily bars, adjusted minute bars, and raw-query compatibility.
