package service

import (
	"context"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/foundation"
	"smallworld/internal/ports"
)

type DevBootstrapService struct {
	users         ports.UserRepository
	verifications ports.VerificationRepository
	vehicles      ports.VehicleRepository
	idg           foundation.IDGenerator
}

func NewDevBootstrapService(
	users ports.UserRepository,
	verifications ports.VerificationRepository,
	vehicles ports.VehicleRepository,
	idg foundation.IDGenerator,
) *DevBootstrapService {
	return &DevBootstrapService{
		users:         users,
		verifications: verifications,
		vehicles:      vehicles,
		idg:           idg,
	}
}

type DevBootstrapInput struct {
	AuthSubject    string
	DisplayName    string
	VerifiedGender domain.Gender
	Vehicle        *DevBootstrapVehicleInput
}

type DevBootstrapVehicleInput struct {
	Make         string `json:"make"`
	Model        string `json:"model"`
	Color        string `json:"color"`
	LicensePlate string `json:"license_plate"`
	Capacity     int    `json:"capacity"`
}

type DevBootstrapResult struct {
	User         domain.User                 `json:"user"`
	Verification domain.IdentityVerification `json:"verification"`
	Vehicle      *domain.Vehicle             `json:"vehicle,omitempty"`
}

func (s *DevBootstrapService) Bootstrap(ctx context.Context, input DevBootstrapInput) (DevBootstrapResult, error) {
	now := time.Now().UTC()

	user, err := s.users.FindByAuthSubject(ctx, input.AuthSubject)
	switch err {
	case nil:
		if input.DisplayName != "" {
			user.DisplayName = input.DisplayName
		}
	case domain.ErrUserNotFound:
		displayName := input.DisplayName
		if displayName == "" {
			displayName = input.AuthSubject
		}
		user = domain.User{
			ID:          s.idg.New("user"),
			AuthSubject: input.AuthSubject,
			DisplayName: displayName,
			Preferences: defaultUserPreferences(),
			CreatedAt:   now,
		}
	default:
		return DevBootstrapResult{}, err
	}

	if err := s.users.Save(ctx, user); err != nil {
		return DevBootstrapResult{}, err
	}

	gender := input.VerifiedGender
	if gender == "" {
		gender = domain.GenderUnknown
	}
	verification := domain.IdentityVerification{
		UserID:         user.ID,
		Status:         domain.VerificationVerified,
		Provider:       "dev_emulator",
		ProviderRef:    input.AuthSubject,
		VerifiedGender: gender,
		VerifiedAt:     ptrTime(now),
		UpdatedAt:      now,
	}
	if err := s.verifications.Save(ctx, verification); err != nil {
		return DevBootstrapResult{}, err
	}

	var vehicle *domain.Vehicle
	if input.Vehicle != nil {
		vehicles, err := s.vehicles.ListByUserID(ctx, user.ID)
		if err != nil {
			return DevBootstrapResult{}, err
		}
		for _, existing := range vehicles {
			if existing.LicensePlate == input.Vehicle.LicensePlate {
				copyVehicle := existing
				vehicle = &copyVehicle
				break
			}
		}
		if vehicle == nil {
			created := domain.Vehicle{
				ID:           s.idg.New("veh"),
				UserID:       user.ID,
				Make:         input.Vehicle.Make,
				Model:        input.Vehicle.Model,
				Color:        input.Vehicle.Color,
				LicensePlate: input.Vehicle.LicensePlate,
				Capacity:     input.Vehicle.Capacity,
				IsActive:     true,
				CreatedAt:    now,
			}
			if err := s.vehicles.Save(ctx, created); err != nil {
				return DevBootstrapResult{}, err
			}
			vehicle = &created
		}
	}

	return DevBootstrapResult{
		User:         user,
		Verification: verification,
		Vehicle:      vehicle,
	}, nil
}

func defaultUserPreferences() domain.UserPreferences {
	return domain.UserPreferences{
		MaxWalkToPickupMeters:       300,
		MaxWalkFromDropoffMeters:    300,
		MaxDriverPickupDetourMeters: 1000,
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
