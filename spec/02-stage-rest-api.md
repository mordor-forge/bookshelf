# Stage 2 — REST API

Goal: expose the full JSON API and PDF byte streaming. Frontend can now be built against a
running backend.

## Deliverables
- `internal/api/router.go` mounts:
  - `/api/*` — JSON endpoints (see [rest-api.md](rest-api.md))
  - `/books/*` — PDF byte streaming
- `internal/pdfstream` — uses `http.ServeContent` so Range requests, ETag, and conditional GETs
  work for PDF.js streaming.
- Request logging middleware (`slog`), recovery middleware, JSON error envelope.
- DTOs separate from `library` domain types (so we can evolve the wire format independently).

## Endpoint surface (summary; full detail in `rest-api.md`)
- `GET    /api/library`
- `GET    /api/books/{path...}`
- `GET    /api/books/{path...}/progress`
- `PUT    /api/books/{path...}/progress`
- `GET    /api/books/{path...}/bookmarks`
- `POST   /api/books/{path...}/bookmarks`
- `DELETE /api/bookmarks/{id}`
- `PUT    /api/books/{path...}/status`
- `GET    /api/collections`
- `POST   /api/collections`
- `PATCH  /api/collections/{id}`
- `DELETE /api/collections/{id}`
- `POST   /api/collections/{id}/books`
- `DELETE /api/collections/{id}/books/{path...}`
- `POST   /api/scan`
- `GET    /api/settings`
- `PUT    /api/settings`
- `GET    /healthz`
- `GET    /books/{path...}` — raw PDF bytes (Range-enabled)

## Path handling
- `{path...}` is the relative library path (URL-encoded slashes preserved by chi `*`).
- Server validates the resolved absolute path is still under `libraryDir` (defense against `..`).
- 404 if the book row is missing or `removed_at` is set.

## Error envelope
```json
{ "error": { "code": "not_found", "message": "book not found" } }
```
Codes: `bad_request`, `not_found`, `conflict`, `internal`.

## Exit criteria
- `curl` walkthrough of every endpoint succeeds against a seeded library.
- Range request (`Range: bytes=0-1023`) returns 206 with correct slice.
- Concurrent `POST /api/scan` returns one 202 and one 409.
- Integration tests under `internal/api/*_test.go` using `httptest` + in-memory SQLite.

## Out of scope
- Frontend (Stage 3).
- AuthN/Z (none, ever).
