package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/foundation"
	"smallworld/internal/infrastructure/memory"
	"smallworld/internal/infrastructure/pricing"
	"smallworld/internal/infrastructure/push"
	"smallworld/internal/infrastructure/realtime"
)

func TestAcceptPreventsOverbooking(t *testing.T) {
	store := memory.NewStore()
	idg := &foundation.AtomicIDGenerator{}
	now := time.Now().UTC()

	if err := store.SaveVehicle(context.Background(), domain.Vehicle{
		ID:           "veh_1",
		UserID:       "driver_1",
		LicensePlate: "SW-001",
		Capacity:     1,
		IsActive:     true,
		CreatedAt:    now,
	}); err != nil {
		t.Fatalf("save vehicle: %v", err)
	}
	if err := store.SaveDriverSession(context.Background(), domain.DriverSession{
		ID:                "ds_1",
		DriverID:          "driver_1",
		VehicleID:         "veh_1",
		State:             domain.DriverSessionStateActive,
		RemainingCapacity: 1,
		UpdatedAt:         now,
		CreatedAt:         now,
	}); err != nil {
		t.Fatalf("save driver session: %v", err)
	}

	for _, demand := range []domain.TripDemand{
		{ID: "td_1", RiderID: "rider_1", State: domain.TripDemandStateOffered, RequestedOrigin: domain.Location{Lat: 45, Lng: 9}, RequestedDestination: domain.Location{Lat: 45.01, Lng: 9.01}},
		{ID: "td_2", RiderID: "rider_2", State: domain.TripDemandStateOffered, RequestedOrigin: domain.Location{Lat: 45.001, Lng: 9.001}, RequestedDestination: domain.Location{Lat: 45.02, Lng: 9.02}},
	} {
		if err := store.SaveTripDemand(context.Background(), demand); err != nil {
			t.Fatalf("save demand: %v", err)
		}
	}
	for _, offer := range []domain.RideOffer{
		{ID: "offer_1", DemandID: "td_1", DriverSessionID: "ds_1", State: domain.RideOfferStatePending, FareCents: 500},
		{ID: "offer_2", DemandID: "td_2", DriverSessionID: "ds_1", State: domain.RideOfferStatePending, FareCents: 500},
	} {
		if err := store.SaveRideOffer(context.Background(), offer); err != nil {
			t.Fatalf("save offer: %v", err)
		}
	}

	svc := NewOfferService(
		memory.RideOffers{Store: store},
		memory.TripDemands{Store: store},
		memory.DriverSessions{Store: store},
		memory.RideBookings{Store: store},
		memory.Vehicles{Store: store},
		pricing.NewFixedFormula(),
		push.NoopNotifier{},
		realtime.NoopHub{},
		nil,
		nil,
		idg,
	)

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, offerID := range []string{"offer_1", "offer_2"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, err := svc.Accept(context.Background(), "driver_1", id)
			results <- err
		}(offerID)
	}
	wg.Wait()
	close(results)

	var successCount int
	var capacityErrorCount int
	for err := range results {
		switch err {
		case nil:
			successCount++
		case domain.ErrCapacityExceeded:
			capacityErrorCount++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if successCount != 1 || capacityErrorCount != 1 {
		t.Fatalf("expected one success and one capacity error, got success=%d capacity=%d", successCount, capacityErrorCount)
	}
}

func TestAcceptRejectsWrongDriver(t *testing.T) {
	store := memory.NewStore()
	idg := &foundation.AtomicIDGenerator{}
	now := time.Now().UTC()

	_ = store.SaveVehicle(context.Background(), domain.Vehicle{
		ID:           "veh_1",
		UserID:       "driver_1",
		LicensePlate: "SW-001",
		Capacity:     1,
		IsActive:     true,
		CreatedAt:    now,
	})
	_ = store.SaveDriverSession(context.Background(), domain.DriverSession{
		ID:                "ds_1",
		DriverID:          "driver_1",
		VehicleID:         "veh_1",
		State:             domain.DriverSessionStateActive,
		RemainingCapacity: 1,
		UpdatedAt:         now,
		CreatedAt:         now,
	})
	_ = store.SaveTripDemand(context.Background(), domain.TripDemand{
		ID:                   "td_1",
		RiderID:              "rider_1",
		State:                domain.TripDemandStateOffered,
		RequestedOrigin:      domain.Location{Lat: 45, Lng: 9},
		RequestedDestination: domain.Location{Lat: 45.01, Lng: 9.01},
	})
	_ = store.SaveRideOffer(context.Background(), domain.RideOffer{
		ID:              "offer_1",
		DemandID:        "td_1",
		DriverSessionID: "ds_1",
		State:           domain.RideOfferStatePending,
		FareCents:       500,
	})

	svc := NewOfferService(
		memory.RideOffers{Store: store},
		memory.TripDemands{Store: store},
		memory.DriverSessions{Store: store},
		memory.RideBookings{Store: store},
		memory.Vehicles{Store: store},
		pricing.NewFixedFormula(),
		push.NoopNotifier{},
		realtime.NoopHub{},
		nil,
		nil,
		idg,
	)

	_, err := svc.Accept(context.Background(), "driver_2", "offer_1")
	if err != domain.ErrUnauthorized {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}
