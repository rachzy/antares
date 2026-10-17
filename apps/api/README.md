# Antares API

The only public surface of Antares. It resolves stars, answers from caches when it can, queues the ten-minute feature extractions otherwise, streams their progress over WebSockets, and calls the private Shaula (feature extraction) and Fang (classifier) gRPC services.

## Running

```sh
npx nx serve api          # go run, needs Shaula and Fang reachable
npx nx test api           # go test -race ./...
npx nx run api:docker-build
```

| Variable                    | Default           | Meaning                                                                          |
| --------------------------- | ----------------- | -------------------------------------------------------------------------------- |
| `ANTARES_ADDR`              | `:8080`           | listen address                                                                   |
| `ANTARES_DB_PATH`           | `antares.db`      | SQLite file (targets, caches, jobs, subscriptions)                               |
| `ANTARES_SHAULA_ADDR`       | `127.0.0.1:50051` | Shaula gRPC address (plaintext: private network only)                            |
| `ANTARES_FANG_ADDR`         | `127.0.0.1:50052` | Fang gRPC address (plaintext: private network only)                              |
| `ANTARES_MAX_ACTIVE`        | `2`               | extractions running at once; keep at or below Shaula's worker pool (4)           |
| `ANTARES_MAX_QUEUE`         | `50`              | waiting jobs allowed before new work is refused with `503`                       |
| `ANTARES_PROMOTION_GRACE`   | `30s`             | a waiting job is kept only if a client is connected or polled less than this ago |
| `ANTARES_JOB_TIMEOUT`       | `30m`             | hard limit for one job                                                           |
| `ANTARES_STALL_TIMEOUT`     | `5m`              | a job with no progress event for this long is failed                             |
| `ANTARES_TOKEN_RETENTION`   | `168h`            | how long a token keeps working                                                   |
| `ANTARES_TESS_TTL`          | `720h`            | TESS cache entries expire after this (Kepler and K2 never do)                    |
| `ANTARES_RING_SIZE`         | `500`             | events kept per job for replay                                                   |
| `ANTARES_CONN_BUFFER`       | `64`              | per-WebSocket buffer; a connection that falls behind is dropped                  |
| `ANTARES_COALESCE_INTERVAL` | `100ms`           | progress events are batched into one frame per interval                          |
| `ANTARES_ALLOWED_ORIGINS`   | none              | comma-separated origins allowed by CORS and WebSocket origin checks              |

## Routes

All bodies are JSON and strict: unknown fields are a `400`. Bodies are limited to 1 MiB.

### `POST /targets/resolve`

`{"query":"Kepler-186f","mission":"Kepler"}` returns `{"catalog":"KIC","catalog_id":"8120608","display_name":"Kepler-186","designation":"KIC 8120608","ra_deg":298.65,"dec_deg":43.95}`.

`{"ra_deg":298.65272,"dec_deg":43.95502,"mission":"Kepler"}` returns the same shape.

`mission` is `Kepler` (default), `K2` or `TESS`, case-insensitive. `ra_deg`/`dec_deg` may replace `query` (send one or the other, never both); the nearest star with light curves within 4 arcseconds is returned, and an empty patch of sky is a 404.

### `POST /pipeline/extract` and `POST /analyze`

Body, all optional except a `query` (or `ra_deg` and `dec_deg` instead, never both): `query`, `ra_deg`, `dec_deg`, `mission`, `sigma_clip` (default 5, in (0, 50]), `download_all`, `author`, `exptime`, `use_tls`, `mask_eclipses`, `force_refresh`, and for `/analyze` also `run_id` (default: newest model) and `star_id`.

- `200 {"status":"done","result":{...}}` when the answer was already cached. `result.cached` is `true` and no job ran.
- `202 {"status":"queued"|"running","token":"...","subscription_path":"/subscription/<token>","position":N}` otherwise. Until the job is `running`, keep a WebSocket open on the subscription or poll its snapshot: a waiting job with nobody connected or polling for longer than `ANTARES_PROMOTION_GRACE` is dropped, by the periodic sweep or when its turn comes.

`result` is `{"target":{...},"cached":bool,"shaula_version":"...","features":[...],"model_run_id":"...","schema_version":1,"predictions":[...]}`; the last three appear only for `/analyze`. Missing feature values are `null`.

Identical requests made while one is running share a single job; each caller still gets its own token.

### `POST /ml/predict`

`{"features":[{...}],"run_id":"...","star_id":"..."}` (at most 100 rows) returns `{"model_run_id","schema_version","predictions":[...],"cached":bool}`.

### `GET /ml/runs`

`{"runs":[{"run_id","created_at","schema_version","selected_model","threshold"}]}`, newest first. `503` from `/analyze` and `/ml/predict` means no trained model is available yet.

### `GET /healthz`

`{"status":"ok","ready":bool,"shaula_version":"...","fang_version":"..."}`. `ready` is `false` until both services have answered once.

### `GET /subscription/{token}`

Without an `Upgrade: websocket` header it returns a JSON snapshot: `{"kind","state","position","error","result","milestones":[...],"last_seq"}`. `state` is `queued`, `running`, `done`, `failed` or `dropped`. Polling the snapshot counts as presence, so it also keeps a waiting job alive.

With the upgrade it streams frames `{"events":[{"seq","type","stage","message","fraction","at","payload"}]}`:

- `type` is `state`, `progress`, `result` or `error`. The stream ends with a `result` (payload = the result above) or an `error`, then a normal close (1000).
- `seq` starts at 1 and has no gaps. Reconnect with `?since=<last seq>` to receive only what you missed; a finished job is replayed from its persisted milestones.
- A client too slow to keep up is closed with code 1013; reconnect with `?since=`.
- The token is the credential. It is stored hashed, is valid for `ANTARES_TOKEN_RETENTION`, and is never logged. Store it (for example in `localStorage`) to resume after a refresh.

## Errors

`{"error":{"code","message"}}`

| Status | `code`            | When                                                     |
| ------ | ----------------- | -------------------------------------------------------- |
| 400    | `invalid_request` | bad input                                                |
| 404    | `not_found`       | unknown or expired token, unknown run, unresolvable star |
| 413    | `too_large`       | body over 1 MiB                                          |
| 422    | `unprocessable`   | extraction failed (for example no light curves)          |
| 503    | `unavailable`     | a service is unreachable or no model is trained          |
| 503    | `busy`            | queue full; honour `Retry-After`                         |
| 500    | `internal`        | unexpected; the message is always generic                |

## Behaviour worth knowing

- Features are cached per star, extraction parameters and Shaula version; a new Shaula version supersedes (never deletes) older entries. Predictions are cached per features and model run.
- A job that fails or stalls writes nothing to the cache. Features survive a later prediction failure.
- On shutdown, queued jobs stay queued; the next start fails every unfinished job with `interrupted by an API restart; resubmit`. Tokens issued before the restart keep working and report that state, and a reconnect to a finished job always replays its terminal event.
- The API never blocks on upstream deadlines: a timeout or stall abandons the call from the Go side only, and the Python worker keeps running to completion.
