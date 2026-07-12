package main

// TestResource is a small example resource used to demonstrate the API layers.
type TestResource struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

// PatchTestInput contains the fields accepted by PATCH /api/test.
type PatchTestInput struct {
	Message *string `json:"message"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
