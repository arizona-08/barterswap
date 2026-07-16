package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type SQLServiceStore struct {
	db *sql.DB
}

func NewSQLServiceStore(db *sql.DB) *SQLServiceStore {
	return &SQLServiceStore{db: db}
}

func (s *SQLServiceStore) CreateService(ctx context.Context, providerID int, input CreateServiceInput) (Service, error) {
	const query = `INSERT INTO services
		(provider_id, titre, description, categorie, duree_minutes, credits, ville)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, provider_id, titre, description, categorie, duree_minutes, credits, ville, actif, created_at`
	return scanService(s.db.QueryRowContext(ctx, query, providerID, input.Titre, input.Description, input.Categorie, input.DureeMinutes, input.Credits, input.Ville))
}

func (s *SQLServiceStore) GetService(ctx context.Context, id int) (Service, error) {
	const query = `SELECT id, provider_id, titre, description, categorie, duree_minutes, credits, ville, actif, created_at
		FROM services WHERE id = $1`
	return scanService(s.db.QueryRowContext(ctx, query, id))
}

func (s *SQLServiceStore) ListServices(ctx context.Context, filters ServiceFilters) ([]Service, error) {
	query := `SELECT id, provider_id, titre, description, categorie, duree_minutes, credits, ville, actif, created_at
		FROM services WHERE actif = TRUE`
	args := make([]any, 0, 5)
	if filters.Categorie != "" {
		args = append(args, filters.Categorie)
		query += fmt.Sprintf(" AND categorie = $%d", len(args))
	}
	if filters.Ville != "" {
		args = append(args, filters.Ville)
		query += fmt.Sprintf(" AND ville = $%d", len(args))
	}
	if filters.Search != "" {
		args = append(args, "%"+filters.Search+"%")
		query += fmt.Sprintf(" AND (titre ILIKE $%d OR description ILIKE $%d)", len(args), len(args))
	}
	query += " ORDER BY created_at DESC, id DESC"

	args = append(args, filters.Limit, filters.Offset)
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	defer rows.Close()

	services := make([]Service, 0)
	for rows.Next() {
		service, err := scanService(rows)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate services: %w", err)
	}
	return services, nil
}

func (s *SQLServiceStore) UpdateService(ctx context.Context, id int, input UpdateServiceInput) (Service, error) {
	const query = `UPDATE services
		SET titre = $1, description = $2, categorie = $3, duree_minutes = $4,
			credits = $5, ville = $6, actif = $7
		WHERE id = $8
		RETURNING id, provider_id, titre, description, categorie, duree_minutes, credits, ville, actif, created_at`
	return scanService(s.db.QueryRowContext(ctx, query, input.Titre, input.Description, input.Categorie, input.DureeMinutes, input.Credits, input.Ville, input.Actif, id))
}

func (s *SQLServiceStore) DeleteService(ctx context.Context, id int) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM services WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete service result: %w", err)
	}
	if count == 0 {
		return ErrServiceNotFound
	}
	return nil
}

func (s *SQLServiceStore) HasSkill(ctx context.Context, userID int, skillName string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM user_skills WHERE user_id = $1 AND nom = $2)`, userID, skillName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check user skill: %w", err)
	}
	return exists, nil
}

type serviceRowScanner interface {
	Scan(...any) error
}

func scanService(row serviceRowScanner) (Service, error) {
	var service Service
	var createdAt time.Time
	if err := row.Scan(&service.ID, &service.ProviderID, &service.Titre, &service.Description, &service.Categorie, &service.DureeMinutes, &service.Credits, &service.Ville, &service.Actif, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Service{}, ErrServiceNotFound
		}
		return Service{}, fmt.Errorf("scan service: %w", err)
	}
	service.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return service, nil
}
