package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"bookshelf/internal/library"
)

// ErrInvalidNote signals client-side validation errors for note inserts/updates.
var ErrInvalidNote = errors.New("invalid note")

const maxNoteBodyLen = 10000

func validateNoteBody(body string) (string, error) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return "", fmt.Errorf("%w: body must not be empty", ErrInvalidNote)
	}
	if len(trimmed) > maxNoteBodyLen {
		return "", fmt.Errorf("%w: body too long", ErrInvalidNote)
	}
	return trimmed, nil
}

func validateNotePosition(x, y *float64) error {
	if (x == nil) != (y == nil) {
		return fmt.Errorf("%w: x and y must both be set or both be omitted", ErrInvalidNote)
	}
	if x == nil {
		return nil
	}
	if *x < 0 || *x > 1 || *y < 0 || *y > 1 {
		return fmt.Errorf("%w: x and y must be within [0, 1]", ErrInvalidNote)
	}
	return nil
}

func (s *Store) ListNotes(ctx context.Context, bookPath string) ([]library.Note, error) {
	var ns []library.Note
	err := s.db.SelectContext(ctx, &ns,
		`SELECT id, book_path, page, body, created_at, updated_at, x, y
		   FROM notes WHERE book_path = ? ORDER BY page ASC, id ASC`, bookPath)
	return ns, err
}

func (s *Store) GetNote(ctx context.Context, id int64) (library.Note, error) {
	var n library.Note
	err := s.db.GetContext(ctx, &n,
		`SELECT id, book_path, page, body, created_at, updated_at, x, y FROM notes WHERE id = ?`, id)
	return n, err
}

func (s *Store) AddNote(ctx context.Context, bookPath string, page int, body string, x, y *float64) (library.Note, error) {
	if page < 1 {
		return library.Note{}, fmt.Errorf("%w: page must be >= 1", ErrInvalidNote)
	}
	trimmed, err := validateNoteBody(body)
	if err != nil {
		return library.Note{}, err
	}
	if err := validateNotePosition(x, y); err != nil {
		return library.Note{}, err
	}

	var exists int
	if err := s.db.GetContext(ctx, &exists,
		`SELECT COUNT(1) FROM books WHERE path = ? AND removed_at IS NULL`, bookPath); err != nil {
		return library.Note{}, err
	}
	if exists == 0 {
		return library.Note{}, sql.ErrNoRows
	}

	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO notes (book_path, page, body, created_at, updated_at, x, y) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		bookPath, page, trimmed, now, now, x, y)
	if err != nil {
		return library.Note{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return library.Note{}, err
	}
	return library.Note{
		ID:        id,
		BookPath:  bookPath,
		Page:      page,
		Body:      trimmed,
		CreatedAt: now,
		UpdatedAt: now,
		X:         x,
		Y:         y,
	}, nil
}

func (s *Store) UpdateNote(ctx context.Context, id int64, body string, page *int, x, y *float64, clearPosition bool) (library.Note, error) {
	trimmed, err := validateNoteBody(body)
	if err != nil {
		return library.Note{}, err
	}
	if page != nil && *page < 1 {
		return library.Note{}, fmt.Errorf("%w: page must be >= 1", ErrInvalidNote)
	}
	if !clearPosition {
		if err := validateNotePosition(x, y); err != nil {
			return library.Note{}, err
		}
	}

	sets := []string{"body = ?", "updated_at = ?"}
	now := time.Now().UTC()
	args := []any{trimmed, now}

	if page != nil {
		sets = append(sets, "page = ?")
		args = append(args, *page)
	}
	if clearPosition {
		sets = append(sets, "x = NULL", "y = NULL")
	} else if x != nil {
		sets = append(sets, "x = ?", "y = ?")
		args = append(args, *x, *y)
	}
	args = append(args, id)

	query := "UPDATE notes SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return library.Note{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return library.Note{}, err
	}
	if n == 0 {
		return library.Note{}, sql.ErrNoRows
	}
	return s.GetNote(ctx, id)
}

func (s *Store) DeleteNote(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, id)
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
