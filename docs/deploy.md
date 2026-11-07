# Deploying to a VPS

Every push to `main` runs `.github/workflows/deploy.yml`: it rsyncs the repository to the VPS over SSH and runs `make build-up ENV=prod` there.

## Releasing

Run the `Release` workflow (Actions > Release > Run workflow). Pick `patch`, `minor` or `major` to bump the latest `vX.Y.Z` tag (the first release starts from `v0.0.0`), or type an exact tag such as `v1.0.0` to override the bump. It merges `dev` into `main` with a merge commit, pushes the tag, creates a GitHub Release with generated notes and then calls `deploy.yml` for that tag. Untick the `Deploy to prod` checkbox to release without deploying.

Pushes made with the default `GITHUB_TOKEN` do not trigger other workflows, which is why the release calls the deploy directly. If branch protection on `main` blocks the push, allow GitHub Actions to bypass it or push with a PAT/GitHub App token instead.

## GitHub secrets

Add these under Settings > Secrets and variables > Actions.

| Secret            | Required | Value                                                                                           |
| ----------------- | -------- | ----------------------------------------------------------------------------------------------- |
| `VPS_HOST`        | yes      | VPS hostname or IP                                                                              |
| `VPS_USER`        | yes      | SSH user, which must be in the `docker` group                                                   |
| `VPS_SSH_KEY`     | yes      | Private key (full PEM/OpenSSH text) whose public half is in the user's `~/.ssh/authorized_keys` |
| `VPS_KNOWN_HOSTS` | yes      | Output of `ssh-keyscan -p <port> <host>`                                                        |
| `VPS_DEPLOY_PATH` | yes      | Absolute checkout path on the VPS, e.g. `/opt/antares`                                          |
| `VPS_PORT`        | no       | SSH port, defaults to `22`                                                                      |

## VPS prerequisites

- Docker with the compose plugin, `make` and `rsync` installed.
- `$VPS_DEPLOY_PATH/.env` (kept across deploys) with at least `BIND=0.0.0.0` to expose the proxy publicly, and `ANTARES_ALLOWED_ORIGINS=<url>` matching the URL users open. `PROXY_PORT` defaults to `80`.

## Model bundles

Copy a bundle directory to the VPS (e.g. `scp -r bundle vps:/opt/antares-models/v1`), then on the VPS:

```sh
make fang-models MODELS=/opt/antares-models/v1
```

This writes `FANG_MODELS_DIR` to `.env`, bind-mounts it at `/var/lib/fang/models` and recreates only the `fang` container, so the other services keep running. Without it, Fang uses the empty `antares-fang-models` volume.

### Training on the VPS

The manual **Train Fang model** workflow (Actions tab) downloads `data/extracted` from `rachzy/shaula`, runs `fang train` in a one-off Fang container that writes into the served models volume, and checks the run is listed. Inputs: model name (empty = timestamp), bundled config name (empty = default), skip evaluation, and the Shaula ref. Fang lists runs on every request, so no restart is needed. Add the `SHAULA_DATA_TOKEN` secret (a read-only token) if `rachzy/shaula` is private. By hand: `make fang-train RUN_ID=... CONFIG=... SKIP_EVAL=1`. The config dropdown comes from `fang configs`: after bumping Fang run `make sync-configs` (a PR check fails when the list is stale).

## TLS

The prod proxy is Caddy. Point a DNS A record at the VPS, open ports 80 and 443, and add to `.env`:

```sh
SITE_ADDRESS=your.domain.com
ANTARES_ALLOWED_ORIGINS=https://your.domain.com
```

Caddy then gets and renews a Let's Encrypt certificate on its own (kept in the `antares-caddy-data` volume). Without `SITE_ADDRESS` it serves plain HTTP on port 80.
