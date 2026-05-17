package store

import (
	"context"
	"database/sql"
	"time"

	"bookshelf/internal/library"
)

// UpsertBook inserts a new book or, if a row with the same path exists, refreshes its
// mutable fields and clears removed_at. Returns true iff a new row was inserted.
func (s *Store) UpsertBook(ctx context.Context, b library.Book) (inserted bool, err error) {
	var existed int
	if err := s.db.GetContext(ctx, &existed,
		`SELECT COUNT(1) FROM books WHERE path = ?`, b.Path); err != nil {
		return false, err
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO books (path, category, title, size_bytes, fingerprint, added_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			category    = excluded.category,
			title       = excluded.title,
			size_bytes  = excluded.size_bytes,
			fingerprint = excluded.fingerprint,
			removed_at  = NULL
	`, b.Path, b.Category, b.Title, b.SizeBytes, b.Fingerprint, b.AddedAt); err != nil {
		return false, err
	}
	return existed == 0, nil
}

func (s *Store) MarkRemoved(ctx context.Context, paths []string, when time.Time) (int, error) {
	if len(paths) == 0 {
		return 0, nil
	}
	q, args, err := buildIn(
		`UPDATE books SET removed_at = ? WHERE removed_at IS NULL AND path IN `,
		paths, when)
	if err != nil {
		return 0, err
	}
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (s *Store) ListLivePaths(ctx context.Context) ([]string, error) {
	var paths []string
	if err := s.db.SelectContext(ctx, &paths,
		`SELECT path FROM books WHERE removed_at IS NULL`); err != nil {
		return nil, err
	}
	return paths, nil
}

func (s *Store) CountBooks(ctx context.Context) (live, removed int, err error) {
	row := s.db.QueryRowxContext(ctx, `
		SELECT
		  SUM(CASE WHEN removed_at IS NULL THEN 1 ELSE 0 END),
		  SUM(CASE WHEN removed_at IS NOT NULL THEN 1 ELSE 0 END)
		FROM books`)
	var l, r *int
	if err := row.Scan(&l, &r); err != nil {
		return 0, 0, err
	}
	if l != nil {
		live = *l
	}
	if r != nil {
		removed = *r
	}
	return live, removed, nil
}

// SetBookHidden sets the hidden flag for a book. Returns sql.ErrNoRows if the
// book is missing or soft-removed.
func (s *Store) SetBookHidden(ctx context.Context, path string, hidden bool) error {
	v := 0
	if hidden {
		v = 1
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE books SET hidden = ? WHERE path = ? AND removed_at IS NULL`, v, path)
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

// buildIn builds "<prefix> (?, ?, ...)" with leading args followed by the IN values.
func buildIn(prefix string, in []string, leading ...any) (string, []any, error) {
	args := make([]any, 0, len(leading)+len(in))
	args = append(args, leading...)
	q := prefix + "("
	for i, v := range in {
		if i > 0 {
			q += ","
		}
		q += "?"
		args = append(args, v)
	}
	q += ")"
	return q, args, nil
}
