# backend

Minimal Go HTTP service scaffold.

## Run

```sh
go run ./cmd/server        # or: make run
PORT=9000 go run ./cmd/server
```

## Endpoints

### `GET /api/health`

```sh
curl localhost:8080/api/health
# {"status":"ok","time":"...","uptime":"3s"}
```

### `GET /api/rates`

Latest exchange rates, sourced from the [frankfurter.dev](https://frankfurter.dev) v2 API.

| Param     | Required | Notes                                                       |
|-----------|----------|-------------------------------------------------------------|
| `base`    | yes      | Three-letter code, case-insensitive.                         |
| `targets` | no       | Comma-separated codes; repeatable. Omit for every currency.  |

```sh
curl "localhost:8080/api/rates?base=USD&targets=EUR,SGD"
# {"base":"USD","date":"2026-09-15","rates":{"EUR":0.86515,"SGD":1.2708}}
```

Behaviour worth knowing:

- Codes are upper-cased and de-duplicated; `usd` and `USD` are the same request.
- `base` listed in `targets` comes back as `1`.
- An unknown code is a `400` naming that code, never a silently missing key.
- Upstream failures surface as `502`, never as a partial `200`.

Upstream dates each currency pair separately, so a thinly traded currency can be
days staler than the rest of the same response. `date` is the newest date in the
set; any pair published earlier is listed under `staleDates`:

```sh
curl "localhost:8080/api/rates?base=USD&targets=EUR,ANG"
# {"base":"USD","date":"2026-09-15","rates":{"ANG":1.79,"EUR":0.86515},
#  "staleDates":{"ANG":"2026-09-11"}}
```

| Status | When                                                  |
|--------|-------------------------------------------------------|
| `400`  | Missing `base`, malformed code, or unquoted currency. |
| `502`  | frankfurter.dev unreachable or erroring.              |

## Docker

```sh
docker build -t incubrix-backend .
docker run --rm -p 8080:8080 incubrix-backend
```

Configuration is passed the same way the binary reads it locally:

```sh
docker run --rm -p 8080:8080   -e CORS_ALLOWED_ORIGINS=http://localhost:5173   incubrix-backend
```

The build is two stages. The first compiles a static binary with
`CGO_ENABLED=0`; the second is `distroless/static`, which carries CA
certificates for the outbound TLS call and nothing else — no shell, no package
manager, no libc. The server runs as `nonroot` (uid 65532).

`go.mod` and `go.sum` are copied before the source so that editing code does not
invalidate the dependency-download layer.

`ENTRYPOINT` is exec form, so the server is PID 1 and receives the `SIGTERM`
that `docker stop` sends — which is what its graceful shutdown waits on.

There is no `HEALTHCHECK` line: the image has no shell or `curl` to run one
with. Point your orchestrator's HTTP probe at `/api/health` instead.

## Caching

Rates are held in memory for an hour, keyed by base currency.

A miss fetches **every** currency for that base, not just the ones requested,
because upstream charges the same single request either way (10 KB vs 127 B).
Every later request for that base is then served by filtering in memory,
whatever its targets, so `?targets=EUR` and `?targets=JPY,INR` share one entry.
The key space is the set of currencies, so the cache is bounded at roughly
165 entries without needing eviction.

Measured against the live API:

| Request                          | Time     |
|----------------------------------|----------|
| `base=USD` first call (miss)     | 173 ms   |
| `base=USD` repeat (hit)          | 1.1 ms   |
| `base=USD`, different targets    | 1.3 ms   |
| `base=EUR` (new base, miss)      | 36 ms    |

Concurrent misses on the same base are collapsed into one upstream call with
`singleflight`; the rest wait for and share its result. Without it, a cold cache
under load sends one request per caller to a free public API.

Expiry is checked on read, so there is no background goroutine to shut down.
Failed fetches are not cached. The shared fetch is deliberately detached from
the triggering request's context, so one caller giving up does not cancel a
fetch the others are waiting on; the HTTP client's own timeout still bounds it.

TTL is `rateCacheTTL` in `cmd/server/main.go`.

## CORS

Browser callers are served via an allowlist. Set `CORS_ALLOWED_ORIGINS` to a
comma-separated list of origins; `*` allows every origin. The allowlist in use
is logged at startup.

```sh
CORS_ALLOWED_ORIGINS=http://localhost:4200 go run ./cmd/server
```

Unset, it defaults to the usual local UI dev servers:
`http://localhost:3000`, `http://localhost:5173`, and their `127.0.0.1` forms.

- The allowed origin is echoed back rather than `*`, so enabling credentialed
  requests later needs no rework.
- `Vary: Origin` is always sent on cross-origin responses so a shared cache
  cannot serve one origin's response to another.
- Preflight `OPTIONS` is answered by the middleware, which wraps the mux — the
  mux itself only routes `GET` and would reject preflight with `405`.
- A request from an unlisted origin is still served, just without the
  `Access-Control-Allow-Origin` header; the browser withholds the body from the
  page. Preflight from an unlisted origin gets a `403` so the cause is visible
  in devtools.

## Layers

```
handler  parses and validates the HTTP shape, maps errors to status codes,
         and applies the CORS and request-logging middleware
cache    per-base rate cache in front of the adapter, same interface
service  business rules: normalise codes, fold per-pair rows into a rate map,
         flag stale pairs, verify every requested target came back
adapter  the frankfurter.dev v2 HTTP client; knows `quotes`, not `targets`
```

Each layer depends on an interface, not a struct, so the one above it is tested
with a fake and the suite makes no network calls.

## Layout

```
cmd/server/        entrypoint, server lifecycle, graceful shutdown
internal/handler/  router, HTTP handlers, error mapping
internal/service/  business rules
internal/cache/    in-memory rate cache
internal/adapter/  outbound clients (frankfurter.dev)
```

## Test

```sh
go test ./...     # or: make test
```
