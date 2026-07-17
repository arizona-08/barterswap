package main

import (
	"context"
	"errors"
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

type ServiceReader interface {
	GetService(context.Context, int) (Service, error)
	ListServices(context.Context, ServiceFilters) ([]Service, error)
}

type ServiceWriter interface {
	CreateService(context.Context, int, CreateServiceInput) (Service, error)
	UpdateService(context.Context, int, UpdateServiceInput) (Service, error)
	DeleteService(context.Context, int) error
}

type SkillChecker interface {
	HasSkill(context.Context, int, string) (bool, error)
}

type ServiceStore interface {
	ServiceReader
	ServiceWriter
	SkillChecker
}

type ServiceService struct {
	store ServiceStore
}

func NewServiceService(store ServiceStore) *ServiceService {
	return &ServiceService{store: store}
}

func (s *ServiceService) Create(ctx context.Context, providerID int, input CreateServiceInput) (Service, error) {
	input.Titre = sanitizeText(input.Titre, 100)
	input.Description = sanitizeText(input.Description, 1000)
	input.Categorie = sanitizeText(input.Categorie, 50)
	input.Ville = sanitizeText(input.Ville, 100)
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
	filters.Categorie = sanitizeText(filters.Categorie, 50)
	filters.Ville = sanitizeText(filters.Ville, 100)
	filters.Search = sanitizeText(filters.Search, 100)
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
	input.Titre = sanitizeText(input.Titre, 100)
	input.Description = sanitizeText(input.Description, 1000)
	input.Categorie = sanitizeText(input.Categorie, 50)
	input.Ville = sanitizeText(input.Ville, 100)
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
