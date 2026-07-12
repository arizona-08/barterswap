package main

import (
	"encoding/json"
	"errors"
	"net/http"
)

type TestHandler struct {
	service *TestService
}

func NewTestHandler(service *TestService) *TestHandler {
	return &TestHandler{service: service}
}

func (h *TestHandler) Get(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.service.Get())
}

func (h *TestHandler) Patch(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var input PatchTestInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"})
		return
	}

	resource, err := h.service.Patch(input)
	if err != nil {
		if errors.Is(err, ErrEmptyMessage) {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, resource)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		// The headers are already written, so the connection log is the only
		// useful place to report an encoding failure in this small setup.
		return
	}
}
