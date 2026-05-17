package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bookshelf/internal/library"
	"bookshelf/internal/scanner"
	"bookshelf/internal/store"
)

func TestCreateAndListCollection(t *testing.T) {
	_, _, h, _ := newTestServer(t)

	rec := do(t, h, http.MethodPost, "/api/collections",
		map[string]any{"name": "Favorites"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d body=%s", rec.Code, rec.Body.String())
	}
	var created CollectionDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ID == 0 || created.Name != "Favorites" || created.Source != "manual" {
		t.Fatalf("bad dto: %+v", created)
	}

	rec = do(t, h, http.MethodGet, "/api/collections", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d", rec.Code)
	}
	var list []CollectionDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("bad list: %+v", list)
	}
}

func TestRenameAnyCollection(t *testing.T) {
	st, _, h, _ := newTestServer(t)
	ctx := context.Background()

	rec := do(t, h, http.MethodPost, "/api/collections", map[string]any{"name": "Foo"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d", rec.Code)
	}
	var manual CollectionDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &manual)

	rec = do(t, h, http.MethodPatch, "/api/collections/"+itoa(manual.ID),
		map[string]any{"name": "Bar"})
	if rec.Code != http.StatusOK {
		t.Fatalf("rename manual status %d body=%s", rec.Code, rec.Body.String())
	}

	scanColl, err := st.UpsertScanCollection(ctx, "Sci-Fi", "Sci-Fi", nil)
	if err != nil {
		t.Fatalf("upsert scan: %v", err)
	}
	rec = do(t, h, http.MethodPatch, "/api/collections/"+itoa(scanColl.ID),
		map[string]any{"name": "Renamed"})
	if rec.Code != http.StatusOK {
		t.Fatalf("rename scan status %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDeleteAnyCollection(t *testing.T) {
	st, _, h, _ := newTestServer(t)
	ctx := context.Background()

	rec := do(t, h, http.MethodPost, "/api/collections", map[string]any{"name": "Doomed"})
	var manual CollectionDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &manual)
	rec = do(t, h, http.MethodDelete, "/api/collections/"+itoa(manual.ID), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete manual: %d", rec.Code)
	}

	scanColl, err := st.UpsertScanCollection(ctx, "Fantasy", "Fantasy", nil)
	if err != nil {
		t.Fatalf("upsert scan: %v", err)
	}
	rec = do(t, h, http.MethodDelete, "/api/collections/"+itoa(scanColl.ID), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete scan: %d", rec.Code)
	}
}

func TestAddBookToCollectionAndLibraryReflects(t *testing.T) {
	st, _, h, _ := newTestServer(t)
	seedBook(t, st, "Fiction/Dune.pdf")

	rec := do(t, h, http.MethodPost, "/api/collections", map[string]any{"name": "Faves"})
	var c CollectionDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &c)

	rec = do(t, h, http.MethodPost, "/api/collections/"+itoa(c.ID)+"/books",
		map[string]any{"path": "Fiction/Dune.pdf"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("link status %d body=%s", rec.Code, rec.Body.String())
	}

	rec = do(t, h, http.MethodGet, "/api/library", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("library status %d", rec.Code)
	}
	var lib LibraryDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &lib)
	if len(lib.Categories) != 1 || len(lib.Categories[0].Books) != 1 {
		t.Fatalf("expected 1 book in 1 category: %+v", lib)
	}
	ids := lib.Categories[0].Books[0].CollectionIDs
	if len(ids) != 1 || ids[0] != c.ID {
		t.Fatalf("expected collectionIds [%d], got %v", c.ID, ids)
	}
	// idempotent re-link
	rec = do(t, h, http.MethodPost, "/api/collections/"+itoa(c.ID)+"/books",
		map[string]any{"path": "Fiction/Dune.pdf"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 on re-link, got %d", rec.Code)
	}
}

func TestPutBookStatus(t *testing.T) {
	st, _, h, _ := newTestServer(t)
	seedBook(t, st, "Fiction/Dune.pdf")

	rec := do(t, h, http.MethodPut, "/api/books/Fiction/Dune.pdf/status",
		map[string]any{"currentlyReading": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	var p ProgressDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	if !p.CurrentlyReading || p.Status != string(library.StatusCurrentlyReading) {
		t.Fatalf("bad dto: %+v", p)
	}

	rec = do(t, h, http.MethodGet, "/api/books/Fiction/Dune.pdf", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d", rec.Code)
	}
	var book BookDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &book)
	if book.Progress == nil || book.Progress.Status != string(library.StatusCurrentlyReading) {
		t.Fatalf("expected currently_reading on book: %+v", book.Progress)
	}

	// flip back
	rec = do(t, h, http.MethodPut, "/api/books/Fiction/Dune.pdf/status",
		map[string]any{"currentlyReading": false})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	if p.CurrentlyReading {
		t.Fatalf("expected false: %+v", p)
	}

	// unknown book → 404
	rec = do(t, h, http.MethodPut, "/api/books/Missing.pdf/status",
		map[string]any{"currentlyReading": true})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestComputeStatus(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name   string
		p      library.Progress
		hasRow bool
		want   library.Status
	}{
		{"never started no row", library.Progress{}, false, library.StatusNeverStarted},
		{"never started with row defaults", library.Progress{CurrentPage: 1}, true, library.StatusNeverStarted},
		{"currently reading", library.Progress{CurrentPage: 3, CurrentlyReading: true, LastReadAt: &now}, true, library.StatusCurrentlyReading},
		{"in progress", library.Progress{CurrentPage: 5, TotalPages: 100, LastReadAt: &now}, true, library.StatusInProgress},
		{"completed wins over currently_reading", library.Progress{CurrentPage: 100, TotalPages: 100, CurrentlyReading: true}, true, library.StatusCompleted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := library.ComputeStatus(tc.p, tc.hasRow)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestScannerBuildsCollections(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenDSN(ctx, ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	dir := t.TempDir()
	// create A/x.pdf and A/B/y.pdf
	mustWrite := func(rel string) {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte("%PDF-1.4 fake"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	mustWrite("A/x.pdf")
	mustWrite("A/B/y.pdf")

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	sc := scanner.New(st, log)
	if _, err := sc.Run(ctx, dir); err != nil {
		t.Fatalf("scan: %v", err)
	}

	cs, err := st.ListCollections(ctx)
	if err != nil {
		t.Fatalf("list collections: %v", err)
	}
	if len(cs) != 2 {
		t.Fatalf("expected 2 collections, got %d: %+v", len(cs), cs)
	}
	byPath := map[string]library.Collection{}
	for _, c := range cs {
		if c.FolderPath != nil {
			byPath[*c.FolderPath] = c
		}
	}
	a, okA := byPath["A"]
	b, okB := byPath["A/B"]
	if !okA || !okB {
		t.Fatalf("expected A and A/B collections, got %+v", byPath)
	}
	if a.ParentID != nil {
		t.Fatalf("A should have nil parent, got %v", a.ParentID)
	}
	if b.ParentID == nil || *b.ParentID != a.ID {
		t.Fatalf("A/B should have parent=A(%d), got %v", a.ID, b.ParentID)
	}

	links, err := st.BookCollectionIDs(ctx)
	if err != nil {
		t.Fatalf("links: %v", err)
	}
	if got := links["A/x.pdf"]; len(got) != 1 || got[0] != a.ID {
		t.Fatalf("expected x.pdf in A, got %v", got)
	}
	if got := links["A/B/y.pdf"]; len(got) != 1 || got[0] != b.ID {
		t.Fatalf("expected y.pdf in A/B, got %v", got)
	}
}
