# Infinity Console

Infinity Console is a browser-based operator surface for `infinity-marketd`. It shows health, watermarks, task runs, data quality issues, realtime quote service runs, TDX HQ smoke checks, and HQ bestip cache state.

It is not a ClickHouse admin UI and does not replace import CLI commands.

## Development

The console frontend lives in `web/console` and uses Node.js with Vite.

Recommended runtime:

```bash
node --version   # v20 or newer; v24 is supported
npm --version
```

Install dependencies:

```bash
make console-install
```

Start the Go API server:

```bash
make serve
```

Start the Vite dev server:

```bash
make console-dev
```

The Vite dev server proxies `/api/*` to `http://127.0.0.1:8808` by default. Override the target with:

```bash
INFINITY_QUERIER_URL=http://127.0.0.1:8808 make console-dev
```

## Build

Build static assets:

```bash
make console-build
```

The output is `web/console/dist`. It is generated output and is not committed.

Type-check without producing a Vite build:

```bash
make console-check
```

## Production Serving

Serve built console assets with the standalone console binary:

```bash
go run ./cmd/infinity-console \
  --config configs/config.yaml \
  --listen 127.0.0.1:8809 \
  --console-dist web/console/dist
```

Open:

```text
http://127.0.0.1:8809/console/
```

The binary also redirects `/` to `/console/`.

`infinity querier serve --console-dist web/console/dist` remains available for deployments that prefer a single read-plane process. If `--console-dist` is omitted, `infinity querier serve` remains API-only and does not require Node.js or built assets.

## Safety Boundary

The first console version is intended for local or private operator networks. It has no authentication layer.

The console exposes only read-only operational views plus narrow non-destructive actions:

- TDX HQ probe smoke checks
- TDX HQ quote smoke checks
- HQ bestip cache refresh

The console must not expose actions that drop, truncate, detach, replace, or delete ClickHouse tables or rows. Smoke checks do not write canonical market fact tables, create task runs, or advance watermarks.
