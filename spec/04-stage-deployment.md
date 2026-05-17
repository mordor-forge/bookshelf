# Stage 4 — Packaging & Deployment

Goal: ship a single static binary and a container image. The choice of where to run it
(k8s, docker, systemd, NAS app) is left to the operator.

## Container
- Multi-stage Dockerfile:
  1. `node:20-alpine` — `npm ci && npm run build` in `web/`.
  2. `golang:1.22-alpine` — `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w"`.
  3. `gcr.io/distroless/static-debian12:nonroot` — copy the binary; runs as `nonroot`.
- The binary has no runtime deps (pure-Go SQLite, no glibc, no CGO).
- Single-arch `linux/amd64`.

## Runtime contract
The image is configured entirely via environment variables (and exposes one port):

| Var                       | Required | Default  | Notes                                    |
|---------------------------|----------|----------|------------------------------------------|
| `BOOKSHELF_LIBRARY_DIR`   | no       | —        | **Bootstrap default only**: seeds the `library_dir` setting on first boot if no DB row exists. Otherwise ignored. The library directory is a runtime setting edited via the Settings page. |
| `BOOKSHELF_DB_PATH`       | yes      | —        | Path to SQLite file (parent must be RW). |
| `BOOKSHELF_LISTEN`        | no       | `:19320`  | `host:port` to bind.                     |

Health endpoint: `GET /healthz` → 200. Use it for liveness/readiness wherever you run it.

Signals: `SIGTERM`/`SIGINT` trigger graceful shutdown (stop accepting, drain in-flight, close DB).

## Reference examples

### `docker run`
```sh
docker run -d --name bookshelf \
  -p 19320:19320 \
  -v /srv/books:/books:ro \
  -v bookshelf-db:/db \
  -e BOOKSHELF_LIBRARY_DIR=/books \
  -e BOOKSHELF_DB_PATH=/db/bookshelf.db \
  ghcr.io/<you>/bookshelf:<tag>
```

### Kubernetes (reference, not prescriptive)
- 1-replica `Deployment`, `RollingUpdate` with `maxUnavailable: 0` (SQLite is single-writer).
- `livenessProbe` + `readinessProbe` → `GET /healthz`.
- Volumes:
  - Library: `hostPath` / `nfs` / `csi` mount, RO at the path passed in `BOOKSHELF_LIBRARY_DIR`.
  - DB: a small RWO PVC (e.g. 1Gi) mounted at the directory containing `BOOKSHELF_DB_PATH`.
- Expose via whatever ingress you have (nginx, Traefik, Tailscale, etc.).

### systemd
- `ExecStart=/usr/local/bin/bookshelf`
- `Environment=BOOKSHELF_LIBRARY_DIR=... BOOKSHELF_DB_PATH=... BOOKSHELF_LISTEN=:19320`
- `User=` a non-root account with read on the library and write on the DB dir.

## Release flow
1. `make build` → static binary in `./bin/bookshelf`.
2. `make image TAG=<tag>` → builds & tags the container image.
3. `docker push <image>:<tag>` (registry is operator's choice).
4. Operator rolls out via their preferred mechanism.

## Migration considerations
- Migrations run in-process on boot. A bad migration causes the process to exit non-zero, so
  k8s/docker keeps the previous instance if you configured `maxUnavailable: 0` / a healthcheck.
- DB should live on persistent storage; the library can be RO and ephemeral.

## Exit criteria
- A built image runs locally with `docker run` against a local PDF directory and serves the SPA.
- A restart preserves progress + bookmarks (when the DB path is on a persistent volume).
- Dropping a new PDF into the library directory and calling `POST /api/scan` makes it appear.
