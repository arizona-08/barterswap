package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeReviewStore struct {
	reviews []Review
}

func (s *fakeReviewStore) CreateReview(_ context.Context, exchangeID, authorID, targetID, note int, commentaire string) (Review, error) {
	review := Review{ID: len(s.reviews) + 1, ExchangeID: exchangeID, AuthorID: authorID, TargetID: targetID, Note: note, Commentaire: commentaire}
	s.reviews = append(s.reviews, review)
	return review, nil
}

func (s *fakeReviewStore) HasReview(_ context.Context, exchangeID, authorID int) (bool, error) {
	for _, review := range s.reviews {
		if review.ExchangeID == exchangeID && review.AuthorID == authorID {
			return true, nil
		}
	}
	return false, nil
}

func (s *fakeReviewStore) ListUserReviews(_ context.Context, userID int) ([]Review, error) {
	result := make([]Review, 0)
	for _, review := range s.reviews {
		if review.TargetID == userID {
			result = append(result, review)
		}
	}
	return result, nil
}

func (s *fakeReviewStore) ListServiceReviews(_ context.Context, _ int) ([]Review, error) {
	return s.reviews, nil
}

type fakeStatsStore struct {
	stats UserStats
}

func (s *fakeStatsStore) GetUserStats(_ context.Context, userID int) (UserStats, error) {
	s.stats.UserID = userID
	return s.stats, nil
}

func newReviewStatsMux(reviewService *ReviewService, statsService *StatsService) http.Handler {
	reviewHandler := NewReviewHandler(reviewService)
	statsHandler := NewStatsHandler(statsService)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/exchanges/{id}/review", reviewHandler.Create)
	mux.HandleFunc("GET /api/users/{id}/reviews", reviewHandler.ListUser)
	mux.HandleFunc("GET /api/services/{id}/reviews", reviewHandler.ListService)
	mux.HandleFunc("GET /api/users/{id}/stats", statsHandler.Get)
	return authMiddleware(mux)
}

func TestReviewRulesAndRoutes(t *testing.T) {
	reviewStore := &fakeReviewStore{}
	exchangeStore := &fakeExchangeStore{exchange: Exchange{ID: 1, ServiceID: 1, RequesterID: 2, OwnerID: 1, Status: ExchangeCompleted}}
	users := &fakeUserStore{user: User{ID: 1, Pseudo: "Alice"}}
	services := &fakeServiceStore{service: Service{ID: 1, ProviderID: 1}}
	reviews := NewReviewService(reviewStore, exchangeStore, users, services)
	stats := NewStatsService(&fakeStatsStore{stats: UserStats{CreditBalance: 10, NoteMoyenne: 4.5}})
	mux := newReviewStatsMux(reviews, stats)

	request := httptest.NewRequest(http.MethodPost, "/api/exchanges/1/review", strings.NewReader(`{"note":5,"commentaire":"Très bien"}`))
	request.Header.Set("X-User-ID", "2")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"target_id":1`) {
		t.Fatalf("unexpected review response: %d %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/exchanges/1/review", strings.NewReader(`{"note":4}`))
	request.Header.Set("X-User-ID", "2")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected duplicate review to be 400, got %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/exchanges/1/review", strings.NewReader(`{"note":0}`))
	request.Header.Set("X-User-ID", "1")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid note to be 400, got %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/users/1/reviews", nil)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"note":5`) {
		t.Fatalf("unexpected user reviews: %d %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/services/1/reviews", nil)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected service reviews 200, got %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/users/1/stats", nil)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"note_moyenne":4.5`) {
		t.Fatalf("unexpected stats response: %d %s", response.Code, response.Body.String())
	}
}

func TestReviewRequiresCompletedExchangeAndParticipant(t *testing.T) {
	reviewStore := &fakeReviewStore{}
	exchangeStore := &fakeExchangeStore{exchange: Exchange{ID: 1, RequesterID: 2, OwnerID: 1, Status: ExchangePending}}
	service := NewReviewService(reviewStore, exchangeStore, &fakeUserStore{user: User{ID: 1}}, &fakeServiceStore{service: Service{ID: 1}})

	if _, err := service.Create(context.Background(), 2, 1, CreateReviewInput{Note: 5}); err != ErrExchangeNotCompleted {
		t.Fatalf("expected incomplete exchange error, got %v", err)
	}
	exchangeStore.exchange.Status = ExchangeCompleted
	if _, err := service.Create(context.Background(), 3, 1, CreateReviewInput{Note: 5}); err != ErrReviewNotAllowed {
		t.Fatalf("expected participant error, got %v", err)
	}
}

func TestSimpleMiddlewares(t *testing.T) {
	panicHandler := recoveryMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("test panic")
	}))
	response := httptest.NewRecorder()
	panicHandler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected recovered panic to return 500, got %d", response.Code)
	}

	response = httptest.NewRecorder()
	corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(response, httptest.NewRequest(http.MethodOptions, "/", nil))
	if response.Code != http.StatusNoContent || response.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("unexpected CORS response: %d %q", response.Code, response.Header().Get("Access-Control-Allow-Origin"))
	}

	response = httptest.NewRecorder()
	loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/log", nil))
	if response.Code != http.StatusAccepted {
		t.Fatalf("expected logging middleware to preserve status, got %d", response.Code)
	}
}

func TestAuthMiddleware(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := authenticatedUserID(r)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"id": id})
	})

	handlerUnderTest := authMiddleware(dummyHandler)

	// Sans header X-User-ID (doit échouer côté Handler)
	requestNoHeader := httptest.NewRequest(http.MethodGet, "/test", nil)
	responseNoHeader := httptest.NewRecorder()
	handlerUnderTest.ServeHTTP(responseNoHeader, requestNoHeader)

	if responseNoHeader.Code != http.StatusUnauthorized {
		t.Fatalf("attendu 401 Unauthorized sans header, reçu %d", responseNoHeader.Code)
	}

	// X-User-ID valide
	requestWithHeader := httptest.NewRequest(http.MethodGet, "/test", nil)
	requestWithHeader.Header.Set("X-User-ID", "42")
	responseWithHeader := httptest.NewRecorder()
	handlerUnderTest.ServeHTTP(responseWithHeader, requestWithHeader)

	if responseWithHeader.Code != http.StatusOK || !strings.Contains(responseWithHeader.Body.String(), `"id":42`) {
		t.Fatalf("attendu 200 OK avec l'id 42, reçu %d : %s", responseWithHeader.Code, responseWithHeader.Body.String())
	}
}