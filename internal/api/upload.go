package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bookshelf/internal/library"
)

const (
	maxUploadBytes          = 200 << 20
	uploadFingerprintBytes  = 64 * 1024
)

// uploadBook handles POST /api/upload — a multipart form with a `file` PDF blob
// and optional `collectionIds` (repeated form field, integer ids) to link the
// newly indexed book to. The file is written to the library root.
func (s *Server) uploadBook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	libDir, err := s.store.GetLibraryDir(ctx)
	if err != nil {
		s.internal(w, r, "get library_dir", err)
		return
	}
	if libDir == "" {
		writeError(w, http.StatusBadRequest, codeBadRequest, "library not configured")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid upload: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "missing file field")
		return
	}
	defer file.Close()
	if header.Size == 0 {
		writeError(w, http.StatusBadRequest, codeBadRequest, "empty file")
		return
	}

	origName := filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	if origName == "" || origName == "." || origName == ".." {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid filename")
		return
	}
	if !strings.EqualFold(filepath.Ext(origName), ".pdf") {
		writeError(w, http.StatusBadRequest, codeBadRequest, "only PDF files are accepted")
		return
	}

	libAbs, err := filepath.Abs(libDir)
	if err != nil {
		s.internal(w, r, "abs library dir", err)
		return
	}
	targetAbs := filepath.Clean(filepath.Join(libAbs, origName))
	rel, err := filepath.Rel(libAbs, targetAbs)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid filename")
		return
	}

	if _, err := os.Stat(targetAbs); err == nil {
		writeError(w, http.StatusConflict, codeConflict, "file already exists")
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		s.internal(w, r, "stat target", err)
		return
	}

	tmpPath, err := streamToTemp(file, targetAbs)
	if err != nil {
		s.internal(w, r, "write upload", err)
		return
	}
	if err := os.Rename(tmpPath, targetAbs); err != nil {
		_ = os.Remove(tmpPath)
		s.internal(w, r, "rename upload", err)
		return
	}

	info, err := os.Stat(targetAbs)
	if err != nil {
		s.internal(w, r, "stat written file", err)
		return
	}
	fp, err := fingerprintFile(targetAbs)
	if err != nil {
		s.internal(w, r, "fingerprint", err)
		return
	}
	book := library.Book{
		Path:        origName,
		Category:    "Uncategorized",
		Title:       strings.TrimSuffix(origName, filepath.Ext(origName)),
		SizeBytes:   info.Size(),
		Fingerprint: fp,
		AddedAt:     time.Now().UTC(),
	}
	if _, err := s.store.UpsertBook(ctx, book); err != nil {
		s.internal(w, r, "upsert book", err)
		return
	}

	for _, idStr := range r.Form["collectionIds"] {
		for _, part := range strings.Split(idStr, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, perr := strconv.ParseInt(part, 10, 64)
			if perr != nil || id <= 0 {
				continue
			}
			if err := s.store.AddBookToCollection(ctx, origName, id); err != nil {
				s.log.Warn("link upload to collection", "id", id, "err", err)
			}
		}
	}

	stored, err := s.store.GetBook(ctx, origName)
	if err != nil {
		s.internal(w, r, "get book after upload", err)
		return
	}
	collMap, err := s.store.BookCollectionIDs(ctx)
	if err != nil {
		s.internal(w, r, "book collection ids", err)
		return
	}
	ids := collMap[stored.Path]
	if ids == nil {
		ids = []int64{}
	}
	writeJSON(w, http.StatusCreated, BookDTO{
		Path:          stored.Path,
		Title:         stored.Title,
		Category:      stored.Category,
		SizeBytes:     stored.SizeBytes,
		Fingerprint:   stored.Fingerprint,
		AddedAt:       stored.AddedAt,
		Bookmarks:     []BookmarkDTO{},
		CollectionIDs: ids,
	})
}

// streamToTemp writes src to a sibling temp file of finalPath and returns the
// temp path on success. On error the temp file is removed.
func streamToTemp(src io.Reader, finalPath string) (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	tmpPath := finalPath + ".tmp." + hex.EncodeToString(buf[:])
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, src); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
}

func fingerprintFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.CopyN(h, f, uploadFingerprintBytes); err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
