package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeServiceStore struct {
	service Service
	skills  map[int]map[string]bool
}

func (s *fakeServiceStore) CreateService(_ context.Context, providerID int, input CreateServiceInput) (Service, error) {
	s.service = Service{ID: 1, ProviderID: providerID, Titre: input.Titre, Categorie: input.Categorie, DureeMinutes: input.DureeMinutes, Credits: input.Credits, Actif: true}
	return s.service, nil
}

func (s *fakeServiceStore) GetService(_ context.Context, id int) (Service, error) {
	if s.service.ID != id {
		return Service{}, ErrServiceNotFound
	}
	return s.service, nil
}

func (s *fakeServiceStore) ListServices(_ context.Context, _ ServiceFilters) ([]Service, error) {
	if s.service.ID == 0 {
		return []Service{}, nil
	}
	return []Service{s.service}, nil
}

func (s *fakeServiceStore) UpdateService(_ context.Context, id int, input UpdateServiceInput) (Service, error) {
	if s.service.ID != id {
		return Service{}, ErrServiceNotFound
	}
	s.service.Titre, s.service.Categorie = input.Titre, input.Categorie
	s.service.DureeMinutes, s.service.Credits, s.service.Actif = input.DureeMinutes, input.Credits, input.Actif
	return s.service, nil
}

func (s *fakeServiceStore) DeleteService(_ context.Context, id int) error {
	if s.service.ID != id {
		return ErrServiceNotFound
	}
	s.service = Service{}
	return nil
}

func (s *fakeServiceStore) HasSkill(_ context.Context, userID int, skill string) (bool, error) {
	return s.skills[userID][skill], nil
}

func newMarketMux(serviceStore ServiceStore, exchangeStore ExchangeStore) http.Handler {
	serviceHandler := NewServiceHandler(NewServiceService(serviceStore))
	exchangeHandler := NewExchangeHandler(NewExchangeService(exchangeStore))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/services", serviceHandler.List)
	mux.HandleFunc("POST /api/services", serviceHandler.Create)
	mux.HandleFunc("GET /api/services/{id}", serviceHandler.Get)
	mux.HandleFunc("PUT /api/services/{id}", serviceHandler.Update)
	mux.HandleFunc("DELETE /api/services/{id}", serviceHandler.Delete)
	mux.HandleFunc("POST /api/exchanges", exchangeHandler.Create)
	mux.HandleFunc("GET /api/exchanges", exchangeHandler.List)
	mux.HandleFunc("GET /api/exchanges/{id}", exchangeHandler.Get)
	mux.HandleFunc("PUT /api/exchanges/{id}/accept", exchangeHandler.Accept)
	mux.HandleFunc("PUT /api/exchanges/{id}/reject", exchangeHandler.Reject)
	mux.HandleFunc("PUT /api/exchanges/{id}/complete", exchangeHandler.Complete)
	mux.HandleFunc("PUT /api/exchanges/{id}/cancel", exchangeHandler.Cancel)
	return authMiddleware(mux)
}

func TestCreateServiceRequiresProviderSkill(t *testing.T) {
	store := &fakeServiceStore{skills: map[int]map[string]bool{1: {"Jardinage": true}}}
	mux := newMarketMux(store, &fakeExchangeStore{})

	request := httptest.NewRequest(http.MethodPost, "/api/services", strings.NewReader(`{"titre":"Aide","categorie":"Jardinage","duree_minutes":60,"credits":1}`))
	request.Header.Set("X-User-ID", "1")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/services", strings.NewReader(`{"titre":"Aide","categorie":"Cuisine","duree_minutes":60,"credits":1}`))
	request.Header.Set("X-User-ID", "1")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing skill, got %d", response.Code)
	}
}

func TestServiceOwnerAndFilters(t *testing.T) {
	store := &fakeServiceStore{service: Service{ID: 1, ProviderID: 1, Titre: "Aide", Categorie: "Jardinage", DureeMinutes: 60, Credits: 1, Actif: true}, skills: map[int]map[string]bool{1: {"Jardinage": true}}}
	mux := newMarketMux(store, &fakeExchangeStore{})

	request := httptest.NewRequest(http.MethodPut, "/api/services/1", strings.NewReader(`{"titre":"Nouveau","categorie":"Jardinage","duree_minutes":60,"credits":1,"actif":true}`))
	request.Header.Set("X-User-ID", "2")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner, got %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/services?categorie=Jardinage", nil)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"titre":"Aide"`) {
		t.Fatalf("unexpected service list: %d %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/services/1", nil)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200 for service get, got %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodDelete, "/api/services/1", nil)
	request.Header.Set("X-User-ID", "1")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for service delete, got %d", response.Code)
	}
}

type fakeExchangeStore struct {
	exchange Exchange
}

func (s *fakeExchangeStore) CreateExchange(_ context.Context, requesterID, serviceID int) (Exchange, error) {
	s.exchange = Exchange{ID: 1, ServiceID: serviceID, RequesterID: requesterID, OwnerID: 1, Status: ExchangePending}
	return s.exchange, nil
}

func (s *fakeExchangeStore) ListExchanges(_ context.Context, _ int, _ string) ([]Exchange, error) {
	return []Exchange{s.exchange}, nil
}

func (s *fakeExchangeStore) GetExchange(_ context.Context, id int) (Exchange, error) {
	if s.exchange.ID != id {
		return Exchange{}, ErrExchangeNotFound
	}
	return s.exchange, nil
}

func (s *fakeExchangeStore) AcceptExchange(_ context.Context, id, _ int) (Exchange, error) {
	if s.exchange.ID != id {
		return Exchange{}, ErrExchangeNotFound
	}
	s.exchange.Status = ExchangeAccepted
	return s.exchange, nil
}

func (s *fakeExchangeStore) RejectExchange(_ context.Context, id, _ int) (Exchange, error) {
	if s.exchange.ID != id {
		return Exchange{}, ErrExchangeNotFound
	}
	s.exchange.Status = ExchangeRejected
	return s.exchange, nil
}

func (s *fakeExchangeStore) CompleteExchange(_ context.Context, id, _ int) (Exchange, error) {
	if s.exchange.ID != id {
		return Exchange{}, ErrExchangeNotFound
	}
	s.exchange.Status = ExchangeCompleted
	return s.exchange, nil
}

func (s *fakeExchangeStore) CancelExchange(_ context.Context, id, _ int) (Exchange, error) {
	if s.exchange.ID != id {
		return Exchange{}, ErrExchangeNotFound
	}
	s.exchange.Status = ExchangeCancelled
	return s.exchange, nil
}

func TestExchangeLifecycleHandlers(t *testing.T) {
	serviceStore := &fakeServiceStore{}
	exchangeStore := &fakeExchangeStore{}
	mux := newMarketMux(serviceStore, exchangeStore)

	request := httptest.NewRequest(http.MethodPost, "/api/exchanges", strings.NewReader(`{"service_id":4}`))
	request.Header.Set("X-User-ID", "2")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPut, "/api/exchanges/1/accept", nil)
	request.Header.Set("X-User-ID", "1")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"accepted"`) {
		t.Fatalf("unexpected accept response: %d %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/exchanges?status=accepted", nil)
	request.Header.Set("X-User-ID", "2")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200 for exchange list, got %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/exchanges/1", nil)
	request.Header.Set("X-User-ID", "2")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200 for exchange get, got %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPut, "/api/exchanges/1/complete", nil)
	request.Header.Set("X-User-ID", "2")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"completed"`) {
		t.Fatalf("unexpected complete response: %d %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/exchanges", strings.NewReader(`{"service_id":5}`))
	request.Header.Set("X-User-ID", "2")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	request = httptest.NewRequest(http.MethodPut, "/api/exchanges/1/reject", nil)
	request.Header.Set("X-User-ID", "1")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"rejected"`) {
		t.Fatalf("unexpected reject response: %d %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/exchanges", strings.NewReader(`{"service_id":6}`))
	request.Header.Set("X-User-ID", "2")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	request = httptest.NewRequest(http.MethodPut, "/api/exchanges/1/cancel", nil)
	request.Header.Set("X-User-ID", "2")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"cancelled"`) {
		t.Fatalf("unexpected cancel response: %d %s", response.Code, response.Body.String())
	}
}

func TestInvalidExchangeStatus(t *testing.T) {
	service := NewExchangeService(&fakeExchangeStore{})
	_, err := service.List(context.Background(), 1, "unknown")
	if err != ErrInvalidExchangeStatus {
		t.Fatalf("expected invalid status error, got %v", err)
	}
}
