## 1. Protocol

- [x] 1.1 Add ExHQ instrument count packet and decoder.
- [x] 1.2 Add ExHQ instrument list packet and decoder.
- [x] 1.3 Add ExHQ K-line packet and decoder.
- [x] 1.4 Add ExHQ minute-time and historical minute-time packets and decoders.
- [x] 1.5 Add ExHQ transaction and historical transaction packets and decoders.
- [x] 1.6 Add ExHQ historical K-line range packet and decoder.
- [x] 1.7 Decode ExHQ GBK/GB18030 text fields when UTF-8 is not valid.

## 2. CLI

- [x] 2.1 Add `exquote-count`.
- [x] 2.2 Add `exquote-instruments`.
- [x] 2.3 Add `exquote-bars`.
- [x] 2.4 Add `exquote-minute`.
- [x] 2.5 Add `exquote-history-minute`.
- [x] 2.6 Add `exquote-transactions`.
- [x] 2.7 Add `exquote-history-transactions`.
- [x] 2.8 Add `exquote-history-bars`.
- [x] 2.9 Keep all ExHQ commands read-only and independent of ClickHouse config.

## 3. Tests

- [x] 3.1 Add packet construction tests.
- [x] 3.2 Add response decoder tests.
- [x] 3.3 Add scripted local server tests for fetch wiring.
- [x] 3.4 Add CLI JSON and argument tests.

## 4. Documentation and Validation

- [x] 4.1 Update docs with expanded ExHQ command guidance.
- [x] 4.2 Record current server availability caveats.
- [x] 4.3 Run `gofmt`.
- [x] 4.4 Run Go tests.
- [x] 4.5 Run OpenSpec validation.
