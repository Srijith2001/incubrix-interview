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
internal/adapter/  outbound clients (frankfurter.dev)
```

## Test

```sh
go test ./...     # or: make test
```
