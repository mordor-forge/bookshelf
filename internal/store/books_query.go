package store

import (
	"context"

	"bookshelf/internal/library"
)

func (s *Store) GetBook(ctx context.Context, path string) (library.Book, error) {
	var b library.Book
	err := s.db.GetContext(ctx, &b, `
		SELECT path, category, title, size_bytes, fingerprint, added_at, removed_at, hidden
		  FROM books
		 WHERE path = ? AND removed_at IS NULL`, path)
	return b, err
}

func (s *Store) ListBooks(ctx context.Context) ([]library.Book, error) {
	var bs []library.Book
	err := s.db.SelectContext(ctx, &bs, `
		SELECT path, category, title, size_bytes, fingerprint, added_at, removed_at, hidden
		  FROM books
		 WHERE removed_at IS NULL
		 ORDER BY category ASC, title ASC`)
	return bs, err
}

func (s *Store) BookmarkCounts(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryxContext(ctx, `
		SELECT book_path, COUNT(*) AS n
		  FROM bookmarks
		 GROUP BY book_path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var path string
		var n int
		if err := rows.Scan(&path, &n); err != nil {
			return nil, err
		}
		out[path] = n
	}
	return out, rows.Err()
}

func (s *Store) ProgressMap(ctx context.Context) (map[string]library.Progress, error) {
	var ps []library.Progress
	if err := s.db.SelectContext(ctx, &ps, `
		SELECT book_path, current_page, total_pages, last_read_at, currently_reading
		  FROM progress`); err != nil {
		return nil, err
	}
	out := make(map[string]library.Progress, len(ps))
	for _, p := range ps {
		out[p.BookPath] = p
	}
	return out, nil
}
