package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeUserStore struct {
	user   User
	skills []Skill
}

func (s *fakeUserStore) CreateUser(_ context.Context, input CreateUserInput, credits int) (User, error) {
	s.user = User{ID: 1, Pseudo: input.Pseudo, Bio: input.Bio, Ville: input.Ville, CreditBalance: credits, CreatedAt: "2026-01-01T00:00:00Z"}
	return s.user, nil
}

func (s *fakeUserStore) GetUser(_ context.Context, id int) (User, error) {
	if s.user.ID != id {
		return User{}, ErrUserNotFound
	}
	return s.user, nil
}

func (s *fakeUserStore) UpdateUser(_ context.Context, id int, input UpdateUserInput) (User, error) {
	if s.user.ID != id {
		return User{}, ErrUserNotFound
	}
	s.user.Pseudo, s.user.Bio, s.user.Ville = input.Pseudo, input.Bio, input.Ville
	return s.user, nil
}

func (s *fakeUserStore) GetSkills(_ context.Context, id int) ([]Skill, error) {
	if s.user.ID != id {
		return nil, ErrUserNotFound
	}
	return s.skills, nil
}

func (s *fakeUserStore) ReplaceSkills(_ context.Context, id int, skills []Skill) ([]Skill, error) {
	if s.user.ID != id {
		return nil, ErrUserNotFound
	}
	s.skills = skills
	return skills, nil
}

func newTestMux(store UserStore) *http.ServeMux {
	handler := NewUserHandler(NewUserService(store))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/users", handler.Create)
	mux.HandleFunc("GET /api/users/{id}", handler.Get)
	mux.HandleFunc("PUT /api/users/{id}", handler.Update)
	mux.HandleFunc("GET /api/users/{id}/skills", handler.GetSkills)
	mux.HandleFunc("PUT /api/users/{id}/skills", handler.ReplaceSkills)
	return mux
}

func TestCreateUser(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{name: "valid user", body: `{"pseudo":"Alice","ville":"Paris"}`, wantStatus: http.StatusCreated, wantBody: `"credit_balance":10`},
		{name: "empty pseudo", body: `{"pseudo":"  "}`, wantStatus: http.StatusBadRequest, wantBody: `"error":"pseudo must not be empty"`},
		{name: "unknown field", body: `{"pseudo":"Alice","email":"a@example.com"}`, wantStatus: http.StatusBadRequest, wantBody: `"error":"invalid JSON body"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(test.body))
			response := httptest.NewRecorder()
			newTestMux(&fakeUserStore{}).ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", test.wantStatus, response.Code, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), test.wantBody) {
				t.Fatalf("expected body to contain %q, got %s", test.wantBody, response.Body.String())
			}
		})
	}
}

func TestUpdateUserRequiresOwner(t *testing.T) {
	store := &fakeUserStore{user: User{ID: 1, Pseudo: "Alice"}}
	request := httptest.NewRequest(http.MethodPut, "/api/users/1", strings.NewReader(`{"pseudo":"Bob"}`))
	request.Header.Set("X-User-ID", "2")
	response := httptest.NewRecorder()

	newTestMux(store).ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", response.Code, response.Body.String())
	}
}

func TestReplaceSkills(t *testing.T) {
	store := &fakeUserStore{user: User{ID: 1, Pseudo: "Alice"}}
	request := httptest.NewRequest(http.MethodPut, "/api/users/1/skills", strings.NewReader(`{"skills":[{"nom":"Jardinage","niveau":"expert"}]}`))
	request.Header.Set("X-User-ID", "1")
	response := httptest.NewRecorder()

	newTestMux(store).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	if len(store.skills) != 1 || store.skills[0].Nom != "Jardinage" {
		t.Fatalf("skills were not replaced: %#v", store.skills)
	}
}

func TestGetUserAndSkills(t *testing.T) {
	store := &fakeUserStore{user: User{ID: 1, Pseudo: "Alice"}, skills: []Skill{{Nom: "Go", Niveau: "expert"}}}
	mux := newTestMux(store)

	request := httptest.NewRequest(http.MethodGet, "/api/users/1", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"pseudo":"Alice"`) {
		t.Fatalf("unexpected user response: %d %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/users/1/skills", nil)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"nom":"Go"`) {
		t.Fatalf("unexpected skills response: %d %s", response.Code, response.Body.String())
	}
}

func TestDuplicateSkillsAreRejected(t *testing.T) {
	service := NewUserService(&fakeUserStore{user: User{ID: 1}})
	_, err := service.ReplaceSkills(context.Background(), 1, 1, []Skill{
		{Nom: "Go", Niveau: "expert"},
		{Nom: "Go", Niveau: "débutant"},
	})
	if !errors.Is(err, ErrDuplicateSkill) {
		t.Fatalf("expected duplicate skill error, got %v", err)
	}
}

func TestInvalidSkillLevel(t *testing.T) {
	service := NewUserService(&fakeUserStore{user: User{ID: 1}})
	_, err := service.ReplaceSkills(context.Background(), 1, 1, []Skill{{Nom: "Go", Niveau: "senior"}})
	if !errors.Is(err, ErrInvalidLevel) {
		t.Fatalf("expected ErrInvalidLevel, got %v", err)
	}
}
