## 1. Protocol

- [x] 1.1 Add extended quote setup packet and default `exhq` server candidates.
- [x] 1.2 Add extended market list packet builder and decoder.
- [x] 1.3 Add extended instrument quote packet builder and decoder.
- [x] 1.4 Add an `ExQuoteSession` that uses the shared TDX response header handling but a separate `exhq` setup path.

## 2. CLI

- [x] 2.1 Add `exquote-markets` command with repeatable/comma-separated `--server`.
- [x] 2.2 Add `exquote` command with `--market`, `--code`, and repeatable/comma-separated `--server`.
- [x] 2.3 Emit deterministic JSON and never open ClickHouse from these commands.

## 3. Tests

- [x] 3.1 Add tests for extended quote request validation.
- [x] 3.2 Add tests for extended market-list packet and decoder.
- [x] 3.3 Add tests for extended quote packet and decoder.
- [x] 3.4 Add tests for session setup and CLI wiring with scripted local servers.

## 4. Documentation

- [x] 4.1 Update realtime quote design docs with implemented `exhq` commands and limits.
- [x] 4.2 Update command guidance to include `exquote` and `exquote-markets`.

## 5. Validation

- [x] 5.1 Run `gofmt`.
- [x] 5.2 Run `go test ./...`.
- [x] 5.3 Run `openspec validate support-tdx-exhq-quotes --type change`.
