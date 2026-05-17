package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrInvalidSetting is returned when SetLibraryDir is given a path that does
// not exist or is not a directory. Callers should use errors.Is to detect it.
var ErrInvalidSetting = errors.New("invalid setting")

const settingLibraryDir = "library_dir"

// GetSetting returns the value of a meta key. found is false if no row exists.
func (s *Store) GetSetting(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := s.db.GetContext(ctx, &v, `SELECT value FROM meta WHERE key = ?`, key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// SetSetting upserts a meta key.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

// DeleteSetting removes a meta row. No-op if absent.
func (s *Store) DeleteSetting(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM meta WHERE key = ?`, key)
	return err
}

// GetLibraryDir returns the configured library directory (absolute path) or
// the empty string if no library is configured.
func (s *Store) GetLibraryDir(ctx context.Context) (string, error) {
	v, _, err := s.GetSetting(ctx, settingLibraryDir)
	return v, err
}

// SetLibraryDir validates the path and persists it. An empty string clears
// the setting. The path must exist and be a directory; otherwise the returned
// error wraps ErrInvalidSetting.
func (s *Store) SetLibraryDir(ctx context.Context, path string) error {
	if path == "" {
		return s.DeleteSetting(ctx, settingLibraryDir)
	}
	path = normalizePath(path)
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSetting, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSetting, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %q is not a directory", ErrInvalidSetting, abs)
	}
	return s.SetSetting(ctx, settingLibraryDir, abs)
}

// normalizePath rewrites MSYS/Cygwin-style paths (e.g. "/c/Users/me/Books") to
// native Windows form ("C:\Users\me\Books") when running on Windows. On other
// platforms it returns path unchanged.
func normalizePath(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}
	// match /<letter>/... or /<letter> exactly
	if len(path) >= 2 && path[0] == '/' && isDriveLetter(path[1]) &&
		(len(path) == 2 || path[2] == '/') {
		drive := strings.ToUpper(string(path[1])) + ":"
		rest := path[2:]
		if rest == "" {
			rest = "\\"
		}
		return drive + filepath.FromSlash(rest)
	}
	return path
}

func isDriveLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
