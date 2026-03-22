package service

import (
	"context"
	"testing"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/foundation"
	"smallworld/internal/infrastructure/memory"
	"smallworld/internal/infrastructure/pricing"
	"smallworld/internal/infrastructure/push"
	"smallworld/internal/infrastructure/realtime"
	"smallworld/internal/infrastructure/routing"
	"smallworld/internal/matching"
)

type stubPresenceStore struct {
	sessions []domain.DriverSession
}

func (s stubPresenceStore) SaveSession(context.Context, domain.DriverSession) error { return nil }
func (s stubPresenceStore) DeleteSession(context.Context, string) error             { return nil }
func (s stubPresenceStore) ListActiveSessions(context.Context) ([]domain.DriverSession, error) {
	return s.sessions, nil
}

func TestCreateTripDemandIsIdempotent(t *testing.T) {
	store := memory.NewStore()
	idg := &foundation.AtomicIDGenerator{}
	now := time.Now().UTC()

	if err := store.Save(context.Background(), domain.User{ID: "rider_1", DisplayName: "Rider", CreatedAt: now}); err != nil {
		t.Fatalf("save user: %v", err)
	}
	if err := store.SaveVerification(context.Background(), domain.IdentityVerification{
		UserID:         "rider_1",
		Status:         domain.VerificationVerified,
		VerifiedGender: domain.GenderFemale,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("save verification: %v", err)
	}

	svc := NewTripDemandService(
		memory.TripDemands{Store: store},
		memory.Verifications{Store: store},
		memory.Idempotency{Store: store},
		memory.DriverSessions{Store: store},
		memory.RideOffers{Store: store},
		memory.RideBookings{Store: store},
		memory.Vehicles{Store: store},
		routing.NewHaversineProvider(),
		pricing.NewFixedFormula(),
		push.NoopNotifier{},
		realtime.NoopHub{},
		nil,
		nil,
		idg,
		matching.NewEngine(routing.NewHaversineProvider(), matching.Config{
			ETASafetyBufferSeconds: 30,
			PickupSearchStepMeters: 100,
		}),
	)

	input := CreateTripDemandInput{
		RiderID:                  "rider_1",
		RequestedOrigin:          domain.Location{Lat: 45, Lng: 9},
		RequestedDestination:     domain.Location{Lat: 45.01, Lng: 9.01},
		MaxWalkToPickupMeters:    400,
		MaxWalkFromDropoffMeters: 400,
		IdempotencyKey:           "same-key",
	}

	first, _, err := svc.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("first create returned error: %v", err)
	}
	second, _, err := svc.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("second create returned error: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected same demand id, got %s and %s", first.ID, second.ID)
	}
}

func TestTripDemandPrefersPresenceSessionsForMatching(t *testing.T) {
	store := memory.NewStore()
	idg := &foundation.AtomicIDGenerator{}
	now := time.Now().UTC()

	_ = store.Save(context.Background(), domain.User{ID: "rider_1", DisplayName: "Rider", CreatedAt: now})
	_ = store.SaveVerification(context.Background(), domain.IdentityVerification{
		UserID:         "rider_1",
		Status:         domain.VerificationVerified,
		VerifiedGender: domain.GenderFemale,
		UpdatedAt:      now,
	})
	_ = store.SaveVerification(context.Background(), domain.IdentityVerification{
		UserID:         "driver_1",
		Status:         domain.VerificationVerified,
		VerifiedGender: domain.GenderFemale,
		UpdatedAt:      now,
	})

	presence := stubPresenceStore{
		sessions: []domain.DriverSession{
			{
				ID:                          "ds_live",
				DriverID:                    "driver_1",
				State:                       domain.DriverSessionStateActive,
				Origin:                      domain.Location{Lat: 45.0002, Lng: 9.0002},
				Destination:                 domain.Location{Lat: 45.0102, Lng: 9.0102},
				CurrentLocation:             domain.Location{Lat: 45.0001, Lng: 9.0001},
				RemainingCapacity:           2,
				MaxDriverPickupDetourMeters: 1500,
				LastHeartbeatAt:             now,
			},
		},
	}

	svc := NewTripDemandService(
		memory.TripDemands{Store: store},
		memory.Verifications{Store: store},
		memory.Idempotency{Store: store},
		memory.DriverSessions{Store: store},
		memory.RideOffers{Store: store},
		memory.RideBookings{Store: store},
		memory.Vehicles{Store: store},
		routing.NewHaversineProvider(),
		pricing.NewFixedFormula(),
		push.NoopNotifier{},
		realtime.NoopHub{},
		presence,
		nil,
		idg,
		matching.NewEngine(routing.NewHaversineProvider(), matching.Config{
			MaxDriverSessionStalenessSeconds: 30,
			ETASafetyBufferSeconds:           30,
			PickupSearchStepMeters:           100,
		}),
	)

	demand, offer, err := svc.Create(context.Background(), CreateTripDemandInput{
		RiderID:                  "rider_1",
		RequestedOrigin:          domain.Location{Lat: 45.0000, Lng: 9.0000},
		RequestedDestination:     domain.Location{Lat: 45.0100, Lng: 9.0100},
		MaxWalkToPickupMeters:    400,
		MaxWalkFromDropoffMeters: 400,
	})
	if err != nil {
		t.Fatalf("create trip demand returned error: %v", err)
	}
	if offer == nil {
		t.Fatalf("expected a live offer from presence-backed sessions")
	}
	if offer.DriverSessionID != "ds_live" {
		t.Fatalf("expected offer to use live presence session, got %s", offer.DriverSessionID)
	}
	if demand.State != domain.TripDemandStateSearching {
		t.Fatalf("expected returned demand snapshot to be the created searching record, got %s", demand.State)
	}
}
