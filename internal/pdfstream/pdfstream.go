package pdfstream

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Handler returns an http.Handler that serves raw PDF bytes. The library directory
// is resolved per-request via getLibraryDir; if it returns the empty string the
// handler responds 404 with the standard error envelope (library not configured).
// Uses http.ServeContent for Range / If-Modified-Since / ETag support.
func Handler(getLibraryDir func() string, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		libraryDir := getLibraryDir()
		if libraryDir == "" {
			writeJSONError(w, http.StatusNotFound, "not_found", "library not configured")
			return
		}
		absLibrary, err := filepath.Abs(libraryDir)
		if err != nil {
			absLibrary = libraryDir
		}
		rel := strings.TrimPrefix(r.URL.Path, "/")
		if rel == "" {
			http.NotFound(w, r)
			return
		}
		// chi has already URL-decoded r.URL.Path for percent-encoded sequences.
		fsPath := filepath.Join(absLibrary, filepath.FromSlash(rel))
		absPath, err := filepath.Abs(fsPath)
		if err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		// ensure absPath stays under libraryDir
		if !strings.HasPrefix(absPath, absLibrary+string(filepath.Separator)) && absPath != absLibrary {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		f, err := os.Open(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			if log != nil {
				log.Error("pdf open", "path", absPath, "err", err)
			}
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
		if info.IsDir() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	})
}

func writeJSONError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": msg},
	})
}
