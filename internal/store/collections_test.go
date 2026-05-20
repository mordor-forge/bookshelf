package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"bookshelf/internal/library"
)

func TestSyncScanCollectionForBookRollsBackOnRelinkFailure(t *testing.T) {
	ctx := context.Background()
	st, err := OpenDSN(ctx, ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	a, err := st.UpsertScanCollection(ctx, "A", "A", nil)
	if err != nil {
		t.Fatalf("upsert A: %v", err)
	}
	aID := a.ID
	b, err := st.UpsertScanCollection(ctx, "A/B", "B", &aID)
	if err != nil {
		t.Fatalf("upsert A/B: %v", err)
	}

	book := library.Book{
		Path:        "A/B/y.pdf",
		Category:    "A",
		Title:       "y",
		SizeBytes:   123,
		Fingerprint: "sha256:test",
		AddedAt:     time.Now().UTC(),
	}
	if _, err := st.UpsertBook(ctx, book); err != nil {
		t.Fatalf("seed book: %v", err)
	}
	if err := st.AddBookToCollection(ctx, book.Path, a.ID); err != nil {
		t.Fatalf("seed legacy flat link: %v", err)
	}
	if _, err := st.MarkRemoved(ctx, []string{book.Path}, time.Now().UTC()); err != nil {
		t.Fatalf("mark removed: %v", err)
	}

	err = st.SyncScanCollectionForBook(ctx, book.Path, &b.ID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}

	links, err := st.BookCollectionIDs(ctx)
	if err != nil {
		t.Fatalf("links: %v", err)
	}
	got := links[book.Path]
	if len(got) != 1 || got[0] != a.ID {
		t.Fatalf("expected legacy link to remain after rollback, got %v", got)
	}
}
