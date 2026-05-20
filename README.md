# Bookshelf

Bookshelf is a self-hosted PDF library and reader built around a single Go
server and an embedded Vue frontend.

## Overview

The project exists because the current self-hosted book/comic reader space often
forces awkward trade-offs between usability, clean architecture, and deployability.
Bookshelf starts small on purpose: one binary, one UI, one personal library, and
room to grow into a stronger server-first reading platform over time.

## Installation

Prerequisites: Go 1.25+, Node 20+.

- Single-user, no built-in auth
- PDF-only today
- One Go binary serving the API and SPA
- Folder-derived scan collections plus manual collections
- Reading progress, bookmarks, notes, hidden books, and theme support

Because there is no auth layer yet, treat Bookshelf as a private-network service:
use it behind Tailscale or an authenticated reverse proxy rather than exposing it
directly to the public internet.

## Usage

Runtime environment variables:

| Var | Required | Default | Notes |
|---|---|---|---|
| `BOOKSHELF_DB_PATH` | yes | — | Path to the SQLite database file |
| `BOOKSHELF_LIBRARY_DIR` | no | — | Bootstrap default for the library directory |
| `BOOKSHELF_LISTEN` | no | `:19320` | Bind address |

```bash
make build                 # builds frontend, then bin/bookshelf
make image TAG=v0.1.0     # builds the container image
go run ./cmd/bookshelf    # runs the backend directly
```

## Development

Prerequisites: Go 1.25+ and Node 20+.

```bash
./scripts/dev.sh start
xdg-open http://localhost:19321 2>/dev/null || true
./scripts/dev.sh stop
```

The dev helper starts:

- the Go server on `:19320`
- the Vite dev server on `:19321`

On first run, open `/settings` in the UI and configure the library directory.
Until then, the API reports that no library is configured.

## Documentation

- [ARCHITECTURE.md](ARCHITECTURE.md) - system overview and component boundaries
- [CONTRIBUTING.md](CONTRIBUTING.md) - workflow, testing, and PR expectations
- [AGENTS.md](AGENTS.md) - concise repo guidance for coding agents
- [docs/README.md](docs/README.md) - human-facing docs index
- [docs/reference/rest-api.md](docs/reference/rest-api.md) - REST API reference
- [docs/operations/deployment.md](docs/operations/deployment.md) - deployment notes
- [docs/adr/](docs/adr/) - architecture decision records

Planning and spec-driven design artifacts stay under `spec/`. Human-facing
operational and contributor documentation lives under `docs/`.

## License

Bookshelf is licensed under AGPL-3.0. See [LICENSE](LICENSE).
