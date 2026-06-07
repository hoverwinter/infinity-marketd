## Why

The initial TDX realtime quote implementation proves that `marketd` can fetch standard A-share snapshots without pytdx or mootdx, but it still depends on manually selected public HQ servers and only covers a narrow `sh` / `sz` snapshot path. Operators need a tracked roadmap for reliability, wider market coverage, batch operation, and storage decisions before realtime quotes become an operational data-plane feature.

## What Changes

- Add requirements for TDX HQ server probing, best server selection, and retry across server candidates.
- Add requirements for connection reuse or pooling for batch quote sweeps.
- Track support for online security lists so full-market quote jobs can discover symbols without local files.
- Track investigation and implementation boundaries for Beijing market realtime quotes and `exhq` extended-market quotes.
- Track the decision for whether realtime quote snapshots should be stored in a dedicated ClickHouse table.
- Track improved quote timestamp semantics, including trade date and `Asia/Shanghai` timezone handling.

## Capabilities

### New Capabilities
- `marketd-realtime-quote-operations`: Operational enhancements for TDX realtime quote reliability, market coverage, batch workflows, and storage decisions.

### Modified Capabilities

## Impact

- Future changes to `internal/tdx` quote clients, server selection, retry behavior, and symbol discovery.
- Future CLI additions for server probing, batch quote sweeps, and possibly quote snapshot import.
- Potential ClickHouse schema proposal for realtime quote snapshots if storage is accepted.
- No immediate breaking change to the current `marketd quote` command.
