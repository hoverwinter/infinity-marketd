## 1. Schema And Models

- [x] 1.1 Add bootstrap DDL for `a_share_xdxr_events`.
- [x] 1.2 Add bootstrap DDL for `a_share_adjust_factors_1d`.
- [x] 1.3 Add Go model types for normalized xdxr events and adjustment factors.
- [x] 1.4 Add ClickHouse insert paths for xdxr events and adjustment factors.
- [x] 1.5 Add schema tests covering both new tables and proving adjusted K-line tables are not created.

## 2. XDXR Refresh

- [x] 2.1 Add a `marketd refresh-tdx-xdxr` command for one market/symbol.
- [x] 2.2 Normalize decoded `tdx.HQXDXRInfo` rows into xdxr event model rows.
- [x] 2.3 Record task runs, watermarks, and quality issues for xdxr refresh.
- [x] 2.4 Add tests for successful xdxr refresh, empty response, and unsupported category preservation.

## 3. Factor Refresh

- [x] 3.1 Add read helpers needed by refresh jobs to load raw daily bars and persisted xdxr events for one symbol.
- [x] 3.2 Implement ordinary category `1` event ratio calculation from previous valid raw close.
- [x] 3.3 Implement qfq factor generation anchored at the latest available daily bar.
- [x] 3.4 Implement hfq factor generation anchored at the earliest available daily bar.
- [x] 3.5 Add a `marketd refresh-adjust-factors` command for one market/symbol.
- [x] 3.6 Record task runs, watermarks, and data quality issues for missing or invalid factor inputs.
- [x] 3.7 Add tests for no-event symbols, one dividend/share event, multiple events, missing previous close, and rebuild replacement.

## 4. Querier API

- [x] 4.1 Add `Adjust` to `querier.BarQuery` with default `none`.
- [x] 4.2 Validate `adjust` as `none`, `qfq`, or `hfq`.
- [x] 4.3 Include normalized `adjust` in the `/api/v1/bars` query echo.
- [x] 4.4 Update daily bar SQL to join `a_share_adjust_factors_1d` for adjusted queries.
- [x] 4.5 Update minute bar SQL to join daily factors by `market`, `symbol`, and `trade_date` for adjusted queries.
- [x] 4.6 Ensure adjusted queries multiply only OHLC and leave volume and amount raw.
- [x] 4.7 Ensure adjusted queries do not call live TDX provider paths.
- [x] 4.8 Add API and repository tests for raw default, qfq daily, hfq daily, qfq minute, invalid adjust, and missing factor behavior.

## 5. CLI Client And Docs

- [x] 5.1 Add `--adjust` to `infinity querier bars`.
- [x] 5.2 Update `docs/storage/clickhouse.md` with xdxr event and factor table contracts.
- [x] 5.3 Update `docs/api/README.md` with `adjust` semantics, default behavior, missing-factor behavior, and raw volume/amount policy.
- [x] 5.4 Update README command examples for adjusted bar queries.
- [x] 5.5 Document non-destructive migration and backfill order for existing deployments.

## 6. Verification

- [x] 6.1 Run focused tests for `internal/tdx`, `internal/clickhouse`, `internal/cli`, `internal/querier`, and `internal/infinitycli`.
- [x] 6.2 Run `make test`.
- [x] 6.3 Run `openspec validate --all`.
- [x] 6.4 Manually dry-run or fixture-test the operator workflow: refresh xdxr, refresh factors, query raw bars, query qfq bars, query hfq bars.
