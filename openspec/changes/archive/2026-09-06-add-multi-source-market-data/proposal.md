## Why

The application needs online index data from TDX, THS and Eastmoney through one data-product contract. Consumers should select a source without implementing its pagination, field order or identifier mapping.

## What Changes

- Introduce small, optional capabilities for online bars and board discovery/resolution, with explicit provider-scoped instrument identities, periods, units and typed errors.
- Add a THS client for industry/concept board discovery, page-code to quotation-code resolution and annual index daily history, using AKShare as protocol reference.
- Adapt existing TDX security/index K-line clients to the same bars capability; preserve all existing TDX protocol routes.
- Expose provider discovery and data through `/api/providers/...` and thin `infinity querier` commands.
- Implement and register Eastmoney industry/concept catalogs, board resolution and historical daily index bars; validate its pagination, response identity and distinct OHLC field order.
- Document the delivered three-source capability matrix, source-specific units and coverage limits without changing storage.

## Capabilities

### New Capabilities

- `online-market-data-providers`: Provider-scoped online data contracts, TDX/THS/Eastmoney implementations, HTTP and CLI access.

### Modified Capabilities

None. Existing canonical queries, imports and raw TDX protocol contracts remain compatible.

## Impact

New `internal/marketdata` contracts and source clients in `internal/ths` and `internal/eastmoney`; a thin adapter in existing `internal/tdx`; querier composition and HTTP client/CLI additions. Existing `golang.org/x/text` handles THS GBK pages. No Python/JavaScript runtime, database schema, binary, scheduler or automatic cross-source fallback is introduced.
