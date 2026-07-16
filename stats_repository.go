package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type SQLStatsStore struct {
	db *sql.DB
}

func NewSQLStatsStore(db *sql.DB) *SQLStatsStore {
	return &SQLStatsStore{db: db}
}

func (s *SQLStatsStore) GetUserStats(ctx context.Context, userID int) (UserStats, error) {
	const query = `
		SELECT
			u.id,
			u.credit_balance,
			(SELECT COUNT(*) FROM services WHERE provider_id = u.id AND actif = TRUE),
			(SELECT COUNT(*) FROM exchanges WHERE (requester_id = u.id OR owner_id = u.id) AND status = 'completed'),
			COALESCE((SELECT AVG(note)::float8 FROM reviews WHERE target_id = u.id), 0),
			(SELECT COUNT(*) FROM reviews WHERE target_id = u.id),
			COALESCE((SELECT SUM(montant) FROM credit_transactions WHERE user_id = u.id AND type = 'earn'), 0),
			COALESCE((SELECT SUM(-montant) FROM credit_transactions WHERE user_id = u.id AND type = 'spend'), 0)
		FROM users u
		WHERE u.id = $1`

	var stats UserStats
	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&stats.UserID,
		&stats.CreditBalance,
		&stats.ServicesActifs,
		&stats.EchangesCompletes,
		&stats.NoteMoyenne,
		&stats.NbAvis,
		&stats.TotalGagne,
		&stats.TotalDepense,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserStats{}, ErrUserNotFound
		}
		return UserStats{}, fmt.Errorf("scan user stats: %w", err)
	}
	return stats, nil
}
