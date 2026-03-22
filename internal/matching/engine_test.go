package matching

import (
	"context"
	"testing"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/infrastructure/routing"
)

func TestPhaseOnePreferredBeforeWalking(t *testing.T) {
	engine := NewEngine(routing.NewHaversineProvider(), Config{
		ETASafetyBufferSeconds: 30,
		PickupSearchStepMeters: 100,
	})

	demand := domain.TripDemand{
		ID:                       "td_1",
		RiderID:                  "rider_1",
		RequestedOrigin:          domain.Location{Lat: 45.0000, Lng: 9.0000},
		RequestedDestination:     domain.Location{Lat: 45.0100, Lng: 9.0100},
		MaxWalkToPickupMeters:    400,
		MaxWalkFromDropoffMeters: 400,
	}

	session := domain.DriverSession{
		ID:                          "ds_1",
		DriverID:                    "driver_1",
		State:                       domain.DriverSessionStateActive,
		Origin:                      domain.Location{Lat: 45.0003, Lng: 9.0003},
		Destination:                 domain.Location{Lat: 45.0103, Lng: 9.0103},
		CurrentLocation:             domain.Location{Lat: 45.0002, Lng: 9.0002},
		RemainingCapacity:           2,
		MaxDriverPickupDetourMeters: 1500,
		LastHeartbeatAt:             time.Now().UTC(),
	}

	candidates, err := engine.FindCandidates(
		context.Background(),
		demand,
		domain.IdentityVerification{Status: domain.VerificationVerified},
		map[string]domain.IdentityVerification{
			"driver_1": {Status: domain.VerificationVerified, VerifiedGender: domain.GenderFemale},
		},
		[]domain.DriverSession{session},
	)
	if err != nil {
		t.Fatalf("FindCandidates returned error: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatalf("expected at least one candidate")
	}
	if candidates[0].Phase != 1 {
		t.Fatalf("expected phase 1 candidate to be preferred, got phase %d", candidates[0].Phase)
	}
}

func TestRejectsPickupIfDriverWouldNeedToWait(t *testing.T) {
	engine := NewEngine(routing.NewHaversineProvider(), Config{
		ETASafetyBufferSeconds: 30,
		PickupSearchStepMeters: 100,
	})

	demand := domain.TripDemand{
		ID:                       "td_1",
		RiderID:                  "rider_1",
		RequestedOrigin:          domain.Location{Lat: 45.0000, Lng: 9.0000},
		RequestedDestination:     domain.Location{Lat: 45.0100, Lng: 9.0100},
		MaxWalkToPickupMeters:    1200,
		MaxWalkFromDropoffMeters: 400,
	}

	session := domain.DriverSession{
		ID:                          "ds_1",
		DriverID:                    "driver_1",
		State:                       domain.DriverSessionStateActive,
		Origin:                      domain.Location{Lat: 45.0000, Lng: 9.0000},
		Destination:                 domain.Location{Lat: 45.0100, Lng: 9.0100},
		CurrentLocation:             domain.Location{Lat: 45.0001, Lng: 9.0001},
		RemainingCapacity:           2,
		MaxDriverPickupDetourMeters: 1500,
		LastHeartbeatAt:             time.Now().UTC(),
	}

	candidate, ok, err := engine.evaluateCandidate(
		context.Background(),
		demand,
		session,
		domain.Location{Lat: 45.0030, Lng: 9.0030},
		demand.RequestedDestination,
		2,
	)
	if err != nil {
		t.Fatalf("evaluateCandidate returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected candidate to be rejected because the driver would wait, got %+v", candidate)
	}
}

func TestStaleDriverSessionIsRejected(t *testing.T) {
	engine := NewEngine(routing.NewHaversineProvider(), Config{
		MaxDriverSessionStalenessSeconds: 10,
		ETASafetyBufferSeconds:           30,
		PickupSearchStepMeters:           100,
	})

	demand := domain.TripDemand{
		ID:                       "td_1",
		RiderID:                  "rider_1",
		RequestedOrigin:          domain.Location{Lat: 45.0000, Lng: 9.0000},
		RequestedDestination:     domain.Location{Lat: 45.0100, Lng: 9.0100},
		MaxWalkToPickupMeters:    400,
		MaxWalkFromDropoffMeters: 400,
	}

	session := domain.DriverSession{
		ID:                          "ds_stale",
		DriverID:                    "driver_1",
		State:                       domain.DriverSessionStateActive,
		Origin:                      domain.Location{Lat: 45.0001, Lng: 9.0001},
		Destination:                 domain.Location{Lat: 45.0101, Lng: 9.0101},
		CurrentLocation:             domain.Location{Lat: 45.0001, Lng: 9.0001},
		RemainingCapacity:           2,
		MaxDriverPickupDetourMeters: 1500,
		LastHeartbeatAt:             time.Now().UTC().Add(-time.Minute),
	}

	candidates, err := engine.FindCandidates(
		context.Background(),
		demand,
		domain.IdentityVerification{Status: domain.VerificationVerified},
		map[string]domain.IdentityVerification{
			"driver_1": {Status: domain.VerificationVerified, VerifiedGender: domain.GenderFemale},
		},
		[]domain.DriverSession{session},
	)
	if err != nil {
		t.Fatalf("FindCandidates returned error: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected stale session to be rejected, got %d candidates", len(candidates))
	}
}
