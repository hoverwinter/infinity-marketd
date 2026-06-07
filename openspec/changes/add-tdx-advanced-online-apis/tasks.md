## 1. Protocol Reference And Fixtures

- [ ] 1.1 Capture `millken/tdx` packet, decoder, and model references for `GetQuotesList`, `GetTopBoard`, `GetBoardMembers`, `GetLHB`, `GetFundKline`, and `GetFundDetail` in a local implementation note.
- [ ] 1.2 Add deterministic binary fixtures or fake-server response bodies for quote-list decoding.
- [ ] 1.3 Add deterministic binary fixtures or fake-server response bodies for top-board decoding.
- [ ] 1.4 Add deterministic binary fixtures or fake-server response bodies for SP board-member decoding.
- [ ] 1.5 Add deterministic F10 text fixtures for LHB parser cases, including missing-section and malformed-row cases.
- [ ] 1.6 Add deterministic binary fixtures or fake-server response bodies for fund K-line and fund-detail decoding.

## 2. HQ Sorted Quote Lists And Top Boards

- [ ] 2.1 Add typed request/response models for sorted quote lists with category, sort key, start, count, reverse order, exclude filters, and raw protocol metadata.
- [ ] 2.2 Implement quote-list packet builder and decoder in `internal/tdx`.
- [ ] 2.3 Add typed request/response models for top boards with grouped ranking output and raw group ids.
- [ ] 2.4 Implement top-board packet builder and decoder in `internal/tdx`.
- [ ] 2.5 Add `QuoteSession` methods and `FetchHQ*` helpers with existing timeout, fallback, and BestIP behavior.
- [ ] 2.6 Add unit tests for validation, decoder fixtures, fake-server request wiring, and server fallback behavior.

## 3. SP Live Board Members

- [ ] 3.1 Add explicit SP session bootstrap/open helper if the existing HQ session cannot safely represent SP/MAC mode.
- [ ] 3.2 Add typed request/response models for SP board members with board id, sort type, count, sort order, decoded known fields, and raw bitmap fields.
- [ ] 3.3 Implement SP board-member packet builder, decoder, and auto-pagination.
- [ ] 3.4 Add fetch helper with explicit SP server options and clear errors when a server does not support SP board reads.
- [ ] 3.5 Add tests that prove existing HQ block-file reads and client-local block imports remain unchanged.

## 4. F10 LHB Parser

- [ ] 4.1 Add LHB record model with date, reason, buy seats, sell seats, amounts, net amount, raw text, and parser warnings.
- [ ] 4.2 Implement F10 category matching for `资金动向` and configurable aliases.
- [ ] 4.3 Implement LHB text parser using deterministic fixtures and no live server dependency.
- [ ] 4.4 Add fetch helper that composes existing F10 category/content reads and the LHB parser.
- [ ] 4.5 Add tests for missing section, empty content, partial rows, and normal records.

## 5. Fund-Specific 7727 Reads

- [ ] 5.1 Add fund server options and fund 7727 bootstrap/open helper separate from generic ExHQ reads.
- [ ] 5.2 Add typed request/response models for fund K-line.
- [ ] 5.3 Implement fund K-line packet builder, decoder, and fetch helper.
- [ ] 5.4 Add typed request/response models for fund detail with raw item id and six-value arrays.
- [ ] 5.5 Implement fund-detail packet builder, decoder, mode option, and fetch helper.
- [ ] 5.6 Add tests proving existing generic ExHQ bar commands still use the generic path.

## 6. CLI Productization

- [ ] 6.1 Add `marketd hq-quotes-list` command with category, sort, start/count, reverse, exclude, server, timeout, and BestIP flags.
- [ ] 6.2 Add `marketd hq-top-board` command with category, size, server, timeout, and BestIP flags.
- [ ] 6.3 Add `marketd sp-board-members` command with board id, sort type, count, order, server, and timeout flags.
- [ ] 6.4 Add `marketd hq-lhb` command with market/symbol, server, timeout, and optional F10 category alias flags.
- [ ] 6.5 Add `marketd fund-kline` and `marketd fund-detail` commands with fund server, code, period/count or mode, and timeout flags.
- [ ] 6.6 Add CLI tests for successful JSON output, validation errors, explicit server override, and no ClickHouse dependency.

## 7. HTTP Provider API

- [ ] 7.1 Extend `internal/querier` provider types for quote lists, top boards, SP board members, LHB, fund K-line, and fund detail.
- [ ] 7.2 Add `/api/tdx/hq/quotes-list` endpoint.
- [ ] 7.3 Add `/api/tdx/hq/top-board` endpoint.
- [ ] 7.4 Add `/api/tdx/sp/board-members` endpoint.
- [ ] 7.5 Add `/api/tdx/hq/lhb` endpoint.
- [ ] 7.6 Add `/api/tdx/fund/kline` and `/api/tdx/fund/detail` endpoints.
- [ ] 7.7 Add HTTP tests for query validation, provider wiring, response shape, and provider boundary separation from `/api/v1`.

## 8. Documentation And Validation

- [ ] 8.1 Update `docs/reference/tdx-server-capabilities.md` with advanced server capabilities, SP/fund server boundaries, and known public-server caveats.
- [ ] 8.2 Update `docs/design/tdx-server-capabilities.md` with implementation status, CLI matrix, provider endpoint matrix, and non-persistence boundary.
- [ ] 8.3 Update `docs/api/tdx.md` with request/response examples for every new endpoint.
- [ ] 8.4 Update `docs/reference/tdx-python-libraries.md` to mark the `millken/tdx` gaps as covered once implemented.
- [ ] 8.5 Run `go test ./...`.
- [ ] 8.6 Run `openspec validate --all`.
