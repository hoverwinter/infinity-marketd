# MySQL Securities Master

`marketd` stores mutable securities reference data in MySQL. ClickHouse remains the store for行情 facts, trading facts, derived market data, and ops records.

## Configuration

MySQL is explicitly enabled. Existing ClickHouse commands do not require MySQL unless a securities-master command or API is used.

```yaml
mysql:
  enabled: true
  addr: "127.0.0.1:3306"
  user: "marketd"
  password: ""
  database: "infinity_market"
  max_open_conns: 5
  max_idle_conns: 2
  conn_max_lifetime: "5m"
```

Environment overrides use `MARKETD_MYSQL_*`, for example `MARKETD_MYSQL_ENABLED`, `MARKETD_MYSQL_ADDR`, `MARKETD_MYSQL_DATABASE`, `MARKETD_MYSQL_USER`, and `MARKETD_MYSQL_PASSWORD`.

TDX native refresh can also use MAC HQ servers for Beijing Stock Exchange names:

```yaml
tdx:
  hq_servers:
    - "180.153.18.170:7709"
  mac_hq_servers:
    - "121.36.248.138:7709"
    - "123.60.47.136:7709"
    - "121.37.207.165:7709"
```

## Tables

`securities` stores current metadata keyed by `(market, symbol)`: exchange, current name, normalized name, board, status, listing/delisting dates, lot size, price precision, source, manual lock state, and timestamps.

`security_name_history` stores effective-dated names keyed by `(market, symbol, valid_from, name_norm)`. `manual_override` protects manually curated segments from source refresh overwrite.

`security_aliases` stores searchable aliases keyed by `(market, symbol, alias_norm, alias_type)`.

`security_refresh_runs` records refresh source, requested markets, timing, status, row counts, and error text.

`market='bj'`, `exchange='BSE'`, and `board='bse'` represent Beijing Stock Exchange rows in the same tables as `sh` and `sz`.

## Refresh

Native TDX refresh:

```bash
go run ./cmd/marketd refresh-security-master \
  --source tdx \
  --market sh,sz,bj \
  --server 180.153.18.170:7709 \
  --mac-server 121.36.248.138:7709
```

`sh` and `sz` use the standard HQ security count/list path (`0x044E` + `0x0450`).

`bj` uses a compatibility path because public standard HQ servers commonly time out on legacy market byte `2` security list reads:

1. `0x054B` quotes list with `category=12`, `sort=code`, and page size `80` enumerates BJ codes.
2. Rows are filtered to `market_code=2`; pagination stops when the returned page is shorter than `80` because overrun pages can contain non-BJ rows.
3. MAC HQ `0x122B` batch symbol quotes fetch current names for those codes.
4. The resulting rows are normalized into the same MySQL tables as `sh` and `sz`.

This two-step TDX read happens only inside explicit refresh/provider commands. `/api/v1/bars` does not join MySQL names and does not trigger live TDX reads.

Normalized CSV refresh:

```bash
go run ./cmd/marketd refresh-security-master \
  --source file \
  --file securities.csv \
  --market sh,sz,bj
```

CSV headers use canonical field names such as `market`, `symbol`, `current_name`, `exchange`, `board`, `status`, `listing_date`, `delisting_date`, `lot_size`, `price_precision`, `aliases`, and `source`.

`--dry-run` fetches and normalizes rows without writing MySQL.

## Query Boundary

The querier exposes:

- `GET /api/v1/securities?market=sh&symbol=600519`
- `GET /api/v1/securities/resolve?q=贵州茅台`

`GET /api/v1/bars` remains ClickHouse-only and never joins MySQL names into bar responses.
