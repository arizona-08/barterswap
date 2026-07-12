package main

import (
	"errors"
	"strings"
)

var ErrEmptyMessage = errors.New("message must not be empty")

// TestService contains the business logic independently from HTTP.
type TestService struct{}

func NewTestService() *TestService {
	return &TestService{}
}

func (s *TestService) Get() TestResource {
	return TestResource{
		Message: "BarterSwap API is ready",
		Status:  "ok",
	}
}

func (s *TestService) Patch(input PatchTestInput) (TestResource, error) {
	resource := s.Get()
	if input.Message == nil {
		return resource, nil
	}

	message := strings.TrimSpace(*input.Message)
	if message == "" {
		return TestResource{}, ErrEmptyMessage
	}

	resource.Message = message
	resource.Status = "updated"
	return resource, nil
}
