# Antares API

The only public surface of Antares. It resolves stars, answers from caches when it can, queues the ten-minute feature extractions otherwise, streams their progress over WebSockets, and calls the private Shaula (feature extraction) and Fang (classifier) gRPC services.

## Running

```sh
npx nx serve api          # go run; npm run dev:api stores the database in .data/api
npx nx test api           # go test -race ./...
npx nx run api:docker-build
npx nx run api:sqlc       # regenerate internal/db after editing internal/store/queries or migrations
npx nx run api:sqlc-check # fail if the committed internal/db is stale
```

The API starts even when Shaula or Fang is unreachable; `/healthz` then reports `ready: false` until both have answered.

Configuration is via `ANTARES_*` environment variables, all listed with their defaults and explanations in [`.env.example`](.env.example). Copy it to `.env` for local development; `npm run dev:api` and `make dev-api` load it.

## Reference

Routes, bodies, limits and errors are specified in [`openapi.yaml`](openapi.yaml) (embedded in the binary). A running API serves it at `/openapi.yaml` and as Swagger UI at `/docs`.

`GET /targets/features` (by `query` or `ra_deg`+`dec_deg`) and `GET /subscription/<token>/features` return a star's latest cached features without starting a job; `404` when none exist yet.

## Subscription stream

`POST /pipeline/extract` and `POST /analyze` answer `202` with a `token` when a job is queued. Open a WebSocket on `GET /subscription/<token>` (same path as the JSON snapshot, with `Upgrade: websocket`) to follow it.

- Frames are `{"events":[...]}`; progress is batched into one frame per `ANTARES_COALESCE_INTERVAL`. Each event is `{"seq","type","stage","message","fraction","at","payload"}`; `stage`, `message`, `fraction` and `payload` are omitted when empty.
- `type` is `state` (a milestone such as `queued`, `running`, `predicting`), `progress` (`fraction`, normally 0 to 1, is omitted when unknown), `result` (`payload` is the result object from the spec) or `error` (`message` says why). The stream ends with a `result` or an `error`, then a normal close (1000).
- `seq` starts at 1 and increases. Reconnect with `?since=<last seq>` to receive only what you missed. Replay comes from a bounded in-memory buffer (`ANTARES_RING_SIZE`, 500 events), so a reconnect after a long gap may skip progress lines.
- A connection that falls behind (`ANTARES_CONN_BUFFER` frames) is closed with 1013; reconnect with `?since=`. Any other close before the terminal event is also safe to retry the same way.
- The server pings every 20 seconds and closes a connection that does not answer.
- Browsers must connect from an origin in `ANTARES_ALLOWED_ORIGINS` or the API's own host (others get `403`); clients without an `Origin` header are accepted.
- Once a job has finished, a connection receives one frame with its persisted milestones and the terminal event (newer than `since`), then a normal close.
- The token is the credential. It is stored hashed, is valid for `ANTARES_TOKEN_RETENTION`, and is never logged. Store it (for example in `localStorage`) to resume after a refresh.

## Behaviour worth knowing

- Until a job is `running`, keep a WebSocket open or poll the snapshot: a waiting job with nobody connected or polling for longer than `ANTARES_PROMOTION_GRACE` is dropped.
- Features are cached per star, extraction parameters and Shaula version; a new Shaula version supersedes (never deletes) older entries, and TESS entries expire after `ANTARES_TESS_TTL`. Predictions are cached per features and model run. A failed or stalled job writes nothing; features survive a later prediction failure.
- On shutdown, queued jobs stay queued; the next start fails every unfinished job with `interrupted by an API restart; resubmit`. Tokens issued before the restart keep working and report that state.
- A timeout or stall abandons the upstream call from the Go side only; the Python worker keeps running to completion.
