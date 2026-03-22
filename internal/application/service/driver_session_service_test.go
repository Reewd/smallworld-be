package service

import (
	"context"
	"testing"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/foundation"
	"smallworld/internal/infrastructure/memory"
	"smallworld/internal/ports"
)

func TestHeartbeatOwnedRejectsWrongDriver(t *testing.T) {
	store := memory.NewStore()
	now := time.Now().UTC()

	if err := store.SaveDriverSession(context.Background(), domain.DriverSession{
		ID:              "ds_1",
		DriverID:        "driver_1",
		State:           domain.DriverSessionStateActive,
		CurrentLocation: domain.Location{Lat: 45, Lng: 9},
		LastHeartbeatAt: now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatalf("save session: %v", err)
	}

	svc := NewDriverSessionService(
		memory.DriverSessions{Store: store},
		memory.Verifications{Store: store},
		memory.Vehicles{Store: store},
		stubRoutingProvider{},
		memory.Idempotency{Store: store},
		nil,
		&foundation.AtomicIDGenerator{},
	)

	_, err := svc.HeartbeatOwned(context.Background(), "driver_2", HeartbeatDriverSessionInput{
		SessionID:       "ds_1",
		CurrentLocation: domain.Location{Lat: 45.001, Lng: 9.001},
	})
	if err != domain.ErrUnauthorized {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestStartPersistsRoutePolyline(t *testing.T) {
	store := memory.NewStore()
	now := time.Now().UTC()

	if err := store.SaveVerification(context.Background(), domain.IdentityVerification{
		UserID:         "driver_1",
		Status:         domain.VerificationVerified,
		VerifiedGender: domain.GenderFemale,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("save verification: %v", err)
	}

	if err := store.SaveVehicle(context.Background(), domain.Vehicle{
		ID:           "vehicle_1",
		UserID:       "driver_1",
		Make:         "Fiat",
		Model:        "500",
		Color:        "Blue",
		LicensePlate: "AB123CD",
		Capacity:     3,
		IsActive:     true,
		CreatedAt:    now,
	}); err != nil {
		t.Fatalf("save vehicle: %v", err)
	}

	svc := NewDriverSessionService(
		memory.DriverSessions{Store: store},
		memory.Verifications{Store: store},
		memory.Vehicles{Store: store},
		stubRoutingProvider{},
		memory.Idempotency{Store: store},
		nil,
		&foundation.AtomicIDGenerator{},
	)

	session, err := svc.Start(context.Background(), StartDriverSessionInput{
		UserID:                      "driver_1",
		VehicleID:                   "vehicle_1",
		Origin:                      domain.Location{Lat: 45.46, Lng: 9.19},
		Destination:                 domain.Location{Lat: 45.50, Lng: 9.25},
		CurrentLocation:             domain.Location{Lat: 45.46, Lng: 9.19},
		MaxDriverPickupDetourMeters: 1200,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if session.RoutePolyline != "encoded-polyline" {
		t.Fatalf("RoutePolyline = %q", session.RoutePolyline)
	}
}

type stubRoutingProvider struct{}

func (stubRoutingProvider) Route(context.Context, domain.Location, domain.Location) (ports.RoutePlan, error) {
	return ports.RoutePlan{
		DistanceMeters:  1234,
		DurationSeconds: 456,
		Polyline:        "encoded-polyline",
	}, nil
}

func (stubRoutingProvider) WalkingETASeconds(context.Context, domain.Location, domain.Location) (int, int, error) {
	return 120, 150, nil
}

func (stubRoutingProvider) DrivingETASeconds(context.Context, domain.Location, domain.Location) (int, int, error) {
	return 60, 500, nil
}
