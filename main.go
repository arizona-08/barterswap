package main

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"
	"syscall"
	_ "github.com/lib/pq"

	
)

func main() {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(jsonHandler))

	port := envOrDefault("PORT", "8080")
	databaseURL := envOrDefault("DATABASE_URL", "postgres://barterswap:barterswap@localhost:5432/barterswap?sslmode=disable")

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	if err := migrate(ctx, db); err != nil {
		log.Fatalf("migrate database: %v", err)
	}

	userStore := NewSQLUserStore(db)
	serviceStore := NewSQLServiceStore(db)
	exchangeStore := NewSQLExchangeStore(db)
	userHandler := NewUserHandler(NewUserService(userStore))
	serviceHandler := NewServiceHandler(NewServiceService(serviceStore))
	exchangeHandler := NewExchangeHandler(NewExchangeService(exchangeStore))
	reviewHandler := NewReviewHandler(NewReviewService(NewSQLReviewStore(db), exchangeStore, userStore, serviceStore))
	statsHandler := NewStatsHandler(NewStatsService(NewSQLStatsStore(db)))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/users", userHandler.Create)
	mux.HandleFunc("GET /api/users/{id}", userHandler.Get)
	mux.HandleFunc("PUT /api/users/{id}", userHandler.Update)
	mux.HandleFunc("GET /api/users/{id}/skills", userHandler.GetSkills)
	mux.HandleFunc("PUT /api/users/{id}/skills", userHandler.ReplaceSkills)
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
	mux.HandleFunc("POST /api/exchanges/{id}/review", reviewHandler.Create)
	mux.HandleFunc("GET /api/users/{id}/reviews", reviewHandler.ListUser)
	mux.HandleFunc("GET /api/services/{id}/reviews", reviewHandler.ListService)
	mux.HandleFunc("GET /api/users/{id}/stats", statsHandler.Get)

	handler := recoveryMiddleware(corsMiddleware(authMiddleware(loggingMiddleware(timeoutMiddleware(mux)))))

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	
	// intercepter signaux système
	stop := make(chan os.Signal, 1)
	// SIGINT et SIGTERM
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("BarterSwap API listening", slog.String("port", port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("critical HTTP server error: %v", err)
		}
	}()
	<-stop

	// vider connexions actives
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()

	// Demande d'arrêt propre au serveur HTTP
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	} else {
		log.Println("HTTP server stopped gracefully after completing active requests.")
	}

	log.Println("Closing database and exiting.")
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
