## Context

`marketd` currently supports TDX standard行情 realtime snapshots for `sh` and `sz`. Beijing Stock Exchange symbols are recognized by local file import market inference, but realtime quote requests reject `bj` because the standard行情 market byte, security-list behavior, and quote response samples have not been verified.

The implementation must not guess the Beijing market mapping. The first implementation step is to capture or generate verified samples from a reachable TDX HQ server, then update request construction and response decoding only when the observed payload shape matches the existing standard quote parser.

## Goals / Non-Goals

**Goals:**
- Verify the TDX standard行情 market mapping for Beijing realtime quote requests.
- Verify that Beijing quote response payloads decode with the same standard `hq` quote parser.
- Support explicit `bj:<symbol>` quote requests after validation.
- Support inferred Beijing symbols, including `920*`, `8*`, and `4*` code families that `InferMarketFromCode` already maps to `bj`.
- Include `bj` in online security-list discovery and `quote-sweep` when the standard行情 server supports it.
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

- Treat online security-list discovery as part of acceptance.
  - Rationale: quote sweep needs market symbol discovery, not only explicit one-off symbols.
  - Alternative considered: support only explicit `bj` symbols first. This is a useful fallback but incomplete for operational workflows.

- Use captured/synthetic tests for parser behavior.
  - Rationale: live public TDX servers are unstable in CI and cannot be the only verification.
  - Alternative considered: only live smoke tests. That would make the capability hard to validate consistently.

## Risks / Trade-offs

- Beijing market may not be available through standard `hq` servers -> document the finding and update the design before implementation.
- Public TDX servers may return inconsistent lists or reject Beijing requests -> require at least one successful live sample and fixture-based tests.
- Code-prefix inference may include instruments that are not valid realtime quote targets -> parser should continue to validate six-digit symbols and quote response identity.
- Security-list naming may need GBK decoding improvements -> keep name decoding separate from quote correctness.
- Adding `bj` to quote sweep can increase request volume -> existing `--limit` and `--batch-size` controls remain required.
