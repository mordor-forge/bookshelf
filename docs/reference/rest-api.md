# REST API Reference

Base URL: same origin as the frontend. Responses are JSON unless noted otherwise.

## Conventions

- Book paths are URL-encoded relative paths such as `Fiction%2FDune%2FDune.pdf`.
- Timestamps use RFC 3339 UTC strings.
- Errors use the envelope:

```json
{ "error": { "code": "bad_request", "message": "..." } }
```

## Core endpoints

### Health

- `GET /healthz`

### Library and books

- `GET /api/library`
- `GET /api/books/{path}`
- `GET /api/books/{path}/progress`
- `PUT /api/books/{path}/progress`
- `PUT /api/books/{path}/status`
- `PUT /api/books/{path}/hidden`

### Bookmarks and notes

- `GET /api/books/{path}/bookmarks`
- `POST /api/books/{path}/bookmarks`
- `DELETE /api/bookmarks/{id}`
- `GET /api/books/{path}/notes`
- `POST /api/books/{path}/notes`
- `PATCH /api/notes/{id}`
- `DELETE /api/notes/{id}`

### Collections

- `GET /api/collections`
- `POST /api/collections`
- `PATCH /api/collections/{id}`
- `DELETE /api/collections/{id}`
- `POST /api/collections/{id}/books`
- `DELETE /api/collections/{id}/books/{path...}`

Collection rules today:

- `scan` collections are derived from the folder tree
- books may belong to multiple collections
- membership edits for `scan` collections are rejected
- manual collections are the intended place for user-managed grouping

Collection metadata rules are still being tightened. If you are changing rename,
reparent, or delete behavior, verify the current handlers in `internal/api`.

### Scanning and upload

- `POST /api/scan`
- `GET /api/scan`
- `POST /api/upload`

Upload rules today:

- accepts PDFs only
- writes into the configured library root
- optional `collectionIds` can link the uploaded book to manual collections
- upload rejects scan-derived collection IDs

### Settings

- `GET /api/settings`
- `PUT /api/settings`

## Raw book streaming

- `GET /books/{path}`

This endpoint streams the raw PDF with range support via `http.ServeContent`.

## Source of truth

This document is the human-facing API guide. When the docs and code disagree,
check the current handlers and DTOs in:

- `internal/api/router.go`
- `internal/api/handlers.go`
- `internal/api/collections.go`
- `internal/api/upload.go`
- `internal/api/dto.go`
