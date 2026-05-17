package api

import (
	"encoding/json"
	"net/http"
)

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const (
	codeBadRequest = "bad_request"
	codeNotFound   = "not_found"
	codeConflict   = "conflict"
	codeInternal   = "internal"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, errorBody{Error: errorPayload{Code: code, Message: msg}})
}
