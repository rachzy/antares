## Go API (`apps/api`)

- The only public surface. Go 1.27 module `antares/api`, standard-library HTTP (`http.ServeMux` method and wildcard patterns), `github.com/coder/websocket`, `modernc.org/sqlite` (no cgo).
- Layout: `cmd/api` (entry point), `openapi.go` (embeds the spec) and `internal/` (private to the app, enforced by the Go toolchain): `config`, `store` (SQLite, migrations, targets, sqlc query files), `db` (sqlc-generated, do not edit), `cache`, `hub` (events), `jobs` (store, scheduler, runner), `upstream` (gRPC clients and `fake`), `service` (business logic), `httpapi`, `app` (wiring).
- Commands: `npx nx test api` (`go test -race`), `npx nx lint api` (`gofmt` + `go vet`), `npx nx build api`, `npx nx serve api`, `npx nx run api:sqlc` (regenerate `internal/db`), `npx nx run api:sqlc-check` (fail on stale generated code), `npx nx run api:docker-build`. `build`, `serve`, `test` and `lint` depend on `proto:generate`, which needs `apps/shaula` and `apps/fang` installed (`npx nx install shaula`, `npx nx install fang`); `docker-build` generates the Go stubs inside the image.
- `npm run dev:api` / `make dev-api` run it locally with the database in `.data/api/antares.db`.

### Standards

- SQL lives in `internal/store/queries/*.sql` and is called through the sqlc-generated `internal/db` package; after editing a query or migration run `npx nx run api:sqlc` and commit the generated code. Only `store.Open` and the migration runner hold hand-written SQL.
- Configuration is environment variables only (`ANTARES_*`). Every variable is listed with its default and meaning in `apps/api/.env.example`; add new ones there and in `internal/config`.
- The HTTP contract is `apps/api/openapi.yaml`, served at `/openapi.yaml` with Swagger UI at `/docs`. Routes are registered from the route table in `internal/httpapi`, and a test fails if a route and the spec disagree, so update both together. The WebSocket protocol, which OpenAPI cannot describe, is documented in `apps/api/README.md`.
- Every exported identifier and package has a Go doc comment, and non-obvious logic (concurrency, dedupe, error mapping, hashing) has a short comment explaining why.
- Shaula and Fang are used only through the `upstream.Shaula` and `upstream.Fang` interfaces; every test above the gRPC layer uses `upstream/fake`.
- A subscription token is a bearer credential: stored hashed, never logged (log the route pattern, not the URL path).
- Public error messages never carry internal detail (addresses, SQL errors); log the detail instead.
- Hub appends never block: a slow WebSocket is dropped, never waited for.
- A job that fails writes nothing to the cache. Never cancel a running job because its subscriber left.
