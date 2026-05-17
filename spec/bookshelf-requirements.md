# Bookshelf — Requirements

## Stack
- **Server**: Go (1.22+), `chi` router, `sqlx` + `modernc.org/sqlite` (pure-Go, no CGO)
- **Frontend**: Vue 3, TypeScript, Vite
- **Server serves frontend** as static files via `go:embed` (single binary, no separate nginx)
- **PDF rendering**: PDF.js in the browser; server only streams the bytes

## Target platform
- **Primary**: Linux/amd64. Builds produced with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`.
- Deployment target (k8s cluster, bare host, NAS, etc.) is the operator's choice — the server
  is a single static binary that reads two paths from env and serves HTTP on a configurable port.
- Filesystem assumptions are Linux-friendly:
  - Case-sensitive paths.
  - Forward-slash separators stored verbatim in DB.
  - UTF-8 filenames.
- Development on Windows/macOS is possible but not the deployment target; any path handling uses
  `path/filepath` for OS-correct walking and converts to `/`-separated relative paths before
  storing or returning via the API.

## Library
- Books live under a configurable library directory, set at runtime via the Settings page
  (stored in the SQLite `meta` table). `BOOKSHELF_LIBRARY_DIR` is a bootstrap default only:
  on first boot, if the setting is absent and the env var is set, the env var seeds it.
  The server starts cleanly even with no library configured.
- **No required directory layout.** A "book" is any `.pdf` file found anywhere in the tree.
  PDFs only — no comics (`.cbz`/`.cbr`), no EPUB, no other formats.
- Server scans recursively on startup and on demand via API.
- Derived metadata:
  - **Title** = filename without `.pdf` extension.
  - **Category** = first path component relative to the library root if the file is nested
    (e.g. `Fiction/Dune.pdf` → "Fiction"); `"Uncategorized"` for files at the root.
  - Deeper nesting is allowed; only the first component is used for category. The full relative
    path is still the stable ID.
- Scan is idempotent: identifies books by relative path; new/removed files reconcile against DB.

## Features

### Library browser
- List categories and books
- Show last-read timestamp and progress % per book

### PDF reader
- Open PDFs using PDF.js (gives access to page events, unlike native browser renderer)
- Track current page and total pages — persist on every page change

### Per-book state (stored in DB, keyed by relative file path)
- **Progress**: current page, total pages, % complete
- **Last read**: timestamp
- **Bookmarks**: list of `{ page, label?, createdAt }` — add/remove from reader
- **Reading status**: a per-book "currently reading" toggle, combined with progress
  values to derive a unified status enum (`never_started`, `currently_reading`,
  `in_progress`, `completed`). Computed server-side and returned on every progress
  payload; the library list also exposes it per book.

### Collections
- Books can be tagged into **collections**, which form a tree via `parentId`.
- A book may belong to multiple collections.
- Two sources:
  - `scan` collections are created automatically during a library scan, one per
    directory under the library root. They are refreshed on every rescan and cannot
    be renamed, reparented, or deleted via API.
  - `manual` collections are user-created via the API and persist across scans.
- The legacy `category` field (first path component) stays on books for backwards
  compatibility but is no longer the primary grouping in the UI.

### No auth
- Single-user, no login

## Deployment
- Single static Go binary in a `distroless/static` (or `scratch`) image.
- Container expects:
  - `BOOKSHELF_DB_PATH` — path to a writable file for the SQLite DB (required).
  - `BOOKSHELF_LIBRARY_DIR` — optional bootstrap default; if set on first boot, it seeds the
    `library_dir` setting. Otherwise configure it via the Settings page at runtime.
  - `BOOKSHELF_LISTEN` — `host:port` to bind (default `:19320`).
- Operator chooses how to host: k8s Deployment, `docker run`, systemd unit, etc.
- Stage 4 includes example k8s manifests as a reference, not a requirement.

## Non-goals
- Multi-user, auth, sharing
- Server-side PDF rendering / thumbnails (deferred)
- Full-text search inside PDFs (deferred)

## See also
- [spec/00-overview.md](spec/00-overview.md) — architecture & module layout
- [spec/01-stage-foundation.md](spec/01-stage-foundation.md) — Stage 1
- [spec/02-stage-rest-api.md](spec/02-stage-rest-api.md) — Stage 2
- [spec/03-stage-frontend.md](spec/03-stage-frontend.md) — Stage 3
- [spec/04-stage-deployment.md](spec/04-stage-deployment.md) — Stage 4
- [spec/rest-api.md](spec/rest-api.md) — REST API reference
