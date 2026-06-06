## Research Summary

Primary references:

- pytdx ExHQ docs list `get_markets`, `get_instrument_info`, `get_instrument_count`, `get_instrument_quote`, `get_minute_time_data`, `get_history_minute_time_data`, `get_instrument_bars`, `get_transaction_data`, and `get_history_transaction_data`.
- pytdx parser sources define fixed packet prefixes and binary response layouts for each ExHQ API.
- pytdx `TDXParams` documents K-line categories `0..11` and max K-line count `800`.
- gotdx publishes a broader public `ExtensionServerList` for ExHQ server candidates.

## Goals

- Implement the ExHQ read APIs as direct TDX protocol calls.
- Keep each CLI command read-only and independent of ClickHouse config.
- Preserve numeric `market` and string `code` identities exactly as ExHQ returns them.
- Expose pagination where pytdx exposes pagination: instrument list start/count and transaction start/count.
- Decode text names with GB18030 fallback for operational readability.

## Non-Goals

- No storage tables, watermarks, or ingestion jobs for ExHQ data in this change.
- No automated reconciliation of multiple ExHQ servers.
- No authenticated Level-2 protocol support.

## Protocol Notes

Public ExHQ servers currently behave inconsistently:

- Some public `7727` and broker `7721` endpoints answer `instrument count`.
- `47.102.108.214:7727` answered `instrument info` on 2026-06-07.
- Several endpoints accepted TCP but did not answer the pytdx setup packet.
- Some endpoints answer metadata but reset or timeout on quote/K-line/minute/transaction requests, possibly because of weekend maintenance, server capability gating, or public access limits.

Because of that, the Go session opens the TCP connection and sends the requested business packet directly. The pytdx setup packet is kept as a protocol reference but is not sent by default.

## CLI Contract

```bash
go run ./cmd/marketd exquote-count --server 112.74.214.43:7727
go run ./cmd/marketd exquote-instruments --start 0 --count 100 --server 47.102.108.214:7727
go run ./cmd/marketd exquote-bars --market 47 --code ICL0 --category 4 --start 0 --count 100 --server 47.102.108.214:7727
go run ./cmd/marketd exquote-minute --market 47 --code ICL0 --server 47.102.108.214:7727
go run ./cmd/marketd exquote-history-minute --market 47 --code ICL0 --date 20260605 --server 47.102.108.214:7727
go run ./cmd/marketd exquote-transactions --market 47 --code ICL0 --start 0 --count 1800 --server 47.102.108.214:7727
go run ./cmd/marketd exquote-history-transactions --market 47 --code ICL0 --date 20260605 --start 0 --count 1800 --server 47.102.108.214:7727
go run ./cmd/marketd exquote-history-bars --market 74 --code BABA --start-date 20260601 --end-date 20260605 --server 47.102.108.214:7727
```

All commands emit JSON and do not require config or ClickHouse.

## Risks / Trade-offs

- Public ExHQ server availability is not stable. The implementation must surface transport errors clearly and allow repeatable `--server`.
- Metadata availability does not prove quote/K-line availability on the same server.
- Historical APIs may return empty data for non-trading days or reset on servers that only expose catalog metadata.
- The command surface intentionally uses numeric K-line category values to match TDX/pytdx and avoid inventing another mapping.
