# internal/eastmoney/

Eastmoney online board-index data. `client.go` owns bounded HTTP and rc/data envelopes; `boards.go` owns complete category catalog scans and resolution; `bars.go` owns daily kline date chunks and source-field normalization. Implements existing `marketdata.BarsProvider` and `BoardProvider`.

Keep `90.BK...`, pagination, source fields and request options here. No querier, ingest or storage dependencies. Never silently skip failed pages/chunks, substitute another provider or treat null data as an empty result. Fixtures derived from protocol examples must be identified separately from captured live data.
