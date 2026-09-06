## 1. Package Decoder

- [x] 1.1 Implement bounded local/HTTP package loading and SHA-256 reporting.
- [x] 1.2 Parse and validate six dated `.cod`/`.md1` ZIP entries and aligned record counts.
- [x] 1.3 Normalize recognized traded A-share records and classify expected skipped records.
- [x] 1.4 Add synthetic parser and downloader tests for valid, incomplete, mismatched, duplicate, corrupt, and oversized inputs.

## 2. Import Orchestration

- [x] 2.1 Add the g4day ingest adapter on `RunOnlineJob` with daily writes, task metadata, watermark, and dry-run behavior.
- [x] 2.2 Add ingest tests for remote/local sources, summaries, validation failures, and write-plane lifecycle.

## 3. CLI and Documentation

- [x] 3.1 Add `marketd import-tdx-g4-day` flags, routing, help, and summary output.
- [x] 3.2 Add CLI tests for remote date mode, local replay, dry-run, and invalid source combinations.
- [x] 3.3 Document command usage, format limits, A-share scope, and the separation from realtime display and adjustment refresh.

## 4. Verification

- [x] 4.1 Run the parser against the official 2026-09-04 package and confirm the expected 5,547 stock bars with no issues.
- [x] 4.2 Run formatting, targeted tests, the full Go test suite, and `openspec validate --all`.
