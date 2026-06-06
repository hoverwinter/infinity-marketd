## Context

`marketd` already has local TDX binary parsing under `internal/tdx` and CLI commands under `internal/cli`, but all current market data paths are historical-file imports. Realtime A-share snapshots require a separate TDX standard行情 TCP protocol path, compatible with pytdx `hq.get_security_quotes`, and should not affect existing ClickHouse fact-table contracts.

TDX standard HQ servers use a short setup handshake, request packets with one or more `(market, symbol)` pairs, a 16-byte response header, optional zlib-compressed response bodies, and field-specific binary encodings. The realtime quote payload encodes many price fields as signed variable-length deltas from the current price in integer cents.

## Goals / Non-Goals

**Goals:**
- Fetch realtime A-share quote snapshots for one or more symbols from a TDX standard HQ server.
- Support Shanghai and Shenzhen market codes using the repository's `sh` / `sz` naming.
- Decode current price, previous close, open, high, low, volume, amount, server time, and five bid/ask levels.
- Expose the capability through a CLI command that emits deterministic JSON.
- Cover request construction, response decoding, validation, and CLI behavior with tests.

**Non-Goals:**
- Persist realtime snapshots into ClickHouse fact tables.
- Implement extended行情 (`exhq`) futures/options/港股 quote retrieval in this change.
- Implement realtime streaming subscriptions; this change performs on-demand snapshot requests.
- Add trading or order-entry capabilities.

## Decisions

- Place the implementation in `internal/tdx`.
  - Rationale: realtime quote decoding is part of the TDX protocol surface and should live next to existing TDX parsers.
  - Alternative considered: a new `internal/realtime` package. That separates query use cases, but it would split protocol helpers away from existing TDX market conventions.

- Add a `quote` CLI command using standard `flag` parsing.
  - Rationale: current CLI uses direct command routing and `flag.FlagSet`; preserving that style keeps the change small.
  - Alternative considered: adding an HTTP API. That is useful later, but it requires serving lifecycle, auth, and operational decisions outside this change.

- Accept symbols as `--symbol` repeated or comma-separated values, with optional `market:code` prefixes.
  - Rationale: scripts can call `marketd quote --symbol 600519 --symbol sz:000001` without extra files.
  - Alternative considered: only infer market from code. Explicit prefixes are needed for ambiguous instruments and tests.

- Use an explicit `--server host:port` flag with a default standard HQ server.
  - Rationale: this avoids broad config churn while allowing operators to choose a reachable server.
  - Alternative considered: add config/env support immediately. It can be added later without changing quote semantics.

- Decode and expose unknown fields only when they are needed for stable quote behavior.
  - Rationale: pytdx returns several `reversed_bytes*` fields with uncertain semantics. The daemon should expose known, useful market fields first.
  - Alternative considered: mirror every pytdx field. That would make the public shape depend on poorly understood fields.

## Risks / Trade-offs

- TDX public servers can be unreachable or throttle requests -> the CLI returns a clear connection/query error and allows `--server` override.
- Protocol fields may vary for non-stock instruments -> this change validates six-digit A-share symbols and only supports `sh` / `sz`.
- Server time encoding is inferred from pytdx behavior -> tests cover the formatter, and raw response parsing remains isolated for later correction.
- Quote volume unit conventions can be source-specific -> the response preserves numeric values from the protocol without cross-row derived metrics.
