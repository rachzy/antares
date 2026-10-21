# Deploying to a VPS

Every push to `main` runs `.github/workflows/deploy.yml`: it rsyncs the repository to the VPS over SSH and runs `make build-up ENV=prod` there.

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
- `$VPS_DEPLOY_PATH/.env` (kept across deploys) with at least `BIND=0.0.0.0` to expose the proxy publicly, and `ANTARES_ALLOWED_ORIGINS=http://<host>` matching the URL users open. `PROXY_PORT` defaults to `80`.

## Model bundles

Copy a bundle directory to the VPS (e.g. `scp -r bundle vps:/opt/antares-models/v1`), then on the VPS:

```sh
make fang-models MODELS=/opt/antares-models/v1
```

This writes `FANG_MODELS_DIR` to `.env`, bind-mounts it at `/var/lib/fang/models` and recreates only the `fang` container, so the other services keep running. Without it, Fang uses the empty `antares-fang-models` volume.
