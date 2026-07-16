package main

import "context"

type StatsStore interface {
	GetUserStats(context.Context, int) (UserStats, error)
}

type StatsService struct {
	store StatsStore
}

func NewStatsService(store StatsStore) *StatsService {
	return &StatsService{store: store}
}

func (s *StatsService) Get(ctx context.Context, userID int) (UserStats, error) {
	return s.store.GetUserStats(ctx, userID)
}
