package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"bookshelf/internal/library"
)

// ErrInvalidProgress signals client-side validation errors for progress upserts.
var ErrInvalidProgress = errors.New("invalid progress")

func (s *Store) GetProgress(ctx context.Context, bookPath string) (library.Progress, error) {
	var p library.Progress
	err := s.db.GetContext(ctx, &p,
		`SELECT book_path, current_page, total_pages, last_read_at, currently_reading
		   FROM progress WHERE book_path = ?`, bookPath)
	if errors.Is(err, sql.ErrNoRows) {
		return library.Progress{BookPath: bookPath, CurrentPage: 1}, nil
	}
	return p, err
}

// GetProgressRow is like GetProgress but also reports whether a row actually exists.
func (s *Store) GetProgressRow(ctx context.Context, bookPath string) (library.Progress, bool, error) {
	var p library.Progress
	err := s.db.GetContext(ctx, &p,
		`SELECT book_path, current_page, total_pages, last_read_at, currently_reading
		   FROM progress WHERE book_path = ?`, bookPath)
	if errors.Is(err, sql.ErrNoRows) {
		return library.Progress{BookPath: bookPath, CurrentPage: 1}, false, nil
	}
	if err != nil {
		return library.Progress{}, false, err
	}
	return p, true, nil
}

// SetProgress upserts the progress row for a book. Returns sql.ErrNoRows if the book
// does not exist or has been removed. Returns ErrInvalidProgress wrapped with a reason
// if input validation fails.
func (s *Store) SetProgress(ctx context.Context, bookPath string, currentPage, totalPages int) (library.Progress, error) {
	if currentPage < 1 {
		return library.Progress{}, fmt.Errorf("%w: currentPage must be >= 1", ErrInvalidProgress)
	}
	if totalPages < 0 {
		return library.Progress{}, fmt.Errorf("%w: totalPages must be >= 0", ErrInvalidProgress)
	}
	if totalPages > 0 && currentPage > totalPages {
		return library.Progress{}, fmt.Errorf("%w: currentPage must be <= totalPages", ErrInvalidProgress)
	}

	// verify book exists and is live
	var exists int
	if err := s.db.GetContext(ctx, &exists,
		`SELECT COUNT(1) FROM books WHERE path = ? AND removed_at IS NULL`, bookPath); err != nil {
		return library.Progress{}, err
	}
	if exists == 0 {
		return library.Progress{}, sql.ErrNoRows
	}

	now := time.Now().UTC()
	// keep MAX(totalPages, existing)
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO progress (book_path, current_page, total_pages, last_read_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(book_path) DO UPDATE SET
			current_page = excluded.current_page,
			total_pages  = MAX(progress.total_pages, excluded.total_pages),
			last_read_at = excluded.last_read_at
	`, bookPath, currentPage, totalPages, now); err != nil {
		return library.Progress{}, err
	}

	return s.GetProgress(ctx, bookPath)
}

// SetCurrentlyReading sets the currently_reading flag for a book. If no progress row
// exists, it creates one with sensible defaults (currentPage=1, totalPages=0).
// Returns sql.ErrNoRows if the book is missing or soft-removed.
func (s *Store) SetCurrentlyReading(ctx context.Context, bookPath string, value bool) error {
	var exists int
	if err := s.db.GetContext(ctx, &exists,
		`SELECT COUNT(1) FROM books WHERE path = ? AND removed_at IS NULL`, bookPath); err != nil {
		return err
	}
	if exists == 0 {
		return sql.ErrNoRows
	}
	v := 0
	if value {
		v = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO progress (book_path, current_page, total_pages, last_read_at, currently_reading)
		VALUES (?, 1, 0, NULL, ?)
		ON CONFLICT(book_path) DO UPDATE SET
			currently_reading = excluded.currently_reading
	`, bookPath, v)
	return err
}
