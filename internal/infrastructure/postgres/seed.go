package postgres

import (
	"context"
	"time"

	"smallworld/internal/domain"
)

func SeedDemoData(ctx context.Context, users Users, verifications Verifications, vehicles Vehicles) error {
	now := time.Now().UTC()

	if err := users.Save(ctx, domain.User{
		ID:          "user_driver_1",
		AuthSubject: "driver_1",
		DisplayName: "Alice Driver",
		Preferences: domain.UserPreferences{
			MaxWalkToPickupMeters:       400,
			MaxWalkFromDropoffMeters:    400,
			MaxDriverPickupDetourMeters: 1200,
		},
		CreatedAt: now,
	}); err != nil {
		return err
	}

	if err := users.Save(ctx, domain.User{
		ID:          "user_rider_1",
		AuthSubject: "rider_1",
		DisplayName: "Rita Rider",
		Preferences: domain.UserPreferences{
			MaxWalkToPickupMeters:       400,
			MaxWalkFromDropoffMeters:    400,
			MaxDriverPickupDetourMeters: 0,
		},
		CreatedAt: now,
	}); err != nil {
		return err
	}

	if err := verifications.Save(ctx, domain.IdentityVerification{
		UserID:         "user_driver_1",
		Status:         domain.VerificationVerified,
		Provider:       "demo",
		ProviderRef:    "drv_1",
		VerifiedGender: domain.GenderFemale,
		UpdatedAt:      now,
	}); err != nil {
		return err
	}

	if err := verifications.Save(ctx, domain.IdentityVerification{
		UserID:         "user_rider_1",
		Status:         domain.VerificationVerified,
		Provider:       "demo",
		ProviderRef:    "rdr_1",
		VerifiedGender: domain.GenderFemale,
		UpdatedAt:      now,
	}); err != nil {
		return err
	}

	return vehicles.Save(ctx, domain.Vehicle{
		ID:           "veh_1",
		UserID:       "user_driver_1",
		Make:         "Toyota",
		Model:        "Yaris",
		Color:        "Blue",
		LicensePlate: "SW-001",
		Capacity:     3,
		IsActive:     true,
		CreatedAt:    now,
	})
}
