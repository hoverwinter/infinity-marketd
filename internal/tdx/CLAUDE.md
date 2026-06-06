# internal/tdx/

TongDaXin (TDX) binary parsing — the source-of-truth decoders.

- `parse.go` — `ParseDayBytes` / `ParseMinuteBytes` over 32-byte records. `.day` prices are integer cents (÷100); `.lc1`/`.lc5` use float32; `.1`/`.5` use integer cents. Returns `Daily/MinuteParseResult` plus `ParseIssue`s for malformed/duplicate rows.
- `market.go` — `InferMarketFromCode`, `ParseMarketSymbol` (market from `vipdoc/sh|sz|bj/...` path or code prefix).
- `discovery.go` — `DiscoverFile` / `DiscoverFiles` by `Period`, market, code.

All timestamps use `Asia/Shanghai`. Format reference: `docs/tdx-data/`. Pure parsing — no ClickHouse or I/O orchestration here.
