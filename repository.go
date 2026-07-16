package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type SQLUserStore struct {
	db *sql.DB
}

func NewSQLUserStore(db *sql.DB) *SQLUserStore {
	return &SQLUserStore{db: db}
}

func migrate(ctx context.Context, db *sql.DB) error {
	const schema = `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			pseudo TEXT NOT NULL CHECK (btrim(pseudo) <> ''),
			bio TEXT NOT NULL DEFAULT '',
			ville TEXT NOT NULL DEFAULT '',
			credit_balance INTEGER NOT NULL DEFAULT 10 CHECK (credit_balance >= 0),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
			CREATE TABLE IF NOT EXISTS user_skills (
				user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				nom TEXT NOT NULL,
				niveau TEXT NOT NULL CHECK (niveau IN ('débutant', 'intermédiaire', 'expert')),
				PRIMARY KEY (user_id, nom)
			);
			CREATE TABLE IF NOT EXISTS services (
				id SERIAL PRIMARY KEY,
				provider_id INTEGER NOT NULL REFERENCES users(id),
				titre TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				categorie TEXT NOT NULL,
				duree_minutes INTEGER NOT NULL CHECK (duree_minutes > 0),
				credits INTEGER NOT NULL CHECK (credits > 0),
				ville TEXT NOT NULL DEFAULT '',
				actif BOOLEAN NOT NULL DEFAULT TRUE,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			);
			CREATE TABLE IF NOT EXISTS exchanges (
				id SERIAL PRIMARY KEY,
				service_id INTEGER NOT NULL REFERENCES services(id),
				requester_id INTEGER NOT NULL REFERENCES users(id),
				owner_id INTEGER NOT NULL REFERENCES users(id),
				status TEXT NOT NULL CHECK (status IN ('pending', 'accepted', 'rejected', 'cancelled', 'completed')),
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			);
			CREATE TABLE IF NOT EXISTS credit_transactions (
				id SERIAL PRIMARY KEY,
				user_id INTEGER NOT NULL REFERENCES users(id),
				exchange_id INTEGER REFERENCES exchanges(id),
				montant INTEGER NOT NULL,
				type TEXT NOT NULL CHECK (type IN ('earn', 'spend', 'refund')),
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			);
			CREATE UNIQUE INDEX IF NOT EXISTS one_active_exchange_per_service
				ON exchanges(service_id) WHERE status IN ('pending', 'accepted');
			CREATE TABLE IF NOT EXISTS reviews (
				id SERIAL PRIMARY KEY,
				exchange_id INTEGER NOT NULL REFERENCES exchanges(id) ON DELETE CASCADE,
				author_id INTEGER NOT NULL REFERENCES users(id),
				target_id INTEGER NOT NULL REFERENCES users(id),
				note INTEGER NOT NULL CHECK (note BETWEEN 1 AND 5),
				commentaire TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE (exchange_id, author_id)
			);`
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	return nil
}

func (s *SQLUserStore) CreateUser(ctx context.Context, input CreateUserInput, credits int) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("begin create user: %w", err)
	}
	defer tx.Rollback()

	const query = `INSERT INTO users (pseudo, bio, ville, credit_balance)
		VALUES ($1, $2, $3, $4)
		RETURNING id, pseudo, bio, ville, credit_balance, created_at`
	user, err := scanUser(tx.QueryRowContext(ctx, query, input.Pseudo, input.Bio, input.Ville, credits))
	if err != nil {
		return User{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO credit_transactions (user_id, exchange_id, montant, type)
		VALUES ($1, NULL, $2, 'earn')`, user.ID, credits); err != nil {
		return User{}, fmt.Errorf("record welcome credits: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("commit create user: %w", err)
	}
	return user, nil
}

func (s *SQLUserStore) GetUser(ctx context.Context, id int) (User, error) {
	const query = `SELECT id, pseudo, bio, ville, credit_balance, created_at FROM users WHERE id = $1`
	return scanUser(s.db.QueryRowContext(ctx, query, id))
}

func (s *SQLUserStore) UpdateUser(ctx context.Context, id int, input UpdateUserInput) (User, error) {
	const query = `UPDATE users SET pseudo = $1, bio = $2, ville = $3 WHERE id = $4
		RETURNING id, pseudo, bio, ville, credit_balance, created_at`
	return scanUser(s.db.QueryRowContext(ctx, query, input.Pseudo, input.Bio, input.Ville, id))
}

type rowScanner interface {
	Scan(...any) error
}

func scanUser(row rowScanner) (User, error) {
	var user User
	var createdAt time.Time
	if err := row.Scan(&user.ID, &user.Pseudo, &user.Bio, &user.Ville, &user.CreditBalance, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("scan user: %w", err)
	}
	user.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return user, nil
}

func (s *SQLUserStore) GetSkills(ctx context.Context, userID int) ([]Skill, error) {
	if _, err := s.GetUser(ctx, userID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT nom, niveau FROM user_skills WHERE user_id = $1 ORDER BY nom`, userID)
	if err != nil {
		return nil, fmt.Errorf("query skills: %w", err)
	}
	defer rows.Close()

	skills := make([]Skill, 0)
	for rows.Next() {
		var skill Skill
		if err := rows.Scan(&skill.Nom, &skill.Niveau); err != nil {
			return nil, fmt.Errorf("scan skill: %w", err)
		}
		skills = append(skills, skill)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate skills: %w", err)
	}
	return skills, nil
}

func (s *SQLUserStore) ReplaceSkills(ctx context.Context, userID int, skills []Skill) ([]Skill, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin replacing skills: %w", err)
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check user: %w", err)
	}
	if !exists {
		return nil, ErrUserNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_skills WHERE user_id = $1`, userID); err != nil {
		return nil, fmt.Errorf("delete skills: %w", err)
	}
	for _, skill := range skills {
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_skills (user_id, nom, niveau) VALUES ($1, $2, $3)`, userID, skill.Nom, skill.Niveau); err != nil {
			return nil, fmt.Errorf("insert skill: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit skills: %w", err)
	}
	return skills, nil
}
