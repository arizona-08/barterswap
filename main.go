package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
	"syscall"
	_ "github.com/lib/pq"

	
)

func main() {
	port := envOrDefault("PORT", "8080")
	databaseURL := envOrDefault("DATABASE_URL", "postgres://barterswap:barterswap@localhost:5432/barterswap?sslmode=disable")

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	if err := migrate(ctx, db); err != nil {
		log.Fatal(err)
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

	handler := recoveryMiddleware(loggingMiddleware(corsMiddleware(authMiddleware(mux))))

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
		log.Printf("BarterSwap API listening on http://localhost:%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Erreur critique du serveur HTTP : %v", err)
		}
	}()
	<-stop

	// vider connexions actives
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()

	// Demande d'arrêt propre au serveur HTTP
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Erreur lors de la fermeture du serveur : %v", err)
	} else {
		log.Println("Le serveur HTTP a traité toutes les requêtes en cours et s'est arrêté proprement.")
	}

	log.Println("Fermeture de la base de données et fin définitive du programme.")
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
