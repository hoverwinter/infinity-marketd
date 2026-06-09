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
  --server 180.153.18.170:7709
```

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
