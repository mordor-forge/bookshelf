package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"bookshelf/internal/library"
	"bookshelf/internal/scanner"
	"bookshelf/internal/store"
)

func newTestServer(t *testing.T) (*store.Store, *scanner.Scanner, http.Handler, string) {
	t.Helper()
	ctx := context.Background()
	st, err := store.OpenDSN(ctx, ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	libDir := t.TempDir()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	sc := scanner.New(st, log)
	if err := st.SetLibraryDir(ctx, libDir); err != nil {
		t.Fatalf("set library_dir: %v", err)
	}
	h := New(st, sc, nil, log)
	return st, sc, h, libDir
}

func seedBook(t *testing.T, st *store.Store, path string) library.Book {
	t.Helper()
	b := library.Book{
		Path:        path,
		Category:    strings.SplitN(path, "/", 2)[0],
		Title:       strings.TrimSuffix(filepath.Base(path), ".pdf"),
		SizeBytes:   1234,
		Fingerprint: "sha256:abc",
		AddedAt:     time.Now().UTC(),
	}
	if _, err := st.UpsertBook(context.Background(), b); err != nil {
		t.Fatalf("seed book: %v", err)
	}
	return b
}

func do(t *testing.T, h http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(buf)
	}
	req := httptest.NewRequest(method, target, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestGetLibraryEmpty(t *testing.T) {
	_, _, h, _ := newTestServer(t)
	rec := do(t, h, http.MethodGet, "/api/library", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	var got LibraryDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Categories == nil {
		t.Fatalf("categories should not be null: %s", rec.Body.String())
	}
	if len(got.Categories) != 0 {
		t.Fatalf("expected empty categories, got %d", len(got.Categories))
	}
}

func TestGetBookFlow(t *testing.T) {
	st, _, h, _ := newTestServer(t)
	seedBook(t, st, "Fiction/Dune.pdf")

	rec := do(t, h, http.MethodGet, "/api/books/Fiction/Dune.pdf", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	var book BookDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &book); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if book.Path != "Fiction/Dune.pdf" || book.Title != "Dune" {
		t.Fatalf("unexpected book: %+v", book)
	}

	rec = do(t, h, http.MethodGet, "/api/books/Fiction/Missing.pdf", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestPutProgressValidation(t *testing.T) {
	st, _, h, _ := newTestServer(t)
	seedBook(t, st, "Fiction/Dune.pdf")

	rec := do(t, h, http.MethodPut, "/api/books/Fiction/Dune.pdf/progress",
		map[string]int{"currentPage": 0, "totalPages": 10})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = do(t, h, http.MethodPut, "/api/books/Fiction/Dune.pdf/progress",
		map[string]int{"currentPage": 50, "totalPages": 10})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = do(t, h, http.MethodPut, "/api/books/Fiction/Dune.pdf/progress",
		map[string]int{"currentPage": 5, "totalPages": 100})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var p ProgressDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	if p.CurrentPage != 5 || p.TotalPages != 100 {
		t.Fatalf("bad progress dto: %+v", p)
	}
	if p.Percent < 4.9 || p.Percent > 5.1 {
		t.Fatalf("bad percent: %v", p.Percent)
	}
}

func TestBookmarkRoundtrip(t *testing.T) {
	st, _, h, _ := newTestServer(t)
	seedBook(t, st, "Fiction/Dune.pdf")

	label := "Arrakis"
	rec := do(t, h, http.MethodPost, "/api/books/Fiction/Dune.pdf/bookmarks",
		map[string]any{"page": 100, "label": label})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created BookmarkDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ID == 0 || created.Page != 100 || created.Label == nil || *created.Label != "Arrakis" {
		t.Fatalf("bad bookmark dto: %+v", created)
	}

	rec = do(t, h, http.MethodGet, "/api/books/Fiction/Dune.pdf/bookmarks", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d", rec.Code)
	}
	var list []BookmarkDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("bad list: %+v", list)
	}

	rec = do(t, h, http.MethodDelete, "/api/bookmarks/"+itoa(created.ID), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = do(t, h, http.MethodDelete, "/api/bookmarks/"+itoa(created.ID), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on second delete, got %d", rec.Code)
	}
}

func TestScanConflict(t *testing.T) {
	_, sc, h, libDir := newTestServer(t)

	// simulate already-running by starting one and exploiting Running() state via TryRun.
	// to make the race deterministic, set running=true via a synchronous trick: call TryRun
	// against a directory that contains many files. Simpler: call TryRun once, then
	// immediately call POST /api/scan before the first goroutine clears its flag. To make
	// this reliable, point scanner at a nonexistent dir + use sequential POSTs.
	// Easiest deterministic approach: directly call TryRun (it sets running=true synchronously),
	// then issue the HTTP POST while running, then wait for it to settle.
	if err := sc.TryRun(context.Background(), libDir); err != nil {
		t.Fatalf("first TryRun: %v", err)
	}
	rec := do(t, h, http.MethodPost, "/api/scan", nil)
	if rec.Code != http.StatusConflict {
		// the background goroutine may have finished already; retry quickly until it's
		// confirmed running or finishes. accept either 409 (intended) or 202 (race).
		// to ensure we tested the conflict path, fail if not 409.
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}

	// wait for the background scan to finish so cleanup is clean.
	deadline := time.Now().Add(2 * time.Second)
	for sc.Running() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
}

func TestScanAcceptedThenStatus(t *testing.T) {
	_, sc, h, _ := newTestServer(t)
	rec := do(t, h, http.MethodPost, "/api/scan", nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	deadline := time.Now().Add(2 * time.Second)
	for sc.Running() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	rec = do(t, h, http.MethodGet, "/api/scan", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var s ScanStatusDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.Running {
		t.Fatalf("expected scanner to be finished")
	}
}

func TestPdfRangeRequest(t *testing.T) {
	st, _, h, libDir := newTestServer(t)
	rel := "Fiction/test.pdf"
	full := filepath.Join(libDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := []byte("%PDF-1.4 fake content for range testing 0123456789")
	if err := os.WriteFile(full, content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	seedBook(t, st, rel)

	req := httptest.NewRequest(http.MethodGet, "/books/"+rel, nil)
	req.Header.Set("Range", "bytes=0-3")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("expected 206, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.Bytes(); !bytes.Equal(got, content[:4]) {
		t.Fatalf("range body mismatch: got %q want %q", got, content[:4])
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("content-type: %q", ct)
	}
}

func itoa(i int64) string {
	return strconv.FormatInt(i, 10)
}

// newUnconfiguredServer builds a test server with no library_dir setting.
func newUnconfiguredServer(t *testing.T) (*store.Store, http.Handler) {
	t.Helper()
	ctx := context.Background()
	st, err := store.OpenDSN(ctx, ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	sc := scanner.New(st, log)
	h := New(st, sc, nil, log)
	return st, h
}

func TestGetSettingsInitiallyEmpty(t *testing.T) {
	_, h := newUnconfiguredServer(t)
	rec := do(t, h, http.MethodGet, "/api/settings", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	var got SettingsDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.LibraryDir != "" {
		t.Fatalf("expected empty libraryDir, got %q", got.LibraryDir)
	}
}

func TestPutSettingsInvalidPath(t *testing.T) {
	_, h := newUnconfiguredServer(t)
	bogus := filepath.Join(os.TempDir(), "definitely-not-a-real-dir-xyzzy-12345")
	rec := do(t, h, http.MethodPut, "/api/settings",
		map[string]string{"libraryDir": bogus})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPutSettingsValidPathThenLibraryConfigured(t *testing.T) {
	_, h := newUnconfiguredServer(t)
	dir := t.TempDir()
	rec := do(t, h, http.MethodPut, "/api/settings",
		map[string]string{"libraryDir": dir})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = do(t, h, http.MethodGet, "/api/library", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("library status %d", rec.Code)
	}
	var lib LibraryDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &lib); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !lib.LibraryConfigured {
		t.Fatalf("expected libraryConfigured true")
	}
}

func TestPutBookHidden(t *testing.T) {
	st, _, h, _ := newTestServer(t)
	seedBook(t, st, "Fiction/Dune.pdf")

	rec := do(t, h, http.MethodPut, "/api/books/Fiction/Dune.pdf/hidden",
		map[string]bool{"hidden": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var book BookDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &book); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !book.Hidden {
		t.Fatalf("expected hidden=true, got %+v", book)
	}

	rec = do(t, h, http.MethodPut, "/api/books/Fiction/Dune.pdf/hidden",
		map[string]bool{"hidden": false})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &book); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if book.Hidden {
		t.Fatalf("expected hidden=false, got %+v", book)
	}

	rec = do(t, h, http.MethodPut, "/api/books/Fiction/Missing.pdf/hidden",
		map[string]bool{"hidden": true})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPostScanWhenNotConfigured(t *testing.T) {
	_, h := newUnconfiguredServer(t)
	rec := do(t, h, http.MethodPost, "/api/scan", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}
