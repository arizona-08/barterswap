package main

import (
	"context"
	"net/http"
)

type ServiceHandler struct {
	service *ServiceService
}

func NewServiceHandler(service *ServiceService) *ServiceHandler {
	return &ServiceHandler{service: service}
}

func (h *ServiceHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	services, err := h.service.List(r.Context(), ServiceFilters{
		Categorie: r.URL.Query().Get("categorie"),
		Ville:     r.URL.Query().Get("ville"),
		Search:    r.URL.Query().Get("search"),
		Limit: limit,
		Offset: offset,
	})
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, services)
}

func (h *ServiceHandler) Create(w http.ResponseWriter, r *http.Request) {
	providerID, err := authenticatedUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}
	var input CreateServiceInput
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"})
		return
	}
	service, err := h.service.Create(r.Context(), providerID, input)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, service)
}

func (h *ServiceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	service, err := h.service.Get(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, service)
}

func (h *ServiceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	providerID, err := authenticatedUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}
	var input UpdateServiceInput
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"})
		return
	}
	service, err := h.service.Update(r.Context(), providerID, id, input)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, service)
}

func (h *ServiceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	providerID, err := authenticatedUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}
	if err := h.service.Delete(r.Context(), providerID, id); err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type ExchangeHandler struct {
	service *ExchangeService
}

func NewExchangeHandler(service *ExchangeService) *ExchangeHandler {
	return &ExchangeHandler{service: service}
}

func (h *ExchangeHandler) Create(w http.ResponseWriter, r *http.Request) {
	requesterID, err := authenticatedUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}
	var input CreateExchangeInput
	if err := decodeJSON(r, &input); err != nil || input.ServiceID <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"})
		return
	}
	exchange, err := h.service.Create(r.Context(), requesterID, input.ServiceID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, exchange)
}

func (h *ExchangeHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticatedUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}
	limit, offset := parsePagination(r)
	
	exchanges, err := h.service.List(r.Context(), userID, r.URL.Query().Get("status"), limit, offset)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, exchanges)
}

func (h *ExchangeHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	userID, err := authenticatedUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}
	exchange, err := h.service.Get(r.Context(), userID, id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, exchange)
}

func (h *ExchangeHandler) Accept(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, h.service.Accept)
}

func (h *ExchangeHandler) Reject(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, h.service.Reject)
}

func (h *ExchangeHandler) Complete(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, h.service.Complete)
}

func (h *ExchangeHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, h.service.Cancel)
}

type exchangeAction func(context.Context, int, int) (Exchange, error)

func (h *ExchangeHandler) transition(w http.ResponseWriter, r *http.Request, action exchangeAction) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	userID, err := authenticatedUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}
	exchange, err := action(r.Context(), userID, id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, exchange)
}
