package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
)

type UserHandler struct {
	service *UserService
}

func NewUserHandler(service *UserService) *UserHandler {
	return &UserHandler{service: service}
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input CreateUserInput
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"})
		return
	}
	user, err := h.service.Create(r.Context(), input)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	user, err := h.service.Get(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	authenticatedID, err := authenticatedUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}
	var input UpdateUserInput
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"})
		return
	}
	user, err := h.service.Update(r.Context(), authenticatedID, id, input)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *UserHandler) GetSkills(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	skills, err := h.service.Skills(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, skills)
}

func (h *UserHandler) ReplaceSkills(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	authenticatedID, err := authenticatedUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}
	var input ReplaceSkillsInput
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"})
		return
	}
	skills, err := h.service.ReplaceSkills(r.Context(), authenticatedID, id, input.Skills)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, skills)
}

func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid user id"})
		return 0, false
	}
	return id, true
}

func authenticatedUserID(r *http.Request) (int, error) {
	id, ok := r.Context().Value(userIDContextKey).(int)
	if !ok || id <= 0 {
		return 0, ErrUnauthenticated
	}
	return id, nil
}

func decodeJSON(r *http.Request, destination any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("body must contain one JSON object")
	}
	return nil
}

func handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrEmptyPseudo), errors.Is(err, ErrInvalidSkill), errors.Is(err, ErrDuplicateSkill), errors.Is(err, ErrInvalidLevel),
		errors.Is(err, ErrInvalidService), errors.Is(err, ErrInvalidCategory), errors.Is(err, ErrProviderLacksSkill),
		errors.Is(err, ErrInvalidExchangeStatus), errors.Is(err, ErrSelfExchange), errors.Is(err, ErrInsufficientCredits),
		errors.Is(err, ErrInvalidNote), errors.Is(err, ErrExchangeNotCompleted), errors.Is(err, ErrReviewNotAllowed),
		errors.Is(err, ErrReviewAlreadyExists):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	case errors.Is(err, ErrUnauthenticated):
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: err.Error()})
	case errors.Is(err, ErrForbidden):
		writeJSON(w, http.StatusForbidden, ErrorResponse{Error: err.Error()})
	case errors.Is(err, ErrUserNotFound):
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: err.Error()})
	case errors.Is(err, ErrServiceNotFound), errors.Is(err, ErrExchangeNotFound), errors.Is(err, ErrReviewNotFound):
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: err.Error()})
	case errors.Is(err, ErrServiceReserved):
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: err.Error()})
	case errors.Is(err, ErrServiceInactive), errors.Is(err, ErrInvalidExchangeState):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

