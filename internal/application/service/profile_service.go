package service

import (
	"context"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/foundation"
	"smallworld/internal/ports"
)

type ProfileService struct {
	users ports.UserRepository
	idg   foundation.IDGenerator
}

func NewProfileService(users ports.UserRepository, idg foundation.IDGenerator) *ProfileService {
	return &ProfileService{users: users, idg: idg}
}

type UpsertProfileInput struct {
	DisplayName string                 `json:"display_name"`
	Preferences domain.UserPreferences `json:"preferences"`
}

func (s *ProfileService) UpsertAuthenticated(ctx context.Context, authSubject string, input UpsertProfileInput) (domain.User, error) {
	user, err := s.users.FindByAuthSubject(ctx, authSubject)
	switch err {
	case nil:
		user.DisplayName = input.DisplayName
		user.Preferences = input.Preferences
	case domain.ErrUserNotFound:
		user = domain.User{
			ID:          s.idg.New("user"),
			AuthSubject: authSubject,
			DisplayName: input.DisplayName,
			Preferences: input.Preferences,
			CreatedAt:   time.Now().UTC(),
		}
	default:
		return domain.User{}, err
	}

	if err := s.users.Save(ctx, user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *ProfileService) FindByAuthSubject(ctx context.Context, authSubject string) (domain.User, error) {
	return s.users.FindByAuthSubject(ctx, authSubject)
}
