## 1. Documentation And Specs

- [x] 1.1 Document that canonical 1m/5m imports write raw fact tables only.
- [x] 1.2 Document scan-derived table purpose, narrow schema, retention, and rebuild semantics.
- [x] 1.3 Add OpenSpec requirements for explicit scan refresh and no default scan generation during offline import.

## 2. Schema

- [ ] 2.1 Add bootstrap DDL for `a_share_bars_1m_scan`.
- [ ] 2.2 Add bootstrap DDL for `a_share_bars_5m_scan`.
- [ ] 2.3 Use monthly partitions and time-first order keys for scan tables.
- [ ] 2.4 Configure short retention by TTL or documented partition maintenance.

## 3. Refresh Job

- [ ] 3.1 Add an explicit `refresh-minute-scan` command or scheduled job.
- [ ] 3.2 Support `--period 1m|5m`, `--since`, and `--until`.
- [ ] 3.3 Rebuild scan rows from canonical minute facts for the requested window.
- [ ] 3.4 Avoid full-table mutations during large refreshes; prefer partition or bounded-window replacement.

## 4. Import Path

- [ ] 4.1 Ensure `import-tdx-1m` writes only `a_share_bars_1m` by default.
- [ ] 4.2 Ensure `import-tdx-5m` writes only `a_share_bars_5m` by default.
- [ ] 4.3 Do not add implicit scan refresh side effects to offline imports.
