## Why

The full `hsjday.zip` package is appropriate for historical baselines but is unnecessarily large for routine post-close updates. The verified `g4day` broker package contains matching Shanghai, Shenzhen, and Beijing A-share OHLCV data for one trading day, so marketd should import it as an explicit finalized daily increment instead of deriving canonical daily bars from live quotes.

## What Changes

- Add `marketd import-tdx-g4-day` for one post-close trade date.
- Support downloading the official package with `--date`, or replaying an already downloaded package with `--file`.
- Decode aligned 150-byte `.cod` and 512-byte `.md1` records for Shanghai, Shenzhen, and Beijing.
- Reject incomplete, date-inconsistent, count-mismatched, duplicate-code, oversized, or malformed packages before writing any bars.
- Import only recognized A-share equity symbols with valid traded OHLCV values; count non-equities and no-trade rows as skipped.
- Reuse `RunOnlineJob`, `a_share_bars_1d`, `TaskRun`, quality issues, and the dataset watermark. Dry-run performs download/parse/validation but writes nothing.
- Keep live quote reads and intraday display paths read-only with respect to canonical daily facts.
- Do not add a scheduler, Console action, `g3day` support, schema, table, or dependency.

## Capabilities

### New Capabilities

- `marketd-tdx-g4-daily-import`: validated local or official-remote `g4day` post-close A-share daily import.

### Modified Capabilities

None.

## Impact

- `internal/tdx`: `g4day` ZIP parser, equity classification, validation, and bounded HTTP fetch.
- `internal/ingest`: package-to-canonical-bar adapter using the existing online job runner.
- `internal/cli`: one command and summary output.
- TDX format and command documentation plus synthetic and official-package verification.
- No changes to query APIs, canonical schemas, adjustment-factor behavior, or realtime quote persistence.
