package main

import (
	"context"
	"errors"
)

const (
	ExchangePending   = "pending"
	ExchangeAccepted  = "accepted"
	ExchangeRejected  = "rejected"
	ExchangeCancelled = "cancelled"
	ExchangeCompleted = "completed"
)

var (
	ErrExchangeNotFound      = errors.New("exchange not found")
	ErrSelfExchange          = errors.New("you cannot exchange with yourself")
	ErrServiceReserved       = errors.New("service already has an active exchange")
	ErrInsufficientCredits   = errors.New("not enough credits")
	ErrInvalidExchangeStatus = errors.New("invalid exchange status")
	ErrInvalidExchangeState  = errors.New("exchange cannot change to this status")
)

type ExchangeStore interface {
	CreateExchange(context.Context, int, int) (Exchange, error)
	ListExchanges(context.Context, int, string) ([]Exchange, error)
	GetExchange(context.Context, int) (Exchange, error)
	AcceptExchange(context.Context, int, int) (Exchange, error)
	RejectExchange(context.Context, int, int) (Exchange, error)
	CompleteExchange(context.Context, int, int) (Exchange, error)
	CancelExchange(context.Context, int, int) (Exchange, error)
}

type ExchangeService struct {
	store ExchangeStore
}

func NewExchangeService(store ExchangeStore) *ExchangeService {
	return &ExchangeService{store: store}
}

func (s *ExchangeService) Create(ctx context.Context, requesterID, serviceID int) (Exchange, error) {
	return s.store.CreateExchange(ctx, requesterID, serviceID)
}

func (s *ExchangeService) List(ctx context.Context, userID int, status string) ([]Exchange, error) {
	if status != "" && !validExchangeStatus(status) {
		return nil, ErrInvalidExchangeStatus
	}
	return s.store.ListExchanges(ctx, userID, status)
}

func (s *ExchangeService) Get(ctx context.Context, userID, id int) (Exchange, error) {
	exchange, err := s.store.GetExchange(ctx, id)
	if err != nil {
		return Exchange{}, err
	}
	if exchange.RequesterID != userID && exchange.OwnerID != userID {
		return Exchange{}, ErrForbidden
	}
	return exchange, nil
}

func (s *ExchangeService) Accept(ctx context.Context, userID, id int) (Exchange, error) {
	return s.store.AcceptExchange(ctx, id, userID)
}

func (s *ExchangeService) Reject(ctx context.Context, userID, id int) (Exchange, error) {
	return s.store.RejectExchange(ctx, id, userID)
}

func (s *ExchangeService) Complete(ctx context.Context, userID, id int) (Exchange, error) {
	return s.store.CompleteExchange(ctx, id, userID)
}

func (s *ExchangeService) Cancel(ctx context.Context, userID, id int) (Exchange, error) {
	return s.store.CancelExchange(ctx, id, userID)
}

func validExchangeStatus(status string) bool {
	switch status {
	case ExchangePending, ExchangeAccepted, ExchangeRejected, ExchangeCancelled, ExchangeCompleted:
		return true
	default:
		return false
	}
}
