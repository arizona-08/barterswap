package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	testService := NewTestService()
	testHandler := NewTestHandler(testService)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/test", testHandler.Get)
	mux.HandleFunc("PATCH /api/test", testHandler.Patch)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("BarterSwap API listening on http://localhost:%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
