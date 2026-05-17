# REST API

Base URL: same origin as the SPA. All JSON. No auth.

Conventions:
- `{path}` is the **URL-encoded relative path** of a book (e.g. `Fiction%2FDune%2FDune.pdf`).
- Timestamps are RFC 3339 UTC strings (`2026-05-17T12:34:56Z`).
- Errors use the envelope `{ "error": { "code": "...", "message": "..." } }`.
- Unknown JSON fields on input are ignored; missing required fields → `400 bad_request`.

---

## Health

### `GET /healthz`
**200**
```json
{ "status": "ok" }
```

---

## Library

### `GET /api/library`
Returns the full catalog grouped by category. Soft-removed books are omitted.

**200**
```json
{
  "categories": [
    {
      "name": "Fiction",
      "books": [
        {
          "path": "Fiction/Dune/Dune.pdf",
          "title": "Dune",
          "category": "Fiction",
          "sizeBytes": 4823104,
          "progress": {
            "currentPage": 42,
            "totalPages": 688,
            "percent": 6.1,
            "lastReadAt": "2026-05-16T20:11:03Z",
            "currentlyReading": false,
            "status": "in_progress"
          },
          "bookmarkCount": 3,
          "collectionIds": [1, 4],
          "status": "in_progress",
          "hidden": false
        }
      ]
    }
  ],
  "collections": [
    { "id": 1, "name": "Fiction", "parentId": null, "source": "scan", "folderPath": "Fiction" },
    { "id": 4, "name": "Favorites", "parentId": null, "source": "manual", "folderPath": null }
  ],
  "scannedAt": "2026-05-17T08:00:00Z",
  "libraryConfigured": true
}
```

Notes:
- `progress` is `null` if the book has never been opened.
- `percent` is server-computed: `currentPage / totalPages * 100`, or `0` when `totalPages == 0`.
- `libraryConfigured` is `false` when no library directory is configured. In that case
  `categories` is `[]` and `scannedAt` is the zero time.
- `status` is one of `never_started`, `currently_reading`, `in_progress`, `completed`.
  Computed server-side from the progress row (see `PUT /api/books/{path}/status`).
- `collectionIds` lists every collection the book belongs to. Each book carries the IDs;
  the flat `collections` list lets the frontend build the tree via `parentId`.

---

## Book

### `GET /api/books/{path}`
Single book's metadata + progress + bookmarks (one round-trip for the reader).

**200**
```json
{
  "path": "Fiction/Dune/Dune.pdf",
  "title": "Dune",
  "category": "Fiction",
  "sizeBytes": 4823104,
  "fingerprint": "sha256:9f86d0…",
  "addedAt": "2026-04-01T09:12:00Z",
  "progress": {
    "currentPage": 42, "totalPages": 688, "percent": 6.1, "lastReadAt": "…",
    "currentlyReading": false, "status": "in_progress"
  },
  "bookmarks": [
    { "id": 7, "page": 100, "label": "Arrakis", "createdAt": "2026-05-10T18:00:00Z" }
  ],
  "collectionIds": [1, 4],
  "hidden": false
}
```

**404** if not in catalog or soft-removed.

---

## Progress

### `GET /api/books/{path}/progress`
**200**
```json
{
  "currentPage": 42, "totalPages": 688, "percent": 6.1,
  "lastReadAt": "2026-05-16T20:11:03Z",
  "currentlyReading": false, "status": "in_progress"
}
```
Returns `{"currentPage":1,"totalPages":0,"percent":0,"lastReadAt":null,"currentlyReading":false,"status":"never_started"}` if never set.

### `PUT /api/books/{path}/progress`
Upsert. Sets `lastReadAt = now()` server-side.

**Request**
```json
{ "currentPage": 50, "totalPages": 688 }
```
- `currentPage`: int ≥ 1, ≤ `totalPages` (when `totalPages > 0`).
- `totalPages`: int ≥ 0. Frontend sends it once PDF.js reports it; server keeps the max seen value.

**200** — same shape as `GET`.

**Errors**: `400 bad_request` if values are negative or `currentPage > totalPages`.

---

## Reading status

### `PUT /api/books/{path}/status`
Sets the per-book "currently reading" flag. Upserts a progress row with defaults
(`currentPage=1`, `totalPages=0`) if none exists.

**Request**
```json
{ "currentlyReading": true }
```

**200** — full `ProgressDTO` including the computed `status`:
```json
{
  "currentPage": 1, "totalPages": 0, "percent": 0, "lastReadAt": null,
  "currentlyReading": true, "status": "currently_reading"
}
```

**404** if the book is unknown or soft-removed.

The `status` field is one of:
| Value                | When                                                                 |
|----------------------|----------------------------------------------------------------------|
| `never_started`      | No progress row, OR `currentPage <= 1 && lastReadAt == null`         |
| `currently_reading`  | `currentlyReading == true` (overridden by completion)                |
| `in_progress`        | Has progress activity but isn't completed and isn't actively reading |
| `completed`          | `totalPages > 0 && currentPage >= totalPages` — wins over everything |

The `hidden` boolean on book DTOs indicates whether the book is hidden from the
default library view. Hidden books are still returned by `GET /api/library`; the
client filters them out unless the user explicitly requests the hidden view.

---

## Visibility

### `PUT /api/books/{path}/hidden`
Sets the per-book `hidden` flag.

**Request**
```json
{ "hidden": true }
```

**200** — full `BookDTO` (same shape as `GET /api/books/{path}`), reflecting the
new `hidden` value.

**404** if the book is unknown or soft-removed.

---

## Collections

Collections are tagged groupings. A book may belong to multiple. Collections form a tree
via `parentId`. The flat `collections` array on `GET /api/library` lets the frontend
reconstruct the tree.

Two sources:
- `scan` — created automatically per directory under the library root; identity is the
  relative folder path. Refreshed on every scan. **Cannot be renamed, reparented, or deleted via API.**
- `manual` — user-created via API. Free-form, persists across scans.

### `GET /api/collections`
**200** — flat array of `CollectionDTO`:
```json
[
  { "id": 1, "name": "Fiction", "parentId": null, "source": "scan", "folderPath": "Fiction" },
  { "id": 4, "name": "Favorites", "parentId": null, "source": "manual", "folderPath": null }
]
```

### `POST /api/collections`
Create a manual collection.

**Request**
```json
{ "name": "Favorites", "parentId": null }
```
- `name`: non-empty string. Must be unique (case-insensitive) within `parentId`.
- `parentId`: optional collection id. May reference a `scan` or `manual` collection.

**201** — `CollectionDTO`.

**400** — empty/duplicate name, or unknown parent.

### `PATCH /api/collections/{id}`
Rename and/or reparent a manual collection.

**Request**
```json
{ "name": "Favs", "changeParent": true, "parentId": 7 }
```
- `name` — optional new name.
- `changeParent` — set `true` to alter the parent. When `true`, `parentId` is the new
  parent (or `null` to clear). When `false`/absent, the parent is left alone. (Using
  `changeParent` instead of distinguishing `**int64` keeps the wire JSON plain.)

**200** — updated `CollectionDTO`.

**409** if the collection is `scan`-source.

**400** on validation errors (empty name, duplicate, cycle).

### `DELETE /api/collections/{id}`
**204** on success. Cascades to descendants and to `book_collections` rows.

**409** if the collection is `scan`-source.

### `POST /api/collections/{id}/books`
Add a book to a collection. Idempotent.

**Request**
```json
{ "path": "Fiction/Dune.pdf" }
```

**201** on success (no body). **404** if the book or collection is unknown.

### `DELETE /api/collections/{id}/books/{path...}`
Remove the link.

**204** on success. **404** if the link did not exist.

---

## Bookmarks

### `GET /api/books/{path}/bookmarks`
**200**
```json
[
  { "id": 7, "page": 100, "label": "Arrakis", "createdAt": "2026-05-10T18:00:00Z" },
  { "id": 9, "page": 220, "label": null,      "createdAt": "2026-05-12T09:30:00Z" }
]
```
Ordered by `page ASC, id ASC`.

### `POST /api/books/{path}/bookmarks`
**Request**
```json
{ "page": 220, "label": "Sandworm scene" }
```
- `page`: int ≥ 1, required.
- `label`: string ≤ 200 chars, optional.

**201**
```json
{ "id": 9, "page": 220, "label": "Sandworm scene", "createdAt": "2026-05-17T12:34:56Z" }
```

### `DELETE /api/bookmarks/{id}`
**204** on success. **404** if id unknown.

(Note: deletion uses the global id, not the book path, so the URL is shorter.)

---

## Notes

Per-page free-text notes anchored to a book. Notes are returned inline on
`GET /api/books/{path}` and surface in the reader's gutter / side panel.

Notes may optionally carry normalized coordinates `(x, y)` in `[0, 1]` relative to
the page (origin top-left). Both values must be present together (or both omitted).
A note without coordinates renders in the page-side gutter; a note with coordinates
is anchored to a specific spot on the page.

### `GET /api/books/{path}/notes`
**200**
```json
[
  { "id": 3, "page": 12, "body": "fremen lore",
    "createdAt": "2026-05-17T08:00:00Z", "updatedAt": "2026-05-17T08:00:00Z" },
  { "id": 4, "page": 12, "body": "anchored", "x": 0.25, "y": 0.4,
    "createdAt": "2026-05-17T09:00:00Z", "updatedAt": "2026-05-17T09:00:00Z" }
]
```
Ordered by `page ASC, id ASC`. `x` and `y` are omitted from the JSON when the note
is unanchored.

### `POST /api/books/{path}/notes`
**Request**
```json
{ "page": 12, "body": "fremen lore", "x": 0.25, "y": 0.4 }
```
- `page`: int ≥ 1, required.
- `body`: non-empty after trim, ≤ 10000 chars.
- `x`, `y`: optional floats in `[0, 1]`. Must both be present or both omitted.

**201** — `NoteDTO`.

**400** if body is empty/too long, page < 1, or x/y out of range / only one provided.
**404** if book unknown.

### `PATCH /api/notes/{id}`
**Request**
```json
{ "body": "updated text", "page": 42, "x": 0.1, "y": 0.2 }
```
Or to clear the anchor and return the note to the gutter:
```json
{ "clearPosition": true }
```
- Any of `body`, `page`, `x`, `y` may be omitted; missing fields are left unchanged.
- Pointer-only x/y is ambiguous between "unchanged" and "clear"; pass
  `"clearPosition": true` to explicitly clear x/y back to null.
- When updating coordinates, both `x` and `y` must be supplied together.

**200** — updated `NoteDTO`.

**400** on validation failure. **404** if id unknown.

### `DELETE /api/notes/{id}`
**204** on success. **404** if id unknown.

---

## Scanning

### `POST /api/scan`
Triggers a library rescan. Non-blocking: returns immediately, scan runs in background.

**202**
```json
{ "started": true, "scanId": "2026-05-17T12:34:56Z" }
```

**409** if a scan is already in progress:
```json
{ "error": { "code": "conflict", "message": "scan already in progress" } }
```

**400** if no library directory is configured:
```json
{ "error": { "code": "bad_request", "message": "library not configured" } }
```

### `POST /api/upload`
Upload a PDF into the library. Multipart form with fields:

- `file` (required) — the PDF blob. Capped at **200 MiB** (request body limit).
- `folder` (optional) — slash-separated relative folder under the library root
  (e.g. `Fiction/Sci-Fi`). Must be relative; `..` segments and absolute paths
  are rejected.
- `collectionIds` (optional, repeated) — collection ids to link the new book to.
  Repeat the field for multiple values: `collectionIds=1&collectionIds=2`.
  Comma-separated values within a single occurrence are also accepted.

Behavior:
- The file is streamed to a sibling temp file and atomically renamed into place,
  so partial writes never appear under the library root.
- After the write succeeds the server synchronously indexes the file
  (fingerprint + upsert), ensures the scan-source collection chain for `folder`
  exists, and links the new book to the requested `collectionIds`.

**201** — returns the same shape as `GET /api/books/{path}`:
```json
{
  "path": "Fiction/Sci-Fi/Dune.pdf",
  "title": "Dune",
  "category": "Fiction",
  "sizeBytes": 4823104,
  "fingerprint": "sha256:…",
  "addedAt": "2026-05-17T12:34:56Z",
  "progress": null,
  "bookmarks": [],
  "collectionIds": [3, 7]
}
```

**400 `bad_request`** — library not configured, missing/empty file, non-PDF
extension, or an invalid `folder` (absolute, contains `..`, contains illegal
characters, escapes the library root).

**409 `conflict`** — a file already exists at the target path; uploads never
overwrite silently.

**500 `internal`** — filesystem failure (e.g. EROFS, permission denied). The
client surfaces the server message as-is.

### `GET /api/scan`
Status of the last/current scan.

**200**
```json
{
  "running": false,
  "startedAt": "2026-05-17T12:34:56Z",
  "finishedAt": "2026-05-17T12:34:59Z",
  "added": 2,
  "updated": 0,
  "removed": 1,
  "error": null
}
```

---

## Settings

### `GET /api/settings`
Returns runtime settings. Currently only the library directory.

**200**
```json
{ "libraryDir": "/srv/books" }
```
`libraryDir` is the empty string when no library is configured.

### `PUT /api/settings`
Update runtime settings.

**Request**
```json
{ "libraryDir": "/srv/books" }
```
- `libraryDir`: absolute path on the server. Must exist and be a directory.
  Pass the empty string to clear it.

**200** — echoes the new settings.

**400** if the path does not exist or is not a directory:
```json
{ "error": { "code": "bad_request", "message": "invalid setting: ..." } }
```

---

## PDF bytes (non-`/api`)

### `GET /books/{path}`
Returns the raw PDF. Implemented via `http.ServeContent`, so:
- `Range`, `If-Modified-Since`, and `If-None-Match` are honored.
- `Content-Type: application/pdf`.
- 200 for full body; 206 for satisfiable Range; 416 for unsatisfiable.

PDF.js is configured to fetch from this URL and will issue partial-range requests on its own.

---

## Status code summary

| Code | When                                                                 |
|------|----------------------------------------------------------------------|
| 200  | Successful GET / PUT                                                 |
| 201  | Bookmark created, book uploaded                                      |
| 202  | Scan accepted                                                        |
| 204  | Bookmark deleted                                                     |
| 206  | Partial PDF (Range)                                                  |
| 400  | Malformed JSON or invalid field values                               |
| 404  | Unknown book or bookmark                                             |
| 409  | Scan already running, upload target already exists                   |
| 416  | Range not satisfiable                                                |
| 500  | Unhandled server error (logged with `slog.Error`)                    |
