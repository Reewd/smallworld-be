package service

import (
	"context"
	"testing"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/foundation"
	"smallworld/internal/infrastructure/memory"
)

func TestDevBootstrapCreatesVerifiedUserAndVehicle(t *testing.T) {
	store := memory.NewStore()
	svc := NewDevBootstrapService(
		memory.Users{Store: store},
		memory.Verifications{Store: store},
		memory.Vehicles{Store: store},
		&foundation.AtomicIDGenerator{},
	)

	result, err := svc.Bootstrap(context.Background(), DevBootstrapInput{
		AuthSubject:    "emulator-user",
		DisplayName:    "Andrea",
		VerifiedGender: domain.GenderFemale,
		Vehicle: &DevBootstrapVehicleInput{
			Make:         "Toyota",
			Model:        "Yaris",
			Color:        "Blue",
			LicensePlate: "DEV-001",
			Capacity:     3,
		},
	})
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if result.User.AuthSubject != "emulator-user" {
		t.Fatalf("AuthSubject = %q", result.User.AuthSubject)
	}
	if result.Verification.Provider != "dev_emulator" {
		t.Fatalf("Provider = %q", result.Verification.Provider)
	}
	if result.Verification.ProviderRef != "emulator-user" {
		t.Fatalf("ProviderRef = %q", result.Verification.ProviderRef)
	}
	if result.Verification.Status != domain.VerificationVerified {
		t.Fatalf("Status = %q", result.Verification.Status)
	}
	if result.Vehicle == nil || result.Vehicle.LicensePlate != "DEV-001" {
		t.Fatalf("Vehicle = %#v", result.Vehicle)
	}
}

func TestDevBootstrapReusesExistingVehicleByLicensePlate(t *testing.T) {
	store := memory.NewStore()
	now := time.Now().UTC()
	if err := store.Save(context.Background(), domain.User{
		ID:          "user_1",
		AuthSubject: "emulator-user",
		DisplayName: "Existing",
		Preferences: domain.UserPreferences{
			WalkToPickup:       domain.PreferenceLevelMedium,
			WalkFromDropoff:    domain.PreferenceLevelMedium,
			DriverPickupDetour: domain.PreferenceLevelMedium,
		},
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("save user: %v", err)
	}
	if err := store.SaveVehicle(context.Background(), domain.Vehicle{
		ID:           "veh_1",
		UserID:       "user_1",
		Make:         "Toyota",
		Model:        "Yaris",
		Color:        "Blue",
		LicensePlate: "DEV-001",
		Capacity:     3,
		IsActive:     true,
		CreatedAt:    now,
	}); err != nil {
		t.Fatalf("save vehicle: %v", err)
	}

	svc := NewDevBootstrapService(
		memory.Users{Store: store},
		memory.Verifications{Store: store},
		memory.Vehicles{Store: store},
		&foundation.AtomicIDGenerator{},
	)

	result, err := svc.Bootstrap(context.Background(), DevBootstrapInput{
		AuthSubject: "emulator-user",
		Vehicle: &DevBootstrapVehicleInput{
			Make:         "Toyota",
			Model:        "Yaris",
			Color:        "Blue",
			LicensePlate: "DEV-001",
			Capacity:     3,
		},
	})
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if result.Vehicle == nil || result.Vehicle.ID != "veh_1" {
		t.Fatalf("Vehicle = %#v", result.Vehicle)
	}
}
