package main

import "net/http"

type ReviewHandler struct {
	service *ReviewService
}

func NewReviewHandler(service *ReviewService) *ReviewHandler {
	return &ReviewHandler{service: service}
}

func (h *ReviewHandler) Create(w http.ResponseWriter, r *http.Request) {
	exchangeID, ok := pathID(w, r)
	if !ok {
		return
	}
	authorID, err := authenticatedUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}
	var input CreateReviewInput
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"})
		return
	}
	review, err := h.service.Create(r.Context(), authorID, exchangeID, input)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, review)
}

func (h *ReviewHandler) ListUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathID(w, r)
	if !ok {
		return
	}
	reviews, err := h.service.ListUserReviews(r.Context(), userID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reviews)
}

func (h *ReviewHandler) ListService(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := pathID(w, r)
	if !ok {
		return
	}
	reviews, err := h.service.ListServiceReviews(r.Context(), serviceID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reviews)
}

type StatsHandler struct {
	service *StatsService
}

func NewStatsHandler(service *StatsService) *StatsHandler {
	return &StatsHandler{service: service}
}

func (h *StatsHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathID(w, r)
	if !ok {
		return
	}
	stats, err := h.service.Get(r.Context(), userID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
