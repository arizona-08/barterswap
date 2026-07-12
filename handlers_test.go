package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetTestResource(t *testing.T) {
	handler := NewTestHandler(NewTestService())
	request := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	response := httptest.NewRecorder()

	handler.Get(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected application/json, got %q", contentType)
	}
}

func TestPatchTestResource(t *testing.T) {
	handler := NewTestHandler(NewTestService())
	request := httptest.NewRequest(http.MethodPatch, "/api/test", strings.NewReader(`{"message":"Hello team"}`))
	response := httptest.NewRecorder()

	handler.Patch(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"status":"updated"`) {
		t.Fatalf("unexpected response body: %s", response.Body.String())
	}
}
