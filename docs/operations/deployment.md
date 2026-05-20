# Deployment

Bookshelf is currently packaged as a single Go binary plus an embedded frontend.
It is intended to be easy to run on a private host, container platform, or small
cluster without extra moving parts.

## Current deployment assumptions

- built and tested primarily for Linux
- SQLite is the current database backend
- the service is single-user and has no built-in auth
- private-network deployment is recommended today

Use Tailscale or an authenticated reverse proxy if the service needs remote
access. Do not treat the current app as internet-facing by default.

## Runtime configuration

| Var | Required | Default | Notes |
|---|---|---|---|
| `BOOKSHELF_DB_PATH` | yes | — | Writable path to the SQLite database file |
| `BOOKSHELF_LIBRARY_DIR` | no | — | Bootstrap default for the library directory on first boot |
| `BOOKSHELF_LISTEN` | no | `:19320` | Bind address |

## Local build and image

```bash
make build
make image TAG=dev
```

`make build` currently produces a Linux/amd64 binary at `bin/bookshelf`.

## Container notes

- the image is multi-stage and based on a distroless runtime
- the SQLite database path must be writable
- if uploads are enabled, the library path must also be writable
- if uploads are not needed, the library can be mounted read-only

## Recommended mounts

- library path: persistent storage containing the PDF tree
- database path: persistent writable storage for SQLite

## Health and operations

- health check: `GET /healthz`
- background scans can be triggered with `POST /api/scan`
- the service handles `SIGINT` and `SIGTERM` for graceful shutdown

## Future work

The deployment roadmap in issue `#1` includes:

- richer CI/CD and release automation
- Docker Compose examples
- Helm chart support
- Argo CD guidance
- PostgreSQL as a supported deployment mode
