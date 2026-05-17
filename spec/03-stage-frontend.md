# Stage 3 — Frontend

Goal: Vue 3 SPA, embedded via `go:embed`, providing library browser + PDF reader.

## Deliverables
- `web/` Vite + Vue 3 + TS project.
- Routes:
  - `/` — library browser (categories → books, with progress %, last-read).
  - `/read/:path` — PDF reader (PDF.js).
  - `/settings` — configure the library directory (runtime setting stored in DB).
- Composable `useBook(path)` — fetches metadata + progress + bookmarks; debounced writes.
- Progress write strategy: debounce 500 ms; final flush on `beforeunload`.
- Bookmarks UI: list in a side panel, add at current page, delete inline.
- Build output `web/dist` is consumed by `internal/web/embed.go`:
  ```go
  //go:embed all:dist
  var Dist embed.FS
  ```
- SPA fallback: any non-`/api`, non-`/books`, non-`/healthz` GET returns `index.html`.

## PDF.js wiring
- Use the `pdfjs-dist` npm package; load worker via Vite `?worker` import.
- Source URL: `/books/<encoded path>` — PDF.js issues Range requests directly to the Go server.
- Emit `pagechanging` → composable → debounced `PUT /api/books/.../progress`.

## Build integration
- `make web` runs `npm ci && npm run build` in `web/`.
- `make build` runs `make web` then `go build -o bookshelf ./cmd/bookshelf`.
- Dev workflow: Vite dev server on `:19321` proxies `/api` + `/books` to `:19320`.

## Exit criteria
- `make build` produces a single binary that, when run, serves a working SPA at `:19320`.
- Reading a book and refreshing restores the last page.
- Adding/removing bookmarks survives a restart.

## Out of scope
- PWA / offline.
- Server-side thumbnails (could be added later as `GET /api/books/.../thumbnail`).
