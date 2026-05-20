package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"bookshelf/internal/store"
)

var errScanCollectionMembershipReadOnly = errors.New("scan collection membership is read-only")

func (s *Server) listCollections(w http.ResponseWriter, r *http.Request) {
	cs, err := s.store.ListCollections(r.Context())
	if err != nil {
		s.internal(w, r, "list collections", err)
		return
	}
	out := make([]CollectionDTO, 0, len(cs))
	for _, c := range cs {
		out = append(out, collectionToDTO(c))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createCollection(w http.ResponseWriter, r *http.Request) {
	var req CreateCollectionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid JSON: "+err.Error())
		return
	}
	c, err := s.store.CreateManualCollection(r.Context(), req.Name, req.ParentID)
	if errors.Is(err, store.ErrInvalidCollection) {
		writeError(w, http.StatusBadRequest, codeBadRequest, err.Error())
		return
	}
	if err != nil {
		s.internal(w, r, "create collection", err)
		return
	}
	writeJSON(w, http.StatusCreated, collectionToDTO(c))
}

func parseCollectionID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func (s *Server) ensureManualCollectionForMembership(ctx context.Context, id int64) error {
	c, err := s.store.GetCollection(ctx, id)
	if err != nil {
		return err
	}
	if c.Source == "scan" {
		return errScanCollectionMembershipReadOnly
	}
	return nil
}

func (s *Server) patchCollection(w http.ResponseWriter, r *http.Request) {
	id, err := parseCollectionID(r)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid collection id")
		return
	}
	var req UpdateCollectionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid JSON: "+err.Error())
		return
	}
	ctx := r.Context()
	if _, err := s.store.GetCollection(ctx, id); errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, codeNotFound, "collection not found")
		return
	} else if err != nil {
		s.internal(w, r, "get collection", err)
		return
	}
	if req.Name != nil {
		if _, err := s.store.RenameCollection(ctx, id, *req.Name); err != nil {
			if errors.Is(err, store.ErrInvalidCollection) {
				writeError(w, http.StatusBadRequest, codeBadRequest, err.Error())
				return
			}
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, codeNotFound, "collection not found")
				return
			}
			s.internal(w, r, "rename collection", err)
			return
		}
	}
	if req.ChangeParent {
		if err := s.store.MoveCollection(ctx, id, req.ParentID); err != nil {
			if errors.Is(err, store.ErrInvalidCollection) {
				writeError(w, http.StatusBadRequest, codeBadRequest, err.Error())
				return
			}
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, codeNotFound, "collection not found")
				return
			}
			s.internal(w, r, "move collection", err)
			return
		}
	}
	updated, err := s.store.GetCollection(ctx, id)
	if err != nil {
		s.internal(w, r, "get collection", err)
		return
	}
	writeJSON(w, http.StatusOK, collectionToDTO(updated))
}

func (s *Server) deleteCollection(w http.ResponseWriter, r *http.Request) {
	id, err := parseCollectionID(r)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid collection id")
		return
	}
	ctx := r.Context()
	if err := s.store.DeleteCollection(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, codeNotFound, "collection not found")
			return
		}
		s.internal(w, r, "delete collection", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) addBookToCollection(w http.ResponseWriter, r *http.Request) {
	id, err := parseCollectionID(r)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid collection id")
		return
	}
	if err := s.ensureManualCollectionForMembership(r.Context(), id); err != nil {
		if errors.Is(err, errScanCollectionMembershipReadOnly) {
			writeError(w, http.StatusConflict, codeConflict, err.Error())
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, codeNotFound, "book or collection not found")
			return
		}
		s.internal(w, r, "get collection", err)
		return
	}
	var req AddBookToCollectionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, codeBadRequest, "missing book path")
		return
	}
	if err := s.store.AddBookToCollection(r.Context(), req.Path, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, codeNotFound, "book or collection not found")
			return
		}
		s.internal(w, r, "add book to collection", err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) removeBookFromCollection(w http.ResponseWriter, r *http.Request) {
	id, err := parseCollectionID(r)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid collection id")
		return
	}
	if err := s.ensureManualCollectionForMembership(r.Context(), id); err != nil {
		if errors.Is(err, errScanCollectionMembershipReadOnly) {
			writeError(w, http.StatusConflict, codeConflict, err.Error())
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, codeNotFound, "link not found")
			return
		}
		s.internal(w, r, "get collection", err)
		return
	}
	bookPath := chi.URLParam(r, "*")
	if decoded, err := decodeIfEncoded(bookPath); err == nil {
		bookPath = decoded
	}
	if bookPath == "" {
		writeError(w, http.StatusBadRequest, codeBadRequest, "missing book path")
		return
	}
	if err := s.store.RemoveBookFromCollection(r.Context(), bookPath, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, codeNotFound, "link not found")
			return
		}
		s.internal(w, r, "remove book from collection", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
