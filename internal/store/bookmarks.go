package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"bookshelf/internal/library"
)

// ErrInvalidBookmark signals client-side validation errors for bookmark inserts.
var ErrInvalidBookmark = errors.New("invalid bookmark")

func (s *Store) ListBookmarks(ctx context.Context, bookPath string) ([]library.Bookmark, error) {
	var bms []library.Bookmark
	err := s.db.SelectContext(ctx, &bms,
		`SELECT id, book_path, page, label, created_at
		   FROM bookmarks WHERE book_path = ? ORDER BY page ASC, id ASC`, bookPath)
	return bms, err
}

func (s *Store) AddBookmark(ctx context.Context, bookPath string, page int, label *string) (library.Bookmark, error) {
	if page < 1 {
		return library.Bookmark{}, fmt.Errorf("%w: page must be >= 1", ErrInvalidBookmark)
	}
	if label != nil && len(*label) > 200 {
		return library.Bookmark{}, fmt.Errorf("%w: label too long", ErrInvalidBookmark)
	}

	var exists int
	if err := s.db.GetContext(ctx, &exists,
		`SELECT COUNT(1) FROM books WHERE path = ? AND removed_at IS NULL`, bookPath); err != nil {
		return library.Bookmark{}, err
	}
	if exists == 0 {
		return library.Bookmark{}, sql.ErrNoRows
	}

	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO bookmarks (book_path, page, label, created_at) VALUES (?, ?, ?, ?)`,
		bookPath, page, label, now)
	if err != nil {
		return library.Bookmark{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return library.Bookmark{}, err
	}
	return library.Bookmark{ID: id, BookPath: bookPath, Page: page, Label: label, CreatedAt: now}, nil
}

func (s *Store) GetBookmark(ctx context.Context, id int64) (library.Bookmark, error) {
	var bm library.Bookmark
	err := s.db.GetContext(ctx, &bm,
		`SELECT id, book_path, page, label, created_at FROM bookmarks WHERE id = ?`, id)
	return bm, err
}

func (s *Store) DeleteBookmark(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM bookmarks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
