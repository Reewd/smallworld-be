package service

import (
	"context"
	"testing"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/foundation"
	"smallworld/internal/infrastructure/memory"
)

func TestDriverSessionGetCurrentForDriverPrefersActive(t *testing.T) {
	store := memory.NewStore()
	now := time.Now().UTC()
	if err := store.SaveDriverSession(context.Background(), domain.DriverSession{
		ID:        "paused",
		DriverID:  "driver_1",
		State:     domain.DriverSessionStatePaused,
		UpdatedAt: now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("save paused session: %v", err)
	}
	if err := store.SaveDriverSession(context.Background(), domain.DriverSession{
		ID:        "active",
		DriverID:  "driver_1",
		State:     domain.DriverSessionStateActive,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("save active session: %v", err)
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

	session, err := svc.GetCurrentForDriver(context.Background(), "driver_1")
	if err != nil {
		t.Fatalf("GetCurrentForDriver() error = %v", err)
	}
	if session.ID != "active" {
		t.Fatalf("session.ID = %q", session.ID)
	}
}

func TestOfferListPendingForDriverReturnsPendingOffersOnly(t *testing.T) {
	store := memory.NewStore()
	now := time.Now().UTC()
	if err := store.SaveDriverSession(context.Background(), domain.DriverSession{
		ID:        "session_1",
		DriverID:  "driver_1",
		State:     domain.DriverSessionStateActive,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("save session: %v", err)
	}
	_ = store.SaveRideOffer(context.Background(), domain.RideOffer{
		ID:              "offer_pending",
		DriverSessionID: "session_1",
		State:           domain.RideOfferStatePending,
		CreatedAt:       now,
	})
	_ = store.SaveRideOffer(context.Background(), domain.RideOffer{
		ID:              "offer_declined",
		DriverSessionID: "session_1",
		State:           domain.RideOfferStateDeclined,
		CreatedAt:       now.Add(time.Second),
	})

	svc := NewOfferService(
		memory.RideOffers{Store: store},
		memory.TripDemands{Store: store},
		memory.DriverSessions{Store: store},
		memory.RideBookings{Store: store},
		memory.Vehicles{Store: store},
		nil,
		nil,
		nil,
		nil,
		nil,
		&foundation.AtomicIDGenerator{},
	)

	offers, err := svc.ListPendingForDriver(context.Background(), "driver_1")
	if err != nil {
		t.Fatalf("ListPendingForDriver() error = %v", err)
	}
	if len(offers) != 1 || offers[0].ID != "offer_pending" {
		t.Fatalf("offers = %#v", offers)
	}
}

func TestBookingListActiveForActorExcludesTerminalStates(t *testing.T) {
	store := memory.NewStore()
	now := time.Now().UTC()
	_ = store.SaveRideBooking(context.Background(), domain.RideBooking{
		ID:        "active_booking",
		RiderID:   "user_1",
		DriverID:  "driver_1",
		State:     domain.RideBookingStateAssigned,
		UpdatedAt: now,
	})
	_ = store.SaveRideBooking(context.Background(), domain.RideBooking{
		ID:        "completed_booking",
		RiderID:   "user_1",
		DriverID:  "driver_1",
		State:     domain.RideBookingStateCompleted,
		UpdatedAt: now.Add(time.Second),
	})

	svc := NewBookingService(
		memory.RideBookings{Store: store},
		memory.RideOffers{Store: store},
		memory.TripDemands{Store: store},
		memory.DriverSessions{Store: store},
		nil,
	)

	bookings, err := svc.ListActiveForActor(context.Background(), "user_1")
	if err != nil {
		t.Fatalf("ListActiveForActor() error = %v", err)
	}
	if len(bookings) != 1 || bookings[0].ID != "active_booking" {
		t.Fatalf("bookings = %#v", bookings)
	}
}
