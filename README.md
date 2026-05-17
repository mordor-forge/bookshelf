# Bookshelf

A self-hosted PDF library and reader.

## Highlights

- Self-hosted, single-user. No accounts, no auth.
- One Go binary serving an embedded Vue 3 SPA.
- PDF.js reader with table of contents, zoom, and two-page spread.
- Collections layered on top of folder structure; folders auto-seed as collections on scan.
- Per-book status: reading, completed, hidden.
- Bookmarks and page-anchored notes.
- Dark, light, and system theme.

## Quick start (dev)

Prerequisites: Go 1.22+, Node 20+.

```
cd web && npm install
./scripts/dev.sh start    # Go on :19320, Vite on :19321
open http://localhost:19321
./scripts/dev.sh stop
```

On first run, configure the library directory via the Settings page
(`/settings` in the UI). Until then the API will report no library.

## Production build

```
make build   # builds frontend then linux/amd64 static binary at bin/bookshelf
```

Or a container image:

```
make image TAG=v0.1.0
```

## Runtime configuration

| Var | Required | Default | Notes |
|---|---|---|---|
| `BOOKSHELF_DB_PATH` | yes | — | path to SQLite file (parent must be writable) |
| `BOOKSHELF_LIBRARY_DIR` | no | — | optional bootstrap default; otherwise set via Settings UI |
| `BOOKSHELF_LISTEN` | no | `:19320` | host:port to bind |

## Project layout

```
cmd/        # main package (bookshelf binary)
internal/   # server, store, scanner, PDF, web embed
web/        # Vue 3 SPA source
scripts/    # dev helpers
spec/       # design docs
```

## Building only the frontend or backend

```
make web                    # frontend only, output to internal/web/dist
go build ./cmd/bookshelf    # backend only, embeds whatever is in internal/web/dist
```

## License

AGPL-3.0. See LICENSE.
