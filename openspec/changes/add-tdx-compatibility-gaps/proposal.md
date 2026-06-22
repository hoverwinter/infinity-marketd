## Why

`marketd` now covers the core TDX online/data-plane surface, but several compatibility gaps remain against `millken/tdx` and `mootdx`: compact batch quote rows, tick-chart reads, SP/fund server discovery, fully labeled fund detail values, and one-shot adjusted online bars. These gaps matter for scanner, console, and research workflows that need richer live fields or a quick analyst path without turning provider reads into hidden persistence.

## What Changes

- Add standard HQ compact batch quote support equivalent to `millken/tdx.GetBatchQuotes` / `0x054C`, returning the compact live quote fields that are not exposed by the existing realtime quote snapshot.
- Add standard HQ tick-chart support equivalent to `millken/tdx.GetTickChart` / `0x0537`, distinct from existing minute-time point reads.
- Add SP and fund server address catalogs plus probe/best-server helpers for SP board-member and fund 7727 reads, while still allowing explicit server overrides.
- Decode fund detail rows with confirmed labels and typed values where the fund 7727 item semantics are known, preserving raw rows for unknown ids.
- Add an explicit one-shot online adjusted bars convenience path that fetches raw HQ bars plus live XDXR and returns `none/qfq/hfq` adjusted OHLC without writing ClickHouse.
- Keep `/api/v1/bars` unchanged: stable adjusted queries remain backed by persisted raw bars and persisted adjustment factors.
- Keep all new online compatibility reads non-persistent by default.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `marketd-tdx-advanced-online-apis`: add compact batch quotes, tick chart, SP/fund server discovery, labeled fund detail, and one-shot online adjusted bar provider APIs.
- `marketd-adjusted-bars`: clarify that persisted `/api/v1/bars?adjust=...` remains ClickHouse-backed while the new one-shot online adjusted bars path is a separate live provider capability.

## Impact

- `internal/tdx`: add packet builders/decoders for `0x054C` compact batch quotes and `0x0537` tick chart, SP/fund address/probe helpers, fund detail label mapping, and online-adjust helper logic.
- `internal/cli`: add commands for compact batch quotes, tick chart, SP/fund bestip/address listing, labeled fund detail output, and online adjusted bars.
- `internal/querier`: expose matching `/api/tdx/*` provider endpoints without mixing them into `/api/v1`.
- `docs/api/tdx.md`, `docs/reference/tdx-python-libraries.md`, and `docs/reference/tdx-advanced-protocol-notes.md`: document the new contracts, limits, and live-validation status.
- No ClickHouse schema changes. No hidden writes to market fact, derived, or ops tables.
