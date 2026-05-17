package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"bookshelf/internal/library"
	"bookshelf/internal/scanner"
	"bookshelf/internal/store"
)

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	dir, err := s.store.GetLibraryDir(r.Context())
	if err != nil {
		s.internal(w, r, "get library_dir", err)
		return
	}
	writeJSON(w, http.StatusOK, SettingsDTO{LibraryDir: dir})
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var req PutSettingsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := s.store.SetLibraryDir(r.Context(), req.LibraryDir); err != nil {
		if errors.Is(err, store.ErrInvalidSetting) {
			writeError(w, http.StatusBadRequest, codeBadRequest, err.Error())
			return
		}
		s.internal(w, r, "set library_dir", err)
		return
	}
	dir, err := s.store.GetLibraryDir(r.Context())
	if err != nil {
		s.internal(w, r, "get library_dir", err)
		return
	}
	writeJSON(w, http.StatusOK, SettingsDTO{LibraryDir: dir})
}

func (s *Server) getLibrary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	books, err := s.store.ListBooks(ctx)
	if err != nil {
		s.internal(w, r, "list books", err)
		return
	}
	counts, err := s.store.BookmarkCounts(ctx)
	if err != nil {
		s.internal(w, r, "bookmark counts", err)
		return
	}
	progMap, err := s.store.ProgressMap(ctx)
	if err != nil {
		s.internal(w, r, "progress map", err)
		return
	}
	collMap, err := s.store.BookCollectionIDs(ctx)
	if err != nil {
		s.internal(w, r, "book collection ids", err)
		return
	}
	collections, err := s.store.ListCollections(ctx)
	if err != nil {
		s.internal(w, r, "list collections", err)
		return
	}

	// group by category, preserving listing order (already sorted by category, title).
	cats := make([]CategoryDTO, 0)
	byName := make(map[string]int)
	for _, b := range books {
		var prog *ProgressDTO
		p, hasRow := progMap[b.Path]
		if hasRow {
			pd := progressToDTO(p, true)
			prog = &pd
		}
		status := library.ComputeStatus(p, hasRow)
		ids := collMap[b.Path]
		if ids == nil {
			ids = []int64{}
		}
		summary := BookSummaryDTO{
			Path:          b.Path,
			Title:         b.Title,
			Category:      b.Category,
			SizeBytes:     b.SizeBytes,
			Fingerprint:   b.Fingerprint,
			Progress:      prog,
			BookmarkCount: counts[b.Path],
			CollectionIDs: ids,
			Status:        string(status),
			Hidden:        b.Hidden,
		}
		idx, ok := byName[b.Category]
		if !ok {
			cats = append(cats, CategoryDTO{Name: b.Category, Books: []BookSummaryDTO{summary}})
			byName[b.Category] = len(cats) - 1
		} else {
			cats[idx].Books = append(cats[idx].Books, summary)
		}
	}

	last := s.scanner.Last()
	scannedAt := last.FinishedAt
	if scannedAt.IsZero() {
		scannedAt = time.Time{}
	}

	dir, err := s.store.GetLibraryDir(r.Context())
	if err != nil {
		s.internal(w, r, "get library_dir", err)
		return
	}
	collDTOs := make([]CollectionDTO, 0, len(collections))
	for _, c := range collections {
		collDTOs = append(collDTOs, collectionToDTO(c))
	}
	writeJSON(w, http.StatusOK, LibraryDTO{
		Categories:        cats,
		Collections:       collDTOs,
		ScannedAt:         scannedAt,
		LibraryConfigured: dir != "",
	})
}

func (s *Server) getBook(w http.ResponseWriter, r *http.Request, path string) {
	ctx := r.Context()
	book, err := s.store.GetBook(ctx, path)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, codeNotFound, "book not found")
		return
	}
	if err != nil {
		s.internal(w, r, "get book", err)
		return
	}
	prog, hasRow, err := s.store.GetProgressRow(ctx, path)
	if err != nil {
		s.internal(w, r, "get progress", err)
		return
	}
	bms, err := s.store.ListBookmarks(ctx, path)
	if err != nil {
		s.internal(w, r, "list bookmarks", err)
		return
	}
	ns, err := s.store.ListNotes(ctx, path)
	if err != nil {
		s.internal(w, r, "list notes", err)
		return
	}
	collMap, err := s.store.BookCollectionIDs(ctx)
	if err != nil {
		s.internal(w, r, "book collection ids", err)
		return
	}
	ids := collMap[book.Path]
	if ids == nil {
		ids = []int64{}
	}

	dto := BookDTO{
		Path:          book.Path,
		Title:         book.Title,
		Category:      book.Category,
		SizeBytes:     book.SizeBytes,
		Fingerprint:   book.Fingerprint,
		AddedAt:       book.AddedAt,
		Bookmarks:     make([]BookmarkDTO, 0, len(bms)),
		Notes:         make([]NoteDTO, 0, len(ns)),
		CollectionIDs: ids,
		Hidden:        book.Hidden,
	}
	if hasRow {
		pd := progressToDTO(prog, true)
		dto.Progress = &pd
	}
	for _, b := range bms {
		dto.Bookmarks = append(dto.Bookmarks, bookmarkToDTO(b))
	}
	for _, n := range ns {
		dto.Notes = append(dto.Notes, noteToDTO(n))
	}
	writeJSON(w, http.StatusOK, dto)
}

func (s *Server) getProgress(w http.ResponseWriter, r *http.Request, path string) {
	ctx := r.Context()
	if _, err := s.store.GetBook(ctx, path); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, codeNotFound, "book not found")
			return
		}
		s.internal(w, r, "get book", err)
		return
	}
	p, hasRow, err := s.store.GetProgressRow(ctx, path)
	if err != nil {
		s.internal(w, r, "get progress", err)
		return
	}
	writeJSON(w, http.StatusOK, progressToDTO(p, hasRow))
}

func (s *Server) putProgress(w http.ResponseWriter, r *http.Request, path string) {
	var req PutProgressReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid JSON: "+err.Error())
		return
	}
	p, err := s.store.SetProgress(r.Context(), path, req.CurrentPage, req.TotalPages)
	if errors.Is(err, store.ErrInvalidProgress) {
		writeError(w, http.StatusBadRequest, codeBadRequest, err.Error())
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, codeNotFound, "book not found")
		return
	}
	if err != nil {
		s.internal(w, r, "set progress", err)
		return
	}
	writeJSON(w, http.StatusOK, progressToDTO(p, true))
}

func (s *Server) putHidden(w http.ResponseWriter, r *http.Request, path string) {
	var req PutHiddenReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := s.store.SetBookHidden(r.Context(), path, req.Hidden); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, codeNotFound, "book not found")
			return
		}
		s.internal(w, r, "set hidden", err)
		return
	}
	s.getBook(w, r, path)
}

func (s *Server) putStatus(w http.ResponseWriter, r *http.Request, path string) {
	var req PutStatusReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := s.store.SetCurrentlyReading(r.Context(), path, req.CurrentlyReading); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, codeNotFound, "book not found")
			return
		}
		s.internal(w, r, "set status", err)
		return
	}
	p, hasRow, err := s.store.GetProgressRow(r.Context(), path)
	if err != nil {
		s.internal(w, r, "get progress", err)
		return
	}
	writeJSON(w, http.StatusOK, progressToDTO(p, hasRow))
}

func (s *Server) listBookmarks(w http.ResponseWriter, r *http.Request, path string) {
	ctx := r.Context()
	if _, err := s.store.GetBook(ctx, path); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, codeNotFound, "book not found")
			return
		}
		s.internal(w, r, "get book", err)
		return
	}
	bms, err := s.store.ListBookmarks(ctx, path)
	if err != nil {
		s.internal(w, r, "list bookmarks", err)
		return
	}
	out := make([]BookmarkDTO, 0, len(bms))
	for _, b := range bms {
		out = append(out, bookmarkToDTO(b))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) postBookmark(w http.ResponseWriter, r *http.Request, path string) {
	var req PostBookmarkReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid JSON: "+err.Error())
		return
	}
	bm, err := s.store.AddBookmark(r.Context(), path, req.Page, req.Label)
	if errors.Is(err, store.ErrInvalidBookmark) {
		writeError(w, http.StatusBadRequest, codeBadRequest, err.Error())
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, codeNotFound, "book not found")
		return
	}
	if err != nil {
		s.internal(w, r, "add bookmark", err)
		return
	}
	writeJSON(w, http.StatusCreated, bookmarkToDTO(bm))
}

func (s *Server) deleteBookmark(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid bookmark id")
		return
	}
	if err := s.store.DeleteBookmark(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, codeNotFound, "bookmark not found")
			return
		}
		s.internal(w, r, "delete bookmark", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listNotes(w http.ResponseWriter, r *http.Request, path string) {
	ctx := r.Context()
	if _, err := s.store.GetBook(ctx, path); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, codeNotFound, "book not found")
			return
		}
		s.internal(w, r, "get book", err)
		return
	}
	ns, err := s.store.ListNotes(ctx, path)
	if err != nil {
		s.internal(w, r, "list notes", err)
		return
	}
	out := make([]NoteDTO, 0, len(ns))
	for _, n := range ns {
		out = append(out, noteToDTO(n))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) postNote(w http.ResponseWriter, r *http.Request, path string) {
	var req PostNoteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid JSON: "+err.Error())
		return
	}
	n, err := s.store.AddNote(r.Context(), path, req.Page, req.Body, req.X, req.Y)
	if errors.Is(err, store.ErrInvalidNote) {
		writeError(w, http.StatusBadRequest, codeBadRequest, err.Error())
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, codeNotFound, "book not found")
		return
	}
	if err != nil {
		s.internal(w, r, "add note", err)
		return
	}
	writeJSON(w, http.StatusCreated, noteToDTO(n))
}

func (s *Server) patchNote(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid note id")
		return
	}
	var req PatchNoteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid JSON: "+err.Error())
		return
	}
	existing, err := s.store.GetNote(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, codeNotFound, "note not found")
		return
	}
	if err != nil {
		s.internal(w, r, "get note", err)
		return
	}
	body := existing.Body
	if req.Body != nil {
		body = *req.Body
	}
	n, err := s.store.UpdateNote(r.Context(), id, body, req.Page, req.X, req.Y, req.ClearPosition)
	if errors.Is(err, store.ErrInvalidNote) {
		writeError(w, http.StatusBadRequest, codeBadRequest, err.Error())
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, codeNotFound, "note not found")
		return
	}
	if err != nil {
		s.internal(w, r, "update note", err)
		return
	}
	writeJSON(w, http.StatusOK, noteToDTO(n))
}

func (s *Server) deleteNote(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid note id")
		return
	}
	if err := s.store.DeleteNote(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, codeNotFound, "note not found")
			return
		}
		s.internal(w, r, "delete note", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) postScan(w http.ResponseWriter, r *http.Request) {
	dir, err := s.store.GetLibraryDir(r.Context())
	if err != nil {
		s.internal(w, r, "get library_dir", err)
		return
	}
	if dir == "" {
		writeError(w, http.StatusBadRequest, codeBadRequest, "library not configured")
		return
	}
	// detach from the request context so the scan survives the HTTP response.
	if err := s.scanner.TryRun(context.Background(), dir); err != nil {
		if errors.Is(err, scanner.ErrAlreadyRunning) {
			writeError(w, http.StatusConflict, codeConflict, "scan already in progress")
			return
		}
		if errors.Is(err, scanner.ErrNotConfigured) {
			writeError(w, http.StatusBadRequest, codeBadRequest, "library not configured")
			return
		}
		s.internal(w, r, "start scan", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"started": true,
		"scanId":  time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) getScan(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, scanResultToDTO(s.scanner.Last(), s.scanner.Running()))
}

func (s *Server) internal(w http.ResponseWriter, r *http.Request, op string, err error) {
	s.log.Error("handler error", "op", op, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
}

// compile-time check that library types are referenced (silences unused imports
// when the file is trimmed during refactors).
var _ = library.Book{}
