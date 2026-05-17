package api

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"bookshelf/internal/pdfstream"
	"bookshelf/internal/scanner"
	"bookshelf/internal/store"
)

type Server struct {
	store   *store.Store
	scanner *scanner.Scanner
	log     *slog.Logger
}

// libraryDirNow reads the current configured library dir from the store; returns
// "" on any error (treated as "not configured").
func (s *Server) libraryDirNow(ctx context.Context) string {
	dir, err := s.store.GetLibraryDir(ctx)
	if err != nil {
		s.log.Error("read library_dir", "err", err)
		return ""
	}
	return dir
}

// New constructs the HTTP handler for the public API plus PDF streaming.
//
// Routing note: chi's "*" catch-all is greedy, and the spec requires both
// "/api/books/{path...}" and "/api/books/{path...}/progress" style routes.
// We register a single catch-all handler at /api/books/* and dispatch on the
// trailing path suffix (/progress, /bookmarks) inside bookSubRoute.
//
// webFS is the embedded SPA filesystem; pass nil to disable the SPA fallback.
func New(st *store.Store, sc *scanner.Scanner, webFS fs.FS, log *slog.Logger) http.Handler {
	s := &Server{store: st, scanner: sc, log: log}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(slogRequestLogger(log))
	r.Use(middleware.Recoverer)

	r.Get("/healthz", s.healthz)

	r.Route("/api", func(r chi.Router) {
		r.Get("/library", s.getLibrary)
		r.Get("/books/*", s.bookSubRoute)
		r.Put("/books/*", s.bookSubRoute)
		r.Post("/books/*", s.bookSubRoute)
		r.Delete("/bookmarks/{id}", s.deleteBookmark)
		r.Patch("/notes/{id}", s.patchNote)
		r.Delete("/notes/{id}", s.deleteNote)
		r.Post("/scan", s.postScan)
		r.Get("/scan", s.getScan)
		r.Post("/upload", s.uploadBook)
		r.Get("/settings", s.getSettings)
		r.Put("/settings", s.putSettings)

		r.Get("/collections", s.listCollections)
		r.Post("/collections", s.createCollection)
		r.Patch("/collections/{id}", s.patchCollection)
		r.Delete("/collections/{id}", s.deleteCollection)
		r.Post("/collections/{id}/books", s.addBookToCollection)
		r.Delete("/collections/{id}/books/*", s.removeBookFromCollection)
	})

	// pdfstream resolves the library dir per-request via a closure over the store.
	pdfHandler := pdfstream.Handler(func() string {
		return s.libraryDirNow(context.Background())
	}, log)
	r.Get("/books/*", func(w http.ResponseWriter, req *http.Request) {
		// strip the /books prefix so the handler sees only the relative book path.
		// chi's wildcard preserves percent-encoded sequences on some versions, so
		// decode defensively before passing to the filesystem.
		rel := chi.URLParam(req, "*")
		if decoded, err := decodeIfEncoded(rel); err == nil {
			rel = decoded
		}
		req2 := req.Clone(req.Context())
		req2.URL.Path = "/" + rel
		pdfHandler.ServeHTTP(w, req2)
	})

	if webFS != nil {
		r.NotFound(spaHandler(webFS))
	}

	return r
}

// spaHandler serves static SPA assets from webFS and falls back to index.html
// for any unmatched path, enabling vue-router history mode.
func spaHandler(webFS fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(webFS))
	return func(w http.ResponseWriter, r *http.Request) {
		// the SPA fallback must not shadow API / asset routes; any unmatched
		// request under those prefixes is a real 404.
		p := r.URL.Path
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/books/") || p == "/healthz" {
			writeError(w, http.StatusNotFound, codeNotFound, "not found")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		clean := path.Clean(strings.TrimPrefix(p, "/"))
		if clean == "." || clean == "/" {
			serveIndex(w, r, webFS)
			return
		}
		f, err := webFS.Open(clean)
		if err != nil {
			serveIndex(w, r, webFS)
			return
		}
		_ = f.Close()
		fileServer.ServeHTTP(w, r)
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request, webFS fs.FS) {
	data, err := fs.ReadFile(webFS, "index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
	_ = r
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// bookSubRoute splits the chi wildcard into a book path + optional sub-resource
// (/progress, /bookmarks).
func (s *Server) bookSubRoute(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "*")
	bookPath, sub := splitBookSub(raw)
	if bookPath == "" {
		writeError(w, http.StatusBadRequest, codeBadRequest, "missing book path")
		return
	}
	// chi already URL-decodes URLParam for path params, but the wildcard returns the
	// raw path-encoded form on some versions; be defensive.
	if decoded, err := decodeIfEncoded(bookPath); err == nil {
		bookPath = decoded
	}

	switch r.Method {
	case http.MethodGet:
		switch sub {
		case "":
			s.getBook(w, r, bookPath)
		case "progress":
			s.getProgress(w, r, bookPath)
		case "bookmarks":
			s.listBookmarks(w, r, bookPath)
		case "notes":
			s.listNotes(w, r, bookPath)
		default:
			writeError(w, http.StatusNotFound, codeNotFound, "unknown route")
		}
	case http.MethodPut:
		if sub == "progress" {
			s.putProgress(w, r, bookPath)
			return
		}
		if sub == "status" {
			s.putStatus(w, r, bookPath)
			return
		}
		if sub == "hidden" {
			s.putHidden(w, r, bookPath)
			return
		}
		writeError(w, http.StatusMethodNotAllowed, codeBadRequest, "method not allowed")
	case http.MethodPost:
		if sub == "bookmarks" {
			s.postBookmark(w, r, bookPath)
			return
		}
		if sub == "notes" {
			s.postNote(w, r, bookPath)
			return
		}
		writeError(w, http.StatusMethodNotAllowed, codeBadRequest, "method not allowed")
	default:
		writeError(w, http.StatusMethodNotAllowed, codeBadRequest, "method not allowed")
	}
}

func splitBookSub(raw string) (bookPath, sub string) {
	for _, s := range []string{"progress", "bookmarks", "notes", "status", "hidden"} {
		suffix := "/" + s
		if strings.HasSuffix(raw, suffix) {
			return strings.TrimSuffix(raw, suffix), s
		}
	}
	return raw, ""
}
