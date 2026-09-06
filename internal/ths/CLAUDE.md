# internal/ths/

THS HTTP transport and data adapters. `client.go` owns bounded requests, optional startup cookie and GBK pages; `boards.go` owns current catalogs and page-ID/quotation-ID resolution; `bars.go` owns annual index daily JSONP. AKShare is the upstream protocol reference (links in `docs/api/providers.md`).

Implement optional `marketdata` capabilities; never import querier, ingest or database packages. Do not execute upstream JavaScript, silently skip failed years, infer concept quotation codes, or invent volume-unit conversions. New product-specific parsers belong in separate files here when needed.
