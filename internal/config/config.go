package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	// LibraryDir is a bootstrap default. If non-empty and no library_dir setting
	// exists in the database on startup, it seeds the setting. Otherwise ignored.
	LibraryDir string
	DBPath     string
	Listen     string
}

func FromEnv() (Config, error) {
	c := Config{
		LibraryDir: os.Getenv("BOOKSHELF_LIBRARY_DIR"),
		DBPath:     os.Getenv("BOOKSHELF_DB_PATH"),
		Listen:     os.Getenv("BOOKSHELF_LISTEN"),
	}
	if c.Listen == "" {
		c.Listen = ":19320"
	}
	if c.DBPath == "" {
		return c, errors.New("BOOKSHELF_DB_PATH is required")
	}

	if c.LibraryDir != "" {
		abs, err := filepath.Abs(c.LibraryDir)
		if err != nil {
			return c, fmt.Errorf("library dir: %w", err)
		}
		// only validate when set; missing path is tolerated since the setting
		// can be updated via the API.
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			c.LibraryDir = abs
		} else {
			c.LibraryDir = ""
		}
	}

	dbAbs, err := filepath.Abs(c.DBPath)
	if err != nil {
		return c, fmt.Errorf("db path: %w", err)
	}
	if _, err := os.Stat(filepath.Dir(dbAbs)); err != nil {
		return c, fmt.Errorf("db parent dir %q: %w", filepath.Dir(dbAbs), err)
	}
	c.DBPath = dbAbs

	return c, nil
}
