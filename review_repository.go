package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type SQLReviewStore struct {
	db *sql.DB
}

func NewSQLReviewStore(db *sql.DB) *SQLReviewStore {
	return &SQLReviewStore{db: db}
}

func (s *SQLReviewStore) CreateReview(ctx context.Context, exchangeID, authorID, targetID, note int, commentaire string) (Review, error) {
	const query = `INSERT INTO reviews (exchange_id, author_id, target_id, note, commentaire)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, exchange_id, author_id, target_id, note, commentaire, created_at`
	return scanReview(s.db.QueryRowContext(ctx, query, exchangeID, authorID, targetID, note, commentaire))
}

func (s *SQLReviewStore) HasReview(ctx context.Context, exchangeID, authorID int) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM reviews WHERE exchange_id = $1 AND author_id = $2
	)`, exchangeID, authorID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check review: %w", err)
	}
	return exists, nil
}

func (s *SQLReviewStore) ListUserReviews(ctx context.Context, userID int) ([]Review, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, exchange_id, author_id, target_id, note, commentaire, created_at
		FROM reviews WHERE target_id = $1 ORDER BY created_at DESC, id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user reviews: %w", err)
	}
	return scanReviews(rows)
}

func (s *SQLReviewStore) ListServiceReviews(ctx context.Context, serviceID int) ([]Review, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT r.id, r.exchange_id, r.author_id, r.target_id, r.note, r.commentaire, r.created_at
		FROM reviews r
		JOIN exchanges e ON e.id = r.exchange_id
		JOIN services s ON s.id = e.service_id
		WHERE e.service_id = $1 AND r.target_id = s.provider_id
		ORDER BY r.created_at DESC, r.id DESC`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list service reviews: %w", err)
	}
	return scanReviews(rows)
}

type reviewRowScanner interface {
	Scan(...any) error
}

func scanReview(row reviewRowScanner) (Review, error) {
	var review Review
	var createdAt time.Time
	if err := row.Scan(&review.ID, &review.ExchangeID, &review.AuthorID, &review.TargetID, &review.Note, &review.Commentaire, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Review{}, ErrReviewNotFound
		}
		return Review{}, fmt.Errorf("scan review: %w", err)
	}
	review.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return review, nil
}

func scanReviews(rows *sql.Rows) ([]Review, error) {
	defer rows.Close()
	reviews := make([]Review, 0)
	for rows.Next() {
		review, err := scanReview(rows)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, review)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reviews: %w", err)
	}
	return reviews, nil
}
