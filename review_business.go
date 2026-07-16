package main

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrInvalidNote          = errors.New("note must be between 1 and 5")
	ErrExchangeNotCompleted = errors.New("exchange must be completed before review")
	ErrReviewNotAllowed     = errors.New("only an exchange participant can review")
	ErrReviewAlreadyExists  = errors.New("you already reviewed this exchange")
	ErrReviewNotFound       = errors.New("review not found")
)

type ReviewStore interface {
	CreateReview(context.Context, int, int, int, int, string) (Review, error)
	HasReview(context.Context, int, int) (bool, error)
	ListUserReviews(context.Context, int, int, int) ([]Review, error)
	ListServiceReviews(context.Context, int, int, int) ([]Review, error)
}

type ReviewService struct {
	reviews   ReviewStore
	exchanges ExchangeStore
	users     UserStore
	services  ServiceStore
}

func NewReviewService(reviews ReviewStore, exchanges ExchangeStore, users UserStore, services ServiceStore) *ReviewService {
	return &ReviewService{reviews: reviews, exchanges: exchanges, users: users, services: services}
}

func (s *ReviewService) Create(ctx context.Context, authorID, exchangeID int, input CreateReviewInput) (Review, error) {
	if input.Note < 1 || input.Note > 5 {
		return Review{}, ErrInvalidNote
	}
	exchange, err := s.exchanges.GetExchange(ctx, exchangeID)
	if err != nil {
		return Review{}, err
	}
	if exchange.Status != ExchangeCompleted {
		return Review{}, ErrExchangeNotCompleted
	}
	var targetID int
	switch authorID {
	case exchange.RequesterID:
		targetID = exchange.OwnerID
	case exchange.OwnerID:
		targetID = exchange.RequesterID
	default:
		return Review{}, ErrReviewNotAllowed
	}

	exists, err := s.reviews.HasReview(ctx, exchangeID, authorID)
	if err != nil {
		return Review{}, err
	}
	if exists {
		return Review{}, ErrReviewAlreadyExists
	}
	input.Commentaire = strings.TrimSpace(input.Commentaire)
	return s.reviews.CreateReview(ctx, exchangeID, authorID, targetID, input.Note, input.Commentaire)
}

func (s *ReviewService) ListUserReviews(ctx context.Context, userID int, limit, offset int) ([]Review, error) {
	if _, err := s.users.GetUser(ctx, userID); err != nil {
		return nil, err
	}
	return s.reviews.ListUserReviews(ctx, userID, limit, offset)
}

func (s *ReviewService) ListServiceReviews(ctx context.Context, serviceID int, limit, offset int) ([]Review, error) {
	if _, err := s.services.GetService(ctx, serviceID); err != nil {
		return nil, err
	}
	return s.reviews.ListServiceReviews(ctx, serviceID, limit, offset)
}
