package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func buildUpload(t *testing.T, filename string, content []byte, collectionIDs []string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, id := range collectionIDs {
		if err := mw.WriteField("collectionIds", id); err != nil {
			t.Fatalf("write collectionIds: %v", err)
		}
	}
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	if _, err := io.Copy(fw, bytes.NewReader(content)); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close mw: %v", err)
	}
	return &buf, mw.FormDataContentType()
}

func TestUploadBookSuccess(t *testing.T) {
	_, _, h, libDir := newTestServer(t)
	body, ct := buildUpload(t, "Hello.pdf", []byte("%PDF-1.4 fake content"), nil)
	req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var dto BookDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dto.Path != "Hello.pdf" {
		t.Fatalf("path: %q", dto.Path)
	}
	if dto.Title != "Hello" {
		t.Fatalf("title: %q", dto.Title)
	}
	if !strings.HasPrefix(dto.Fingerprint, "sha256:") {
		t.Fatalf("fingerprint: %q", dto.Fingerprint)
	}
	if _, err := os.Stat(filepath.Join(libDir, "Hello.pdf")); err != nil {
		t.Fatalf("file missing on disk: %v", err)
	}
	entries, err := os.ReadDir(libDir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp.") {
			t.Fatalf("leftover temp file: %s", e.Name())
		}
	}
}

func TestUploadBookConflict(t *testing.T) {
	_, _, h, libDir := newTestServer(t)
	if err := os.WriteFile(filepath.Join(libDir, "Existing.pdf"), []byte("%PDF-1.4 original"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	body, ct := buildUpload(t, "Existing.pdf", []byte("%PDF-1.4 new"), nil)
	req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUploadBookLibraryNotConfigured(t *testing.T) {
	_, h := newUnconfiguredServer(t)
	body, ct := buildUpload(t, "Hello.pdf", []byte("%PDF-1.4"), nil)
	req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUploadBookNonPDF(t *testing.T) {
	_, _, h, _ := newTestServer(t)
	body, ct := buildUpload(t, "image.png", []byte("not a pdf"), nil)
	req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUploadBookWithCollectionIDs(t *testing.T) {
	st, _, h, _ := newTestServer(t)
	ctx := context.Background()
	c, err := st.CreateManualCollection(ctx, "Favs", nil)
	if err != nil {
		t.Fatalf("create coll: %v", err)
	}
	body, ct := buildUpload(t, "Linked.pdf", []byte("%PDF-1.4 content"), []string{itoa(c.ID)})
	req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var dto BookDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &dto)
	found := false
	for _, id := range dto.CollectionIDs {
		if id == c.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected collection %d in %+v", c.ID, dto.CollectionIDs)
	}
}
