package main

import (
	"context"
	"errors"
	"strings"
)

const welcomeCredits = 10

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrEmptyPseudo     = errors.New("pseudo must not be empty")
	ErrInvalidSkill    = errors.New("skill name must not be empty")
	ErrDuplicateSkill  = errors.New("skill names must be unique")
	ErrInvalidLevel    = errors.New("level must be 'débutant', 'intermédiaire' or 'expert'")
	ErrUnauthenticated = errors.New("missing or invalid X-User-ID header")
	ErrForbidden       = errors.New("you can only modify your own profile")
)

type UserStore interface {
	CreateUser(context.Context, CreateUserInput, int) (User, error)
	GetUser(context.Context, int) (User, error)
	UpdateUser(context.Context, int, UpdateUserInput) (User, error)
	GetSkills(context.Context, int) ([]Skill, error)
	ReplaceSkills(context.Context, int, []Skill) ([]Skill, error)
}

type UserService struct {
	store UserStore
}

func NewUserService(store UserStore) *UserService {
	return &UserService{store: store}
}

func (s *UserService) Create(ctx context.Context, input CreateUserInput) (User, error) {
	input.Pseudo = sanitizeText(input.Pseudo, 50)
	input.Bio = sanitizeText(input.Bio, 500)
	input.Ville = sanitizeText(input.Ville, 100)
	if input.Pseudo == "" {
		return User{}, ErrEmptyPseudo
	}
	return s.store.CreateUser(ctx, input, welcomeCredits)
}

func (s *UserService) Get(ctx context.Context, id int) (User, error) {
	user, err := s.store.GetUser(ctx, id)
	if err != nil {
		return User{}, err
	}
	skills, err := s.store.GetSkills(ctx, id)
	if err != nil {
		return User{}, err
	}
	user.Skills = skills
	return user, nil
}

func (s *UserService) Update(ctx context.Context, authenticatedID, id int, input UpdateUserInput) (User, error) {
	if authenticatedID != id {
		return User{}, ErrForbidden
	}
	input.Pseudo = sanitizeText(input.Pseudo, 50)
	input.Bio = sanitizeText(input.Bio, 500)
	input.Ville = sanitizeText(input.Ville, 100)
	if input.Pseudo == "" {
		return User{}, ErrEmptyPseudo
	}
	return s.store.UpdateUser(ctx, id, input)
}

func (s *UserService) Skills(ctx context.Context, id int) ([]Skill, error) {
	return s.store.GetSkills(ctx, id)
}

func (s *UserService) ReplaceSkills(ctx context.Context, authenticatedID, id int, skills []Skill) ([]Skill, error) {
	if authenticatedID != id {
		return nil, ErrForbidden
	}
	seen := make(map[string]bool, len(skills))
	for i := range skills {
		skills[i].Nom = sanitizeText(skills[i].Nom, 50)
		skills[i].Niveau = strings.ToLower(sanitizeText(skills[i].Niveau, 20))
		if skills[i].Nom == "" {
			return nil, ErrInvalidSkill
		}
		if seen[skills[i].Nom] {
			return nil, ErrDuplicateSkill
		}
		seen[skills[i].Nom] = true
		switch skills[i].Niveau {
		case "débutant", "intermédiaire", "expert":
		default:
			return nil, ErrInvalidLevel
		}
	}
	return s.store.ReplaceSkills(ctx, id, skills)
}
