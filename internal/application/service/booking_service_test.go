package service

import (
	"context"
	"testing"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/infrastructure/memory"
)

func TestGetDriverTrackingForRiderReturnsBookingScopedSnapshot(t *testing.T) {
	store := memory.NewStore()
	now := time.Now().UTC()

	if err := store.SaveDriverSession(context.Background(), domain.DriverSession{
		ID:              "ds_1",
		DriverID:        "driver_1",
		State:           domain.DriverSessionStateActive,
		RoutePolyline:   "encoded-polyline",
		CurrentLocation: domain.Location{Lat: 45.1, Lng: 9.1},
		LastHeartbeatAt: now,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatalf("save session: %v", err)
	}
	if err := store.SaveRideBooking(context.Background(), domain.RideBooking{
		ID:              "booking_1",
		DriverSessionID: "ds_1",
		RiderID:         "rider_1",
		DriverID:        "driver_1",
		State:           domain.RideBookingStateDriverEnRouteToPickup,
		CreatedAt:       now,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatalf("save booking: %v", err)
	}

	svc := NewBookingService(
		memory.RideBookings{Store: store},
		memory.RideOffers{Store: store},
		memory.TripDemands{Store: store},
		memory.DriverSessions{Store: store},
		nil,
	)

	tracking, err := svc.GetDriverTrackingForRider(context.Background(), "rider_1", "booking_1")
	if err != nil {
		t.Fatalf("GetDriverTrackingForRider() error = %v", err)
	}
	if tracking.BookingID != "booking_1" || tracking.DriverSessionID != "ds_1" {
		t.Fatalf("tracking ids = %#v", tracking)
	}
	if tracking.RoutePolyline != "encoded-polyline" {
		t.Fatalf("tracking.RoutePolyline = %q", tracking.RoutePolyline)
	}
}

func TestGetDriverTrackingForRiderRejectsNonMatchedUserAndTerminalBookings(t *testing.T) {
	store := memory.NewStore()
	now := time.Now().UTC()

	if err := store.SaveDriverSession(context.Background(), domain.DriverSession{
		ID:              "ds_1",
		DriverID:        "driver_1",
		State:           domain.DriverSessionStateActive,
		CurrentLocation: domain.Location{Lat: 45.1, Lng: 9.1},
		LastHeartbeatAt: now,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatalf("save session: %v", err)
	}
	if err := store.SaveRideBooking(context.Background(), domain.RideBooking{
		ID:              "booking_1",
		DriverSessionID: "ds_1",
		RiderID:         "rider_1",
		DriverID:        "driver_1",
		State:           domain.RideBookingStateCompleted,
		CreatedAt:       now,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatalf("save booking: %v", err)
	}

	svc := NewBookingService(
		memory.RideBookings{Store: store},
		memory.RideOffers{Store: store},
		memory.TripDemands{Store: store},
		memory.DriverSessions{Store: store},
		nil,
	)

	if _, err := svc.GetDriverTrackingForRider(context.Background(), "rider_2", "booking_1"); err != domain.ErrUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
	if _, err := svc.GetDriverTrackingForRider(context.Background(), "rider_1", "booking_1"); err != domain.ErrDriverTrackingUnavailable {
		t.Fatalf("expected unavailable, got %v", err)
	}
}
