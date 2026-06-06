## Context

TDX standard行情 security-list records store names in an 8-byte fixed field. Chinese names are commonly encoded as GBK/GB18030, while the current Go decoder only keeps names when the raw bytes are valid UTF-8. As a result, common names such as `贵州茅台` or `平安银行` are dropped even though the code, market, and quote workflow are valid.

This change is limited to name decoding. It does not alter security-list pagination, quote request construction, market mapping, or quote response decoding.

## Goals / Non-Goals

**Goals:**
- Decode security-list names from GB18030-compatible bytes.
- Preserve ASCII names.
- Trim null and space padding from the fixed 8-byte field.
- Keep malformed name bytes from failing the whole security-list response.
- Add unit tests for Chinese GBK names, ASCII names, padding, and invalid bytes.
- Update docs that currently state security names are dropped when non-UTF-8.

**Non-Goals:**
- No change to quote price/volume decoding.
- No change to `bj` market support.
- No `exhq` market-name decoding in this change.
- No ClickHouse schema or persistence change.

## Decisions

- Use `golang.org/x/text/encoding/simplifiedchinese` with `GB18030`.
  - Rationale: GB18030 is compatible with GBK/GB2312 and handles normal TDX Chinese names.
  - Alternative considered: write a custom GBK decoder. That is unnecessary and error-prone.

- Decode names in a small helper.
  - Rationale: `DecodeSecurityListResponse` should remain focused on record structure, and tests can target decoding behavior directly.
  - Alternative considered: inline decoder logic. That makes fallback and padding behavior harder to test.

- Treat name decode errors as field-level failures.
  - Rationale: an invalid name should not discard a valid symbol needed for quote sweep.
  - Alternative considered: fail the whole response. That would make symbol discovery brittle.

## Risks / Trade-offs

- Adds `golang.org/x/text` dependency -> acceptable because it is the standard Go ecosystem package for legacy text encodings.
- Fixed 8-byte TDX name field can truncate long Chinese names -> decode the bytes as provided and do not try to repair truncation.
- Some servers may return names in a different encoding -> malformed names remain empty while symbols stay usable.
