# Stage 1 — Foundation

Goal: a runnable Go binary that scans the library directory, persists the catalog to SQLite,
and exposes a single health endpoint. No reader, no frontend yet.

## Deliverables
- `go.mod` with pinned deps (`chi`, `sqlx`, `modernc.org/sqlite`, `slog` from stdlib).
- `cmd/bookshelf/main.go` — wires config → store → scanner → http server.
- `internal/config` — env-driven config:
  - `BOOKSHELF_LIBRARY_DIR` (optional; **bootstrap default only** — seeds the `meta.library_dir`
    setting on first boot when no row exists, then is ignored. The library directory is otherwise
    a runtime setting edited via the Settings page.)
  - `BOOKSHELF_DB_PATH`     (required; parent directory must exist and be writable)
  - `BOOKSHELF_LISTEN`      (default `:19320`)
- `internal/store` — opens SQLite, applies embedded migrations, exposes typed methods.
- `internal/scanner` — walks library dir, reconciles books into DB, idempotent.
- `internal/library` — domain types (no DB tags, pure Go structs).
- `GET /healthz` returning `{"status":"ok"}`.

## Schema (initial migration `0001_init.sql`)

```sql
CREATE TABLE books (
  path          TEXT PRIMARY KEY,         -- relative path, e.g. "Fiction/Dune/Dune.pdf"
  category      TEXT NOT NULL,
  title         TEXT NOT NULL,
  size_bytes    INTEGER NOT NULL,
  fingerprint   TEXT NOT NULL,            -- sha256 of first 64 KiB
  added_at      DATETIME NOT NULL,
  removed_at    DATETIME                  -- soft-delete: file disappeared on last scan
);

CREATE TABLE progress (
  book_path     TEXT PRIMARY KEY REFERENCES books(path) ON DELETE CASCADE,
  current_page  INTEGER NOT NULL DEFAULT 1,
  total_pages   INTEGER NOT NULL DEFAULT 0,
  last_read_at  DATETIME
);

CREATE TABLE bookmarks (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  book_path     TEXT NOT NULL REFERENCES books(path) ON DELETE CASCADE,
  page          INTEGER NOT NULL,
  label         TEXT,
  created_at    DATETIME NOT NULL
);
CREATE INDEX bookmarks_by_book ON bookmarks(book_path);

CREATE TABLE meta (
  key           TEXT PRIMARY KEY,
  value         TEXT NOT NULL
);
```

`PRAGMA journal_mode=WAL` and `PRAGMA foreign_keys=ON` set on every connection.

## Scanner behaviour
1. Walk the entire `<libraryDir>` tree recursively using `filepath.WalkDir`. Match any file
   whose name ends in `.pdf` (case-insensitive). All other extensions are ignored.
2. For each PDF found:
   - Derive `title` = filename without extension.
   - Derive `category` = first path segment of the relative path if nested, else `"Uncategorized"`.
   - Convert the relative path to slash-separated form (`filepath.ToSlash`) before use as DB key.
   - Compute fingerprint (sha256 of first 64 KiB) using a read-only `os.Open` — the library
     volume is mounted RO on Linux, so anything that would write (chmod, utime) must be avoided.
   - `INSERT OR IGNORE` book; if exists, `UPDATE` size + clear `removed_at`.
3. Books in DB but not on disk: set `removed_at = now()`. Do not hard-delete (preserves bookmarks).
4. Wrapped in a single transaction. Holds `scanner.mu` for the duration.
5. Skip symlinks that escape `libraryDir` (resolve with `filepath.EvalSymlinks`, then verify the
   result has `libraryDir` as a prefix). Linux symlinks are common in book libraries.

## Exit criteria
- `go run ./cmd/bookshelf` against a fixture tree writes the expected rows.
- Re-running produces no diff (idempotent).
- Removing a file marks it `removed_at` but keeps the row.
- `curl localhost:19320/healthz` → 200.
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/bookshelf` succeeds and the resulting
  binary runs on `distroless/static` with the library directory mounted RO.
- `SIGTERM` causes the process to stop accepting connections, drain in-flight requests, close
  the DB, and exit cleanly within the k8s `terminationGracePeriodSeconds` (default 30s).

## Out of scope
- HTTP API for books (Stage 2).
- Frontend (Stage 3).
- Bookmarks/progress endpoints (Stage 2).
