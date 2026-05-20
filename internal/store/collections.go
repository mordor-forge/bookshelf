package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"bookshelf/internal/library"
)

// ErrInvalidCollection signals client-side validation errors for collection operations.
var ErrInvalidCollection = errors.New("invalid collection")

const (
	collectionSourceScan   = "scan"
	collectionSourceManual = "manual"
)

func (s *Store) ListCollections(ctx context.Context) ([]library.Collection, error) {
	var cs []library.Collection
	err := s.db.SelectContext(ctx, &cs, `
		SELECT id, name, parent_id, source, folder_path, created_at
		  FROM collections
		 ORDER BY COALESCE(parent_id, 0) ASC, name ASC`)
	return cs, err
}

func (s *Store) GetCollection(ctx context.Context, id int64) (library.Collection, error) {
	var c library.Collection
	err := s.db.GetContext(ctx, &c, `
		SELECT id, name, parent_id, source, folder_path, created_at
		  FROM collections WHERE id = ?`, id)
	return c, err
}

func (s *Store) getCollectionByFolderPath(ctx context.Context, folderPath string) (library.Collection, bool, error) {
	var c library.Collection
	err := s.db.GetContext(ctx, &c, `
		SELECT id, name, parent_id, source, folder_path, created_at
		  FROM collections WHERE folder_path = ?`, folderPath)
	if errors.Is(err, sql.ErrNoRows) {
		return library.Collection{}, false, nil
	}
	if err != nil {
		return library.Collection{}, false, err
	}
	return c, true, nil
}

// CreateManualCollection creates a manual-source collection. name must be non-empty
// and unique (case-insensitive) within its parent scope.
func (s *Store) CreateManualCollection(ctx context.Context, name string, parentID *int64) (library.Collection, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return library.Collection{}, fmt.Errorf("%w: name is required", ErrInvalidCollection)
	}
	if parentID != nil {
		if _, err := s.GetCollection(ctx, *parentID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return library.Collection{}, fmt.Errorf("%w: parent not found", ErrInvalidCollection)
			}
			return library.Collection{}, err
		}
	}
	dup, err := s.nameTakenUnderParent(ctx, name, parentID, 0)
	if err != nil {
		return library.Collection{}, err
	}
	if dup {
		return library.Collection{}, fmt.Errorf("%w: name already exists under parent", ErrInvalidCollection)
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO collections (name, parent_id, source, folder_path)
		VALUES (?, ?, 'manual', NULL)`, name, parentID)
	if err != nil {
		return library.Collection{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return library.Collection{}, err
	}
	return s.GetCollection(ctx, id)
}

func (s *Store) nameTakenUnderParent(ctx context.Context, name string, parentID *int64, excludeID int64) (bool, error) {
	var n int
	var err error
	if parentID == nil {
		err = s.db.GetContext(ctx, &n,
			`SELECT COUNT(1) FROM collections
			  WHERE parent_id IS NULL AND LOWER(name) = LOWER(?) AND id != ?`,
			name, excludeID)
	} else {
		err = s.db.GetContext(ctx, &n,
			`SELECT COUNT(1) FROM collections
			  WHERE parent_id = ? AND LOWER(name) = LOWER(?) AND id != ?`,
			*parentID, name, excludeID)
	}
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Store) RenameCollection(ctx context.Context, id int64, name string) (library.Collection, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return library.Collection{}, fmt.Errorf("%w: name is required", ErrInvalidCollection)
	}
	c, err := s.GetCollection(ctx, id)
	if err != nil {
		return library.Collection{}, err
	}
	dup, err := s.nameTakenUnderParent(ctx, name, c.ParentID, id)
	if err != nil {
		return library.Collection{}, err
	}
	if dup {
		return library.Collection{}, fmt.Errorf("%w: name already exists under parent", ErrInvalidCollection)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE collections SET name = ? WHERE id = ?`, name, id); err != nil {
		return library.Collection{}, err
	}
	return s.GetCollection(ctx, id)
}

// MoveCollection updates a manual collection's parent. Rejects cycles.
func (s *Store) MoveCollection(ctx context.Context, id int64, newParentID *int64) error {
	c, err := s.GetCollection(ctx, id)
	if err != nil {
		return err
	}
	if newParentID != nil {
		if *newParentID == id {
			return fmt.Errorf("%w: cannot parent to self", ErrInvalidCollection)
		}
		// walk ancestors of newParentID; if we hit id, it's a cycle.
		cur := newParentID
		for cur != nil {
			if *cur == id {
				return fmt.Errorf("%w: cycle detected", ErrInvalidCollection)
			}
			var parent *int64
			err := s.db.GetContext(ctx, &parent,
				`SELECT parent_id FROM collections WHERE id = ?`, *cur)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: parent not found", ErrInvalidCollection)
			}
			if err != nil {
				return err
			}
			cur = parent
		}
	}
	dup, err := s.nameTakenUnderParent(ctx, c.Name, newParentID, id)
	if err != nil {
		return err
	}
	if dup {
		return fmt.Errorf("%w: name already exists under new parent", ErrInvalidCollection)
	}
	_, err = s.db.ExecContext(ctx, `UPDATE collections SET parent_id = ? WHERE id = ?`, newParentID, id)
	return err
}

func (s *Store) DeleteCollection(ctx context.Context, id int64) error {
	if _, err := s.GetCollection(ctx, id); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM collections WHERE id = ?`, id)
	return err
}

// UpsertScanCollection creates or refreshes a scan-source collection identified by folderPath.
func (s *Store) UpsertScanCollection(ctx context.Context, folderPath, name string, parentID *int64) (library.Collection, error) {
	existing, found, err := s.getCollectionByFolderPath(ctx, folderPath)
	if err != nil {
		return library.Collection{}, err
	}
	if found {
		// scan only seeds; never overwrite user changes to name/parent.
		return s.GetCollection(ctx, existing.ID)
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO collections (name, parent_id, source, folder_path)
		VALUES (?, ?, 'scan', ?)`, name, parentID, folderPath)
	if err != nil {
		return library.Collection{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return library.Collection{}, err
	}
	return s.GetCollection(ctx, id)
}

// AddBookToCollection links a book to a collection. Idempotent: re-linking is a no-op.
// Returns sql.ErrNoRows if either the book or the collection is missing.
func (s *Store) AddBookToCollection(ctx context.Context, bookPath string, collectionID int64) error {
	var n int
	if err := s.db.GetContext(ctx, &n,
		`SELECT COUNT(1) FROM books WHERE path = ? AND removed_at IS NULL`, bookPath); err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	if err := s.db.GetContext(ctx, &n,
		`SELECT COUNT(1) FROM collections WHERE id = ?`, collectionID); err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO book_collections (book_path, collection_id) VALUES (?, ?)
		ON CONFLICT DO NOTHING`, bookPath, collectionID)
	return err
}

// SyncScanCollectionForBook reconciles scan-derived collection links for a book.
// Manual collection memberships are preserved. If collectionID is nil, all scan
// collection links for the book are removed.
func (s *Store) SyncScanCollectionForBook(ctx context.Context, bookPath string, collectionID *int64) error {
	if collectionID == nil {
		_, err := s.db.ExecContext(ctx, `
			DELETE FROM book_collections
			 WHERE book_path = ?
			   AND collection_id IN (
			     SELECT id FROM collections WHERE source = 'scan'
			   )`, bookPath)
		return err
	}

	if _, err := s.db.ExecContext(ctx, `
		DELETE FROM book_collections
		 WHERE book_path = ?
		   AND collection_id IN (
		     SELECT id FROM collections WHERE source = 'scan' AND id != ?
		   )`, bookPath, *collectionID); err != nil {
		return err
	}

	return s.AddBookToCollection(ctx, bookPath, *collectionID)
}

// RemoveBookFromCollection unlinks a book from a collection. Returns sql.ErrNoRows
// if the link did not exist.
func (s *Store) RemoveBookFromCollection(ctx context.Context, bookPath string, collectionID int64) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM book_collections WHERE book_path = ? AND collection_id = ?`,
		bookPath, collectionID)
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

// BookCollectionIDs returns a map of book path → collection IDs the book belongs to.
func (s *Store) BookCollectionIDs(ctx context.Context) (map[string][]int64, error) {
	rows, err := s.db.QueryxContext(ctx, `
		SELECT book_path, collection_id FROM book_collections
		 ORDER BY book_path ASC, collection_id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]int64)
	for rows.Next() {
		var path string
		var id int64
		if err := rows.Scan(&path, &id); err != nil {
			return nil, err
		}
		out[path] = append(out[path], id)
	}
	return out, rows.Err()
}
