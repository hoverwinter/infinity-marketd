## Context

`marketd` currently supports TDX standard行情 realtime snapshots for `sh` and `sz`. Beijing Stock Exchange symbols are recognized by local file import market inference, but realtime quote requests reject `bj` because the standard行情 market byte, security-list behavior, and quote response samples have not been verified.

The implementation must not guess the Beijing market mapping. The first implementation step is to capture or generate verified samples from a reachable TDX HQ server, then update request construction and response decoding only when the observed payload shape matches the existing standard quote parser.

## Goals / Non-Goals

**Goals:**
- Verify the TDX standard行情 market mapping for Beijing realtime quote requests.
- Verify that Beijing quote response payloads decode with the same standard `hq` quote parser.
- Support explicit `bj:<symbol>` quote requests after validation.
- Support inferred Beijing symbols, including `920*`, `8*`, and `4*` code families that `InferMarketFromCode` already maps to `bj`.
- Support explicit Beijing symbol lists in `quote-sweep`.
- Keep Beijing online security-list discovery disabled unless a future server path is verified.
- Document the verified mapping, sample commands, and remaining limitations.

**Non-Goals:**
- No `exhq` implementation.
- No ClickHouse realtime snapshot persistence.
- No trading functionality.
- No destructive schema or data changes.
- No fallback to pytdx or mootdx at runtime.

## Decisions

- Keep `bj` disabled until samples are verified.
  - Rationale: returning wrong realtime quotes is worse than a clear unsupported-market error.
  - Alternative considered: map `bj` to an assumed market byte immediately. This is rejected because it risks silent data corruption.

- Add `bj` to the existing standard `hq` path only if the response shape matches existing quote decoding.
  - Rationale: this preserves one A-share standard行情 client for `sh` / `sz` / `bj`.
  - Alternative considered: add a separate Beijing quote client. That is only justified if live samples show a different protocol or endpoint.

- Keep online Beijing security-list discovery disabled after verification.
  - Rationale: reachable HQ servers returned valid `920*` quotes with market byte `2`, but market byte `2` security count/list probes timed out and did not produce a usable discovery path.
  - Alternative considered: map `bj` discovery to `sh` or `sz`. This is rejected because it returns unrelated securities.

- Split requests containing `bj` into single-symbol batches for now.
  - Rationale: live single-symbol Beijing quote responses decode reliably, while live multi-symbol responses require a separate parser hardening pass before enabling batching.
  - Alternative considered: rely on existing multi-symbol parser. This is rejected for Beijing because returning a partial or misaligned quote is worse than a slower but correct request.

- Use captured/synthetic tests for parser behavior.
  - Rationale: live public TDX servers are unstable in CI and cannot be the only verification.
  - Alternative considered: only live smoke tests. That would make the capability hard to validate consistently.

## Risks / Trade-offs

- Beijing market may not be available through standard `hq` servers -> document the finding and update the design before implementation.
- Public TDX servers may return inconsistent lists or reject Beijing discovery -> keep `quote-sweep --market bj` unsupported until a usable list path is verified.
- Code-prefix inference may include instruments that are not valid realtime quote targets -> parser should continue to validate six-digit symbols and quote response identity.
- Security-list naming may need GBK decoding improvements -> keep name decoding separate from quote correctness.
- Adding explicit `bj` quote sweep can increase request volume because `bj` requests are single-symbol batches -> existing `--limit` remains required for operator smoke tests.

## Verification Notes

- Reachable HQ servers: `60.191.117.167:7709`, `180.153.18.170:7709`.
- Verified quote market byte: `2`.
- Successful samples: `bj:920001`, inferred `920001`, `920799`, `920682`, `920167`; responses returned `market=2` and matching symbols.
- Negative samples: `430047` and older candidate `830001` returned mismatched `sh:600839` fallback records on tested servers, so response identity validation rejects them.
- Security count/list with market byte `2` timed out on tested servers, so Beijing online discovery remains unsupported in this change.
