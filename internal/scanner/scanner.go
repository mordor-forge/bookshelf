package scanner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bookshelf/internal/library"
	"bookshelf/internal/store"
)

const fingerprintBytes = 64 * 1024

var (
	ErrAlreadyRunning = errors.New("scan already in progress")
	ErrNotConfigured  = errors.New("library not configured")
)

type Result struct {
	StartedAt  time.Time
	FinishedAt time.Time
	Added      int
	Updated    int
	Removed    int
	Errors     []string
}

type Scanner struct {
	store *store.Store
	log   *slog.Logger

	mu      sync.Mutex
	running bool
	last    Result
}

func New(st *store.Store, log *slog.Logger) *Scanner {
	return &Scanner{store: st, log: log}
}

// Run performs one scan against libraryDir. Returns ErrAlreadyRunning if a scan
// is in flight, or ErrNotConfigured if libraryDir is empty.
func (s *Scanner) Run(ctx context.Context, libraryDir string) (Result, error) {
	if libraryDir == "" {
		return Result{}, ErrNotConfigured
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return Result{}, ErrAlreadyRunning
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	res := s.scan(ctx, libraryDir)
	s.mu.Lock()
	s.last = res
	s.mu.Unlock()
	return res, nil
}

// TryRun starts a scan in a goroutine. Returns ErrAlreadyRunning immediately if a scan
// is in flight; ErrNotConfigured if libraryDir is empty; otherwise returns nil and the
// scan continues in the background.
func (s *Scanner) TryRun(ctx context.Context, libraryDir string) error {
	if libraryDir == "" {
		return ErrNotConfigured
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrAlreadyRunning
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}()
		res := s.scan(ctx, libraryDir)
		s.mu.Lock()
		s.last = res
		s.mu.Unlock()
	}()
	return nil
}

func (s *Scanner) scan(ctx context.Context, libraryDir string) Result {
	res := Result{StartedAt: time.Now().UTC()}
	seen := make(map[string]struct{}, 256)
	// folder_path → collection id, populated during the walk.
	collIDs := make(map[string]int64, 64)

	err := filepath.WalkDir(libraryDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", path, walkErr))
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			// skip the library root itself.
			if path == libraryDir {
				return nil
			}
			if shouldSkipDir(d.Name()) {
				return fs.SkipDir
			}
			rel, err := filepath.Rel(libraryDir, path)
			if err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("rel %s: %v", path, err))
				return nil
			}
			relSlash := filepath.ToSlash(rel)
			var parentID *int64
			if i := strings.LastIndexByte(relSlash, '/'); i > 0 {
				parentSlash := relSlash[:i]
				if pid, ok := collIDs[parentSlash]; ok {
					id := pid
					parentID = &id
				}
			}
			c, err := s.store.UpsertScanCollection(ctx, relSlash, d.Name(), parentID)
			if err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("collection %s: %v", relSlash, err))
				return nil
			}
			collIDs[relSlash] = c.ID
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".pdf") {
			return nil
		}
		rel, err := filepath.Rel(libraryDir, path)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("rel %s: %v", path, err))
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		info, err := d.Info()
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("stat %s: %v", relSlash, err))
			return nil
		}
		fp, err := fingerprint(path)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("fingerprint %s: %v", relSlash, err))
			return nil
		}
		book := library.Book{
			Path:        relSlash,
			Category:    categoryFor(relSlash),
			Title:       strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())),
			SizeBytes:   info.Size(),
			Fingerprint: fp,
			AddedAt:     time.Now().UTC(),
		}
		inserted, err := s.store.UpsertBook(ctx, book)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("upsert %s: %v", relSlash, err))
			return nil
		}
		if inserted {
			res.Added++
		} else {
			res.Updated++
		}
		// reconcile scan-derived links so the book belongs to exactly its
		// immediate parent scan collection, while preserving any manual links.
		var parentID *int64
		if i := strings.LastIndexByte(relSlash, '/'); i > 0 {
			parentSlash := relSlash[:i]
			if cid, ok := collIDs[parentSlash]; ok {
				id := cid
				parentID = &id
			}
		}
		if err := s.store.SyncScanCollectionForBook(ctx, relSlash, parentID); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("link %s: %v", relSlash, err))
		}
		seen[relSlash] = struct{}{}
		return ctx.Err()
	})
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		res.FinishedAt = time.Now().UTC()
		return res
	}

	live, err := s.store.ListLivePaths(ctx)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		res.FinishedAt = time.Now().UTC()
		return res
	}
	missing := make([]string, 0)
	for _, p := range live {
		if _, ok := seen[p]; !ok {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		n, err := s.store.MarkRemoved(ctx, missing, time.Now().UTC())
		if err != nil {
			res.Errors = append(res.Errors, err.Error())
		} else {
			res.Removed = n
		}
	}
	res.FinishedAt = time.Now().UTC()
	s.log.Info("scan finished",
		"added", res.Added, "updated", res.Updated, "removed", res.Removed,
		"errors", len(res.Errors), "duration", res.FinishedAt.Sub(res.StartedAt))
	return res
}

func (s *Scanner) Last() Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}

func (s *Scanner) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// shouldSkipDir reports whether a directory name is a known junk dir (VCS,
// dependency caches, OS metadata) that should be excluded from the library walk.
func shouldSkipDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "node_modules", "__pycache__", "venv", ".venv", "target",
		"vendor", "dist", "build", "out", "$RECYCLE.BIN",
		"System Volume Information", "Thumbs.db":
		return true
	}
	return false
}

func categoryFor(relSlash string) string {
	if i := strings.IndexByte(relSlash, '/'); i > 0 {
		return relSlash[:i]
	}
	return "Uncategorized"
}

func fingerprint(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.CopyN(h, f, fingerprintBytes); err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
