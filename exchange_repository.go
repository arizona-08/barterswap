package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type SQLExchangeStore struct {
	db *sql.DB
}

func NewSQLExchangeStore(db *sql.DB) *SQLExchangeStore {
	return &SQLExchangeStore{db: db}
}

func (s *SQLExchangeStore) CreateExchange(ctx context.Context, requesterID, serviceID int) (Exchange, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Exchange{}, fmt.Errorf("begin exchange: %w", err)
	}
	defer tx.Rollback()

	var ownerID, credits int
	var active bool
	err = tx.QueryRowContext(ctx, `SELECT provider_id, credits, actif FROM services WHERE id = $1`, serviceID).Scan(&ownerID, &credits, &active)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Exchange{}, ErrServiceNotFound
		}
		return Exchange{}, fmt.Errorf("find service for exchange: %w", err)
	}
	if !active {
		return Exchange{}, ErrServiceInactive
	}
	if ownerID == requesterID {
		return Exchange{}, ErrSelfExchange
	}

	var reserved bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM exchanges WHERE service_id = $1 AND status IN ('pending', 'accepted')
	)`, serviceID).Scan(&reserved)
	if err != nil {
		return Exchange{}, fmt.Errorf("check service reservation: %w", err)
	}
	if reserved {
		return Exchange{}, ErrServiceReserved
	}

	var balance int
	err = tx.QueryRowContext(ctx, `SELECT credit_balance FROM users WHERE id = $1`, requesterID).Scan(&balance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Exchange{}, ErrUserNotFound
		}
		return Exchange{}, fmt.Errorf("find requester: %w", err)
	}
	if balance < credits {
		return Exchange{}, ErrInsufficientCredits
	}

	var exchangeID int
	err = tx.QueryRowContext(ctx, `INSERT INTO exchanges (service_id, requester_id, owner_id, status)
		VALUES ($1, $2, $3, $4) RETURNING id`, serviceID, requesterID, ownerID, ExchangePending).Scan(&exchangeID)
	if err != nil {
		return Exchange{}, fmt.Errorf("create exchange: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Exchange{}, fmt.Errorf("commit exchange: %w", err)
	}
	return s.GetExchange(ctx, exchangeID)
}

func (s *SQLExchangeStore) ListExchanges(ctx context.Context, userID int, status string, limit, offset int) ([]Exchange, error) {
	query := `SELECT id, service_id, requester_id, owner_id, status, created_at, updated_at
		FROM exchanges WHERE (requester_id = $1 OR owner_id = $1)`
	args := []any{userID}
	if status != "" {
		args = append(args, status)
		query += " AND status = $2"
	}
	query += " ORDER BY updated_at DESC, id DESC"

	args = append(args, limit, offset)
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list exchanges: %w", err)
	}
	defer rows.Close()

	exchanges := make([]Exchange, 0)
	for rows.Next() {
		exchange, err := scanExchange(rows)
		if err != nil {
			return nil, err
		}
		exchanges = append(exchanges, exchange)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate exchanges: %w", err)
	}
	return exchanges, nil
}

func (s *SQLExchangeStore) GetExchange(ctx context.Context, id int) (Exchange, error) {
	const query = `SELECT id, service_id, requester_id, owner_id, status, created_at, updated_at
		FROM exchanges WHERE id = $1`
	return scanExchange(s.db.QueryRowContext(ctx, query, id))
}

func (s *SQLExchangeStore) AcceptExchange(ctx context.Context, id, userID int) (Exchange, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Exchange{}, fmt.Errorf("begin accept exchange: %w", err)
	}
	defer tx.Rollback()

	exchange, credits, err := scanLockedExchange(tx, id)
	if err != nil {
		return Exchange{}, err
	}
	if exchange.OwnerID != userID {
		return Exchange{}, ErrForbidden
	}
	if exchange.Status != ExchangePending {
		return Exchange{}, ErrInvalidExchangeState
	}
	result, err := tx.ExecContext(ctx, `UPDATE users SET credit_balance = credit_balance - $1
		WHERE id = $2 AND credit_balance >= $1`, credits, exchange.RequesterID)
	if err != nil {
		return Exchange{}, fmt.Errorf("block credits: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Exchange{}, fmt.Errorf("block credits result: %w", err)
	}
	if count == 0 {
		return Exchange{}, ErrInsufficientCredits
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO credit_transactions (user_id, exchange_id, montant, type)
		VALUES ($1, $2, $3, 'spend')`, exchange.RequesterID, id, -credits); err != nil {
		return Exchange{}, fmt.Errorf("record spend: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE exchanges SET status = $1, updated_at = NOW() WHERE id = $2`, ExchangeAccepted, id); err != nil {
		return Exchange{}, fmt.Errorf("accept exchange: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Exchange{}, fmt.Errorf("commit accepted exchange: %w", err)
	}
	return s.GetExchange(ctx, id)
}

func (s *SQLExchangeStore) RejectExchange(ctx context.Context, id, userID int) (Exchange, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Exchange{}, fmt.Errorf("begin reject exchange: %w", err)
	}
	defer tx.Rollback()
	exchange, _, err := scanLockedExchange(tx, id)
	if err != nil {
		return Exchange{}, err
	}
	if exchange.OwnerID != userID {
		return Exchange{}, ErrForbidden
	}
	if exchange.Status != ExchangePending {
		return Exchange{}, ErrInvalidExchangeState
	}
	if _, err := tx.ExecContext(ctx, `UPDATE exchanges SET status = $1, updated_at = NOW() WHERE id = $2`, ExchangeRejected, id); err != nil {
		return Exchange{}, fmt.Errorf("reject exchange: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Exchange{}, fmt.Errorf("commit rejected exchange: %w", err)
	}
	return s.GetExchange(ctx, id)
}

func (s *SQLExchangeStore) CompleteExchange(ctx context.Context, id, userID int) (Exchange, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Exchange{}, fmt.Errorf("begin complete exchange: %w", err)
	}
	defer tx.Rollback()
	exchange, credits, err := scanLockedExchange(tx, id)
	if err != nil {
		return Exchange{}, err
	}
	if exchange.OwnerID != userID && exchange.RequesterID != userID {
		return Exchange{}, ErrForbidden
	}
	if exchange.Status != ExchangeAccepted {
		return Exchange{}, ErrInvalidExchangeState
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET credit_balance = credit_balance + $1 WHERE id = $2`, credits, exchange.OwnerID); err != nil {
		return Exchange{}, fmt.Errorf("transfer credits: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO credit_transactions (user_id, exchange_id, montant, type)
		VALUES ($1, $2, $3, 'earn')`, exchange.OwnerID, id, credits); err != nil {
		return Exchange{}, fmt.Errorf("record earn: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE exchanges SET status = $1, updated_at = NOW() WHERE id = $2`, ExchangeCompleted, id); err != nil {
		return Exchange{}, fmt.Errorf("complete exchange: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Exchange{}, fmt.Errorf("commit completed exchange: %w", err)
	}
	return s.GetExchange(ctx, id)
}

func (s *SQLExchangeStore) CancelExchange(ctx context.Context, id, userID int) (Exchange, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Exchange{}, fmt.Errorf("begin cancel exchange: %w", err)
	}
	defer tx.Rollback()
	exchange, credits, err := scanLockedExchange(tx, id)
	if err != nil {
		return Exchange{}, err
	}
	if exchange.OwnerID != userID && exchange.RequesterID != userID {
		return Exchange{}, ErrForbidden
	}
	if exchange.Status != ExchangePending && exchange.Status != ExchangeAccepted {
		return Exchange{}, ErrInvalidExchangeState
	}
	if exchange.Status == ExchangeAccepted {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET credit_balance = credit_balance + $1 WHERE id = $2`, credits, exchange.RequesterID); err != nil {
			return Exchange{}, fmt.Errorf("refund credits: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO credit_transactions (user_id, exchange_id, montant, type)
			VALUES ($1, $2, $3, 'refund')`, exchange.RequesterID, id, credits); err != nil {
			return Exchange{}, fmt.Errorf("record refund: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE exchanges SET status = $1, updated_at = NOW() WHERE id = $2`, ExchangeCancelled, id); err != nil {
		return Exchange{}, fmt.Errorf("cancel exchange: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Exchange{}, fmt.Errorf("commit cancelled exchange: %w", err)
	}
	return s.GetExchange(ctx, id)
}

type lockedExchange struct {
	Exchange
	Credits int
}

func scanLockedExchange(tx *sql.Tx, id int) (Exchange, int, error) {
	var exchange Exchange
	var credits int
	var createdAt, updatedAt time.Time
	err := tx.QueryRow(`SELECT e.id, e.service_id, e.requester_id, e.owner_id, e.status,
		e.created_at, e.updated_at, s.credits
		FROM exchanges e JOIN services s ON s.id = e.service_id
		WHERE e.id = $1 FOR UPDATE`, id).Scan(&exchange.ID, &exchange.ServiceID, &exchange.RequesterID, &exchange.OwnerID, &exchange.Status, &createdAt, &updatedAt, &credits)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Exchange{}, 0, ErrExchangeNotFound
		}
		return Exchange{}, 0, fmt.Errorf("lock exchange: %w", err)
	}
	exchange.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	exchange.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return exchange, credits, nil
}

func scanExchange(row serviceRowScanner) (Exchange, error) {
	var exchange Exchange
	var createdAt, updatedAt time.Time
	if err := row.Scan(&exchange.ID, &exchange.ServiceID, &exchange.RequesterID, &exchange.OwnerID, &exchange.Status, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Exchange{}, ErrExchangeNotFound
		}
		return Exchange{}, fmt.Errorf("scan exchange: %w", err)
	}
	exchange.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	exchange.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return exchange, nil
}
