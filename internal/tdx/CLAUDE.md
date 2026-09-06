# internal/tdx/

TongDaXin (TDX) binary parsing — the source-of-truth decoders.

- `parse.go` — `ParseDayBytes` / `ParseMinuteBytes` over 32-byte records. `.day` prices are integer cents (÷100); `.lc1`/`.lc5` use float32; `.1`/`.5` use integer cents. Returns `Daily/MinuteParseResult` plus `ParseIssue`s for malformed/duplicate rows.
- `market.go` — `InferMarketFromCode`, `ParseMarketSymbol` (market from `vipdoc/sh|sz|bj/...` path or code prefix).
- `discovery.go` — `DiscoverFile` / `DiscoverFiles` by `Period`, market, code.
- `quote.go` / `quote_ops.go` — TDX standard `hq` realtime quote packets, decoders, server probing, batch retry, security list, and quote sweep.
- `exquote.go` / `exquote_data.go` — TDX extended `exhq` market list, instrument catalog, quote, K-line, minute-time, transaction, and history packets/decoders.

All timestamps use `Asia/Shanghai`. Format reference: `docs/tdx-data/`. This package owns TDX parsing and TDX network protocol clients; it must not open ClickHouse or orchestrate imports.

`marketdata.go` adapts existing HQ security/index bars to the optional `marketdata.BarsProvider` contract. It owns bounded backwards pagination and retains native volume units; wire decoders and existing protocol APIs do not change.
