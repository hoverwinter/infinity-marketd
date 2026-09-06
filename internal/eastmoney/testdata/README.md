# Protocol fixtures

These are synthetic fixtures following AKShare's Eastmoney `clist/get` and
`kline/get` response contracts, not captured live market observations. The history
values deliberately differ between close/high/low to detect field-order errors.

References checked 2026-09-06:
- https://github.com/akfamily/akshare/blob/main/akshare/stock/stock_board_industry_em.py
- https://github.com/akfamily/akshare/blob/main/akshare/stock/stock_board_concept_em.py
- https://github.com/akfamily/akshare/blob/main/akshare/utils/func.py

Live connectivity is tested separately with `MARKETD_EASTMONEY_PROBE=1`.
