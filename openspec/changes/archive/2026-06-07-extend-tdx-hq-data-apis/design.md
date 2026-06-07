## Context

`marketd` currently has a TDX standard行情 (`hq`) client for A-share realtime quote snapshots, server probing, retry, reusable quote sessions, and `sh` / `sz` security count/list discovery. Recent work also added Beijing quote support after verifying market byte `2`, while Beijing security-list discovery remains unavailable on tested public servers.

The broader pytdx-compatible standard行情 API surface is still not implemented in Go. This leaves operators dependent on Python tools for online K-line checks, minute-time checks, transaction checks, company/F10 content, xdxr, finance info, and block membership workflows.

This change keeps the capability read-only. It does not introduce ClickHouse persistence, storage schemas, or ingestion jobs.

## Goals / Non-Goals

**Goals:**
- Implement documented TDX standard行情 read APIs as Go-native protocol calls.
- Keep `hq` separate from `exhq` and preserve `sh` / `sz` / `bj` market names where the standard行情 protocol supports them.
- Reuse the existing HQ server selection, session lifecycle, zlib body handling, timeout, retry, and JSON CLI patterns.
- Provide explicit CLI commands for each API with deterministic JSON output.
- Enforce protocol bounds before connecting to a server, including K-line `count <= 800`.
- Treat empty historical responses as valid outcomes when the requested date or server has no data.
- Decode text fields with GB18030/GBK fallback where the protocol returns encoded names or content.

**Non-Goals:**
- No realtime quote, K-line, minute-time, transaction, F10, xdxr, finance, or block persistence.
- No ClickHouse schema changes.
- No trading or authenticated private broker API.
- No Level-2 order book, order queue, or paid entitlement feed implementation.
- No runtime dependency on pytdx, mootdx, pandas, or Python.

## Decisions

- Use one standard行情 capability for all `hq` read APIs.
  - Rationale: quote, security list, K-line, minute-time, transactions, F10, xdxr, finance, and block APIs share the same standard HQ server class, setup packets, compression handling, server fallback, and A-share market naming.
  - Alternative considered: separate one change per API family. That would reduce each implementation slice but obscure shared protocol and CLI conventions.

- Keep all commands read-only and independent of ClickHouse config.
  - Rationale: these are online TDX reads, not canonical fact ingestion. Operators can inspect and validate responses without affecting storage.
  - Alternative considered: write remote K-line results directly into `a_share_bars_*`. That requires separate ingestion/watermark design and is not needed to expose the protocol surface.

- Preserve existing `quote`, `quote-probe`, and security-list code, and add missing APIs beside them.
  - Rationale: existing behavior is already tested and operator-visible. The change should expand the surface, not rename or break current commands.
  - Alternative considered: replace existing quote paths with a new generic request abstraction. That would risk regressions without clear benefit.

- Expose protocol pagination directly.
  - Rationale: TDX APIs use different paging models. K-lines use `start/count` with `count <= 800`; transaction APIs use `start/count`; historical minute-time is per date and normally returns a full trading day of points.
  - Alternative considered: hide paging behind date ranges for every API. Date-range workflows are useful later, but first implementation should preserve raw protocol behavior for validation.

- Keep K-line data and minute-time data distinct.
  - Rationale: K-lines are OHLCV bars; minute-time data is `price + volume` points and does not contain full OHLC.
  - Alternative considered: normalize minute-time into 1-minute bars. That would be semantically wrong and conflict with existing table rules.

- Decode text with GB18030/GBK fallback.
  - Rationale: TDX names, F10 text, block names, and company info fields are commonly not UTF-8.
  - Alternative considered: leave raw bytes or empty strings. That makes the APIs less useful for operators and docs validation.

- Treat public server behavior as variable.
  - Rationale: TDX public servers differ by network, time, throttling, and capability exposure. A command must surface transport/protocol errors clearly and allow explicit `--server`.
  - Alternative considered: assume one default server supports all APIs. That is brittle and has already proven false for parts of `hq` and `exhq`.

## Risks / Trade-offs

- Public servers may return empty or inconsistent data -> commands surface empty arrays as valid data responses and reserve errors for transport, validation, and decode failures.
- Some APIs may vary by `sh` / `sz` / `bj` support -> each API validates market support independently instead of assuming quote support implies list, K-line, or history support.
- K-line category values are protocol-specific -> CLI accepts numeric categories and documents aliases rather than inventing a new hidden mapping.
- F10/company payloads may be large or encoded inconsistently -> commands decode best-effort GB18030/GBK text and keep request size/window arguments explicit.
- Implementing all APIs is broad -> tasks are split by API family with packet, decoder, client, CLI, tests, and docs for each family.

## Migration Plan

No database migration is required. Existing quote and security-list commands remain available. New commands are additive and can be tested independently against local scripted servers before live server smoke tests.

## Open Questions

- Which standard行情 server list should be preferred for F10/company and block APIs if the realtime quote defaults do not answer those packets reliably?
- Should a later change add date-range wrappers for standard行情 K-lines and historical minute-time after the raw protocol commands are stable?
- Should remote K-line import into canonical ClickHouse fact tables be a separate ingest/watermark change after this read-only API surface is complete?
