package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func integrationDatabase(t *testing.T) *sql.DB {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open integration database: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("integration database is unavailable: %v", err)
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		t.Fatalf("migrate integration database: %v", err)
	}
	return db
}

func TestPostgresServicesAndExchanges(t *testing.T) {
	db := integrationDatabase(t)
	ctx := context.Background()
	users := NewSQLUserStore(db)
	services := NewSQLServiceStore(db)
	exchanges := NewSQLExchangeStore(db)
	reviews := NewSQLReviewStore(db)
	userService := NewUserService(users)
	serviceService := NewServiceService(services)
	exchangeService := NewExchangeService(exchanges)
	reviewService := NewReviewService(reviews, exchanges, users, services)
	statsService := NewStatsService(NewSQLStatsStore(db))

	suffix := time.Now().UnixNano()
	provider, err := userService.Create(ctx, CreateUserInput{Pseudo: fmt.Sprintf("provider-%d", suffix)})
	if err != nil {
		t.Fatal(err)
	}
	requester, err := userService.Create(ctx, CreateUserInput{Pseudo: fmt.Sprintf("requester-%d", suffix)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM reviews WHERE author_id IN ($1, $2) OR target_id IN ($1, $2)`, provider.ID, requester.ID)
		_, _ = db.Exec(`DELETE FROM credit_transactions WHERE user_id IN ($1, $2)`, provider.ID, requester.ID)
		_, _ = db.Exec(`DELETE FROM exchanges WHERE requester_id IN ($1, $2) OR owner_id IN ($1, $2)`, provider.ID, requester.ID)
		_, _ = db.Exec(`DELETE FROM services WHERE provider_id IN ($1, $2)`, provider.ID, requester.ID)
		_, _ = db.Exec(`DELETE FROM user_skills WHERE user_id IN ($1, $2)`, provider.ID, requester.ID)
		_, _ = db.Exec(`DELETE FROM users WHERE id IN ($1, $2)`, provider.ID, requester.ID)
		_ = db.Close()
	})

	if _, err := users.ReplaceSkills(ctx, provider.ID, []Skill{{Nom: "Jardinage", Niveau: "expert"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := userService.Get(ctx, provider.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := userService.Update(ctx, requester.ID, requester.ID, UpdateUserInput{Pseudo: requester.Pseudo + "-updated"}); err != nil {
		t.Fatal(err)
	}
	if _, err := userService.Skills(ctx, provider.ID); err != nil {
		t.Fatal(err)
	}

	service, err := serviceService.Create(ctx, provider.ID, CreateServiceInput{
		Titre:        "Aide au jardin",
		Categorie:    "Jardinage",
		DureeMinutes: 60,
		Credits:      2,
		Ville:        "Paris",
	})
	if err != nil {
		t.Fatal(err)
	}
	if service.Credits != 2 || !service.Actif {
		t.Fatalf("unexpected service: %#v", service)
	}
	if _, err := serviceService.List(ctx, ServiceFilters{Categorie: "Jardinage", Ville: "Paris", Search: "jardin"}); err != nil {
		t.Fatal(err)
	}
	if _, err := serviceService.Update(ctx, provider.ID, service.ID, UpdateServiceInput{
		Titre:        "Aide au grand jardin",
		Categorie:    "Jardinage",
		DureeMinutes: 90,
		Credits:      2,
		Ville:        "Paris",
		Actif:        true,
	}); err != nil {
		t.Fatal(err)
	}

	exchange, err := exchangeService.Create(ctx, requester.ID, service.ID)
	if err != nil {
		t.Fatal(err)
	}
	if exchange.Status != ExchangePending {
		t.Fatalf("expected pending exchange, got %s", exchange.Status)
	}
	if _, err := exchangeService.Create(ctx, requester.ID, service.ID); !errors.Is(err, ErrServiceReserved) {
		t.Fatalf("expected reserved service error, got %v", err)
	}
	if _, err := exchangeService.List(ctx, requester.ID, ExchangePending, 20, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := exchangeService.Get(ctx, requester.ID, exchange.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := exchangeService.Accept(ctx, provider.ID, exchange.ID); err != nil {
		t.Fatal(err)
	}
	if balance := userBalance(t, db, requester.ID); balance != 8 {
		t.Fatalf("expected requester balance 8 after accept, got %d", balance)
	}
	if _, err := exchangeService.Complete(ctx, requester.ID, exchange.ID); err != nil {
		t.Fatal(err)
	}
	if balance := userBalance(t, db, provider.ID); balance != 12 {
		t.Fatalf("expected provider balance 12 after completion, got %d", balance)
	}
	if _, err := reviewService.Create(ctx, requester.ID, exchange.ID, CreateReviewInput{Note: 5, Commentaire: "Très bon service"}); err != nil {
		t.Fatal(err)
	}
	if _, err := reviewService.Create(ctx, provider.ID, exchange.ID, CreateReviewInput{Note: 4}); err != nil {
		t.Fatal(err)
	}
	if _, err := reviewService.ListUserReviews(ctx, provider.ID, 20, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := reviewService.ListServiceReviews(ctx, service.ID, 20, 0); err != nil {
		t.Fatal(err)
	}
	providerStats, err := statsService.Get(ctx, provider.ID)
	if err != nil {
		t.Fatal(err)
	}
	if providerStats.EchangesCompletes != 1 || providerStats.NbAvis != 1 || providerStats.NoteMoyenne != 5 || providerStats.TotalGagne != 12 {
		t.Fatalf("unexpected provider stats: %#v", providerStats)
	}

	second, err := serviceService.Create(ctx, provider.ID, CreateServiceInput{
		Titre:        "Petit jardin",
		Categorie:    "Jardinage",
		DureeMinutes: 30,
		Credits:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := exchangeService.Create(ctx, requester.ID, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exchangeService.Reject(ctx, provider.ID, rejected.ID); err != nil {
		t.Fatal(err)
	}
	cancelled, err := exchangeService.Create(ctx, requester.ID, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exchangeService.Accept(ctx, provider.ID, cancelled.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := exchangeService.Cancel(ctx, requester.ID, cancelled.ID); err != nil {
		t.Fatal(err)
	}
	if balance := userBalance(t, db, requester.ID); balance != 8 {
		t.Fatalf("expected refund to restore requester balance to 8, got %d", balance)
	}

	if _, err := exchangeService.Create(ctx, provider.ID, second.ID); !errors.Is(err, ErrSelfExchange) {
		t.Fatalf("expected self-exchange error, got %v", err)
	}
	if err := serviceService.Delete(ctx, provider.ID, second.ID); err == nil {
		t.Fatal("expected service with exchange history to be protected by foreign key")
	}
}

func userBalance(t *testing.T, db *sql.DB, id int) int {
	t.Helper()
	var balance int
	if err := db.QueryRow(`SELECT credit_balance FROM users WHERE id = $1`, id).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	return balance
}
