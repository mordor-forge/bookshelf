# 00 — Overview

## Goal
A single-binary Go server that scans a directory of PDFs, exposes a JSON REST API for library
metadata + per-book reading state, and serves an embedded Vue 3 SPA that renders PDFs via PDF.js.

## High-level architecture

```
                ┌─────────────────────────────────────────┐
                │            Single Go binary             │
                │                                         │
  HTTP ──▶ chi router ──▶ ┌── /api/*    handlers ──▶ store (sqlx + modernc/sqlite)
                │         ├── /books/*  PDF byte streamer ──▶ /data/books (RO)
                │         └── /*        embedded SPA (go:embed dist)
                │                                         │
                │   scanner goroutine ─▶ store (reconcile)│
                └─────────────────────────────────────────┘
```

## Module layout

```
nappa/
├── cmd/bookshelf/main.go        # entrypoint: flags, wire deps, http.ListenAndServe
├── internal/
│   ├── config/                  # env + flag parsing
│   ├── store/                   # sqlx wrappers, migrations, queries
│   │   ├── migrations/*.sql
│   │   ├── books.go
│   │   ├── progress.go
│   │   └── bookmarks.go
│   ├── scanner/                 # filesystem walker + reconciler
│   ├── library/                 # domain types (Book, Category, Progress, Bookmark)
│   ├── api/                     # chi handlers, request/response DTOs
│   │   ├── router.go
│   │   ├── books.go
│   │   ├── progress.go
│   │   ├── bookmarks.go
│   │   └── scan.go
│   ├── pdfstream/               # range-request capable file streamer for /books/*
│   └── web/                     # go:embed of frontend dist
├── web/                         # Vue 3 source (vite project)
├── spec/
├── Dockerfile
└── go.mod
```

## Target platform
- **Linux/amd64** — container base is `gcr.io/distroless/static-debian12`.
- Build: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build`.
- Deployment target is operator's choice (k8s / docker / systemd / bare). The binary is
  self-contained and configured entirely via environment variables.
- No CGO, no shared libs, no glibc dependency — runs on `scratch`/`distroless/static`.
- All path handling internally uses `path/filepath` (OS-correct during the walk) but the value
  persisted in DB and returned via the API is always slash-separated (`filepath.ToSlash`).
- Linux signal handling: graceful shutdown on `SIGTERM`/`SIGINT` via `signal.NotifyContext`,
  needed for clean k8s pod terminations.
- Library volume is read-only (`hostPath: /data/books` mounted RO); scanner must never write
  to it. DB volume (`/data/db`) is the only writable mount.

## Key technical choices

| Concern               | Choice                                | Reason                                                 |
|-----------------------|---------------------------------------|--------------------------------------------------------|
| HTTP router           | `go-chi/chi/v5`                       | stdlib-style, middleware, route groups                 |
| DB driver             | `modernc.org/sqlite`                  | pure Go, no CGO → static binary, scratch image works   |
| DB access             | `jmoiron/sqlx`                        | thin layer over `database/sql`, struct scanning        |
| Migrations            | embedded `.sql` files, applied on boot| zero external tool                                     |
| PDF serving           | `http.ServeContent`                   | gets Range requests, ETag, If-Modified-Since for free  |
| Frontend embedding    | `embed.FS`                            | single binary                                          |
| Logging               | `log/slog` (stdlib)                   | structured, no dep                                     |
| Tests                 | stdlib `testing` + `testify/require`  | minimal                                                |

## Identifying a book
Books are keyed by their **relative path** from the library root (e.g. `Fiction/Dune/Dune.pdf`).
Path is the stable ID — moving a file = new book. A `sha256` of the first 64 KiB is stored as a
secondary fingerprint so a future "detect moved" feature can run without a schema change.

## Concurrency
- One scanner goroutine, triggered on boot and via `POST /api/scan`. Guarded by a mutex so two
  scans never run concurrently; the second call returns `409 Conflict`.
- All DB writes go through `store`, which holds a single `*sqlx.DB` (SQLite serializes writers).

## See also
- [01-stage-foundation.md](01-stage-foundation.md)
- [02-stage-rest-api.md](02-stage-rest-api.md)
- [03-stage-frontend.md](03-stage-frontend.md)
- [04-stage-deployment.md](04-stage-deployment.md)
- [rest-api.md](rest-api.md)
