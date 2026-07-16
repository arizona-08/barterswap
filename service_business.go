package main

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrServiceNotFound    = errors.New("service not found")
	ErrInvalidService     = errors.New("service fields are invalid")
	ErrInvalidCategory    = errors.New("invalid service category")
	ErrProviderLacksSkill = errors.New("provider does not have this skill")
	ErrServiceInactive    = errors.New("service is inactive")
)

var serviceCategories = map[string]bool{
	"Informatique": true,
	"Jardinage":    true,
	"Bricolage":    true,
	"Cuisine":      true,
	"Musique":      true,
	"Langues":      true,
	"Sport":        true,
	"Tutorat":      true,
	"Déménagement": true,
	"Photographie": true,
	"Animalier":    true,
	"Couture":      true,
	"Autre":        true,
}

type ServiceStore interface {
	CreateService(context.Context, int, CreateServiceInput) (Service, error)
	GetService(context.Context, int) (Service, error)
	ListServices(context.Context, ServiceFilters) ([]Service, error)
	UpdateService(context.Context, int, UpdateServiceInput) (Service, error)
	DeleteService(context.Context, int) error
	HasSkill(context.Context, int, string) (bool, error)
}

type ServiceService struct {
	store ServiceStore
}

func NewServiceService(store ServiceStore) *ServiceService {
	return &ServiceService{store: store}
}

func (s *ServiceService) Create(ctx context.Context, providerID int, input CreateServiceInput) (Service, error) {
	input.Titre = strings.TrimSpace(input.Titre)
	input.Description = strings.TrimSpace(input.Description)
	input.Categorie = strings.TrimSpace(input.Categorie)
	input.Ville = strings.TrimSpace(input.Ville)
	if err := validateServiceInput(input.Titre, input.Categorie, input.DureeMinutes, input.Credits); err != nil {
		return Service{}, err
	}
	if !serviceCategories[input.Categorie] {
		return Service{}, ErrInvalidCategory
	}
	hasSkill, err := s.store.HasSkill(ctx, providerID, input.Categorie)
	if err != nil {
		return Service{}, err
	}
	if !hasSkill {
		return Service{}, ErrProviderLacksSkill
	}
	return s.store.CreateService(ctx, providerID, input)
}

func (s *ServiceService) Get(ctx context.Context, id int) (Service, error) {
	return s.store.GetService(ctx, id)
}

func (s *ServiceService) List(ctx context.Context, filters ServiceFilters) ([]Service, error) {
	filters.Categorie = strings.TrimSpace(filters.Categorie)
	filters.Ville = strings.TrimSpace(filters.Ville)
	filters.Search = strings.TrimSpace(filters.Search)
	return s.store.ListServices(ctx, filters)
}

func (s *ServiceService) Update(ctx context.Context, providerID, id int, input UpdateServiceInput) (Service, error) {
	service, err := s.store.GetService(ctx, id)
	if err != nil {
		return Service{}, err
	}
	if service.ProviderID != providerID {
		return Service{}, ErrForbidden
	}
	input.Titre = strings.TrimSpace(input.Titre)
	input.Description = strings.TrimSpace(input.Description)
	input.Categorie = strings.TrimSpace(input.Categorie)
	input.Ville = strings.TrimSpace(input.Ville)
	if err := validateServiceInput(input.Titre, input.Categorie, input.DureeMinutes, input.Credits); err != nil {
		return Service{}, err
	}
	if !serviceCategories[input.Categorie] {
		return Service{}, ErrInvalidCategory
	}
	hasSkill, err := s.store.HasSkill(ctx, providerID, input.Categorie)
	if err != nil {
		return Service{}, err
	}
	if !hasSkill {
		return Service{}, ErrProviderLacksSkill
	}
	return s.store.UpdateService(ctx, id, input)
}

func (s *ServiceService) Delete(ctx context.Context, providerID, id int) error {
	service, err := s.store.GetService(ctx, id)
	if err != nil {
		return err
	}
	if service.ProviderID != providerID {
		return ErrForbidden
	}
	return s.store.DeleteService(ctx, id)
}

func validateServiceInput(title, category string, duration, credits int) error {
	if title == "" || category == "" || duration <= 0 || credits <= 0 {
		return ErrInvalidService
	}
	return nil
}
