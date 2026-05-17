package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestNotesPostAndGet(t *testing.T) {
	st, _, h, _ := newTestServer(t)
	seedBook(t, st, "Fiction/Dune.pdf")

	rec := do(t, h, http.MethodPost, "/api/books/Fiction/Dune.pdf/notes",
		map[string]any{"page": 12, "body": "fremen lore"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created NoteDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ID == 0 || created.Page != 12 || created.Body != "fremen lore" {
		t.Fatalf("bad note dto: %+v", created)
	}

	rec = do(t, h, http.MethodGet, "/api/books/Fiction/Dune.pdf/notes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d", rec.Code)
	}
	var list []NoteDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("bad list: %+v", list)
	}

	// Notes also surface via GET /api/books/{path}.
	rec = do(t, h, http.MethodGet, "/api/books/Fiction/Dune.pdf", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get book %d", rec.Code)
	}
	var book BookDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &book)
	if len(book.Notes) != 1 || book.Notes[0].ID != created.ID {
		t.Fatalf("expected notes in book DTO, got %+v", book.Notes)
	}
}

func TestNotesPatch(t *testing.T) {
	st, _, h, _ := newTestServer(t)
	seedBook(t, st, "Fiction/Dune.pdf")

	rec := do(t, h, http.MethodPost, "/api/books/Fiction/Dune.pdf/notes",
		map[string]any{"page": 5, "body": "first"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created NoteDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = do(t, h, http.MethodPatch, "/api/notes/"+itoa(created.ID),
		map[string]any{"body": "updated body"})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch body: %d %s", rec.Code, rec.Body.String())
	}
	var updated NoteDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Body != "updated body" || updated.Page != 5 {
		t.Fatalf("bad patched note: %+v", updated)
	}

	rec = do(t, h, http.MethodPatch, "/api/notes/"+itoa(created.ID),
		map[string]any{"page": 42})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch page: %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Page != 42 || updated.Body != "updated body" {
		t.Fatalf("bad patched note: %+v", updated)
	}
}

func TestNotesDelete(t *testing.T) {
	st, _, h, _ := newTestServer(t)
	seedBook(t, st, "Fiction/Dune.pdf")

	rec := do(t, h, http.MethodPost, "/api/books/Fiction/Dune.pdf/notes",
		map[string]any{"page": 1, "body": "hi"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d", rec.Code)
	}
	var created NoteDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = do(t, h, http.MethodDelete, "/api/notes/"+itoa(created.ID), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	rec = do(t, h, http.MethodGet, "/api/books/Fiction/Dune.pdf/notes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d", rec.Code)
	}
	var list []NoteDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %+v", list)
	}
}

func TestNotesPostEmptyBody(t *testing.T) {
	st, _, h, _ := newTestServer(t)
	seedBook(t, st, "Fiction/Dune.pdf")

	rec := do(t, h, http.MethodPost, "/api/books/Fiction/Dune.pdf/notes",
		map[string]any{"page": 1, "body": "   "})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNotesPostWithPosition(t *testing.T) {
	st, _, h, _ := newTestServer(t)
	seedBook(t, st, "Fiction/Dune.pdf")

	rec := do(t, h, http.MethodPost, "/api/books/Fiction/Dune.pdf/notes",
		map[string]any{"page": 3, "body": "anchored", "x": 0.25, "y": 0.4})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created NoteDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.X == nil || created.Y == nil || *created.X != 0.25 || *created.Y != 0.4 {
		t.Fatalf("bad x/y: %+v", created)
	}

	// list roundtrip preserves x/y.
	rec = do(t, h, http.MethodGet, "/api/books/Fiction/Dune.pdf/notes", nil)
	var list []NoteDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0].X == nil || *list[0].X != 0.25 {
		t.Fatalf("list missing x/y: %+v", list)
	}
}

func TestNotesPostInvalidPosition(t *testing.T) {
	st, _, h, _ := newTestServer(t)
	seedBook(t, st, "Fiction/Dune.pdf")

	rec := do(t, h, http.MethodPost, "/api/books/Fiction/Dune.pdf/notes",
		map[string]any{"page": 1, "body": "bad", "x": 1.5, "y": 0.5})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	// only one of x/y is also invalid.
	rec = do(t, h, http.MethodPost, "/api/books/Fiction/Dune.pdf/notes",
		map[string]any{"page": 1, "body": "bad", "x": 0.3})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for partial x, got %d", rec.Code)
	}
}

func TestNotesPatchClearPosition(t *testing.T) {
	st, _, h, _ := newTestServer(t)
	seedBook(t, st, "Fiction/Dune.pdf")

	rec := do(t, h, http.MethodPost, "/api/books/Fiction/Dune.pdf/notes",
		map[string]any{"page": 2, "body": "anchored", "x": 0.5, "y": 0.5})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created NoteDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = do(t, h, http.MethodPatch, "/api/notes/"+itoa(created.ID),
		map[string]any{"clearPosition": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body.String())
	}
	var updated NoteDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.X != nil || updated.Y != nil {
		t.Fatalf("expected cleared position: %+v", updated)
	}
}

func TestNotesPatchUnknown(t *testing.T) {
	_, _, h, _ := newTestServer(t)
	rec := do(t, h, http.MethodPatch, "/api/notes/99999",
		map[string]any{"body": "x"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}
