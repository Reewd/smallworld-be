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
		memory.RideBookings{Store: store},
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
		memory.RideBookings{Store: store},
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

func TestHeartbeatPublishesDriverTrackingOnlyToActiveMatchedRiders(t *testing.T) {
	store := memory.NewStore()
	now := time.Now().UTC()

	if err := store.SaveDriverSession(context.Background(), domain.DriverSession{
		ID:              "ds_1",
		DriverID:        "driver_1",
		State:           domain.DriverSessionStateActive,
		RoutePolyline:   "encoded-polyline",
		CurrentLocation: domain.Location{Lat: 45, Lng: 9},
		LastHeartbeatAt: now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatalf("save session: %v", err)
	}
	if err := store.SaveRideBooking(context.Background(), domain.RideBooking{
		ID:              "booking_active",
		DriverSessionID: "ds_1",
		RiderID:         "rider_1",
		DriverID:        "driver_1",
		State:           domain.RideBookingStateAssigned,
		CreatedAt:       now,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatalf("save active booking: %v", err)
	}
	if err := store.SaveRideBooking(context.Background(), domain.RideBooking{
		ID:              "booking_completed",
		DriverSessionID: "ds_1",
		RiderID:         "rider_2",
		DriverID:        "driver_1",
		State:           domain.RideBookingStateCompleted,
		CreatedAt:       now,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatalf("save completed booking: %v", err)
	}

	realtime := &recordingRealtimeHub{}
	svc := NewDriverSessionService(
		memory.DriverSessions{Store: store},
		memory.Verifications{Store: store},
		memory.Vehicles{Store: store},
		stubRoutingProvider{},
		memory.Idempotency{Store: store},
		nil,
		memory.RideBookings{Store: store},
		realtime,
		&foundation.AtomicIDGenerator{},
	)

	session, err := svc.Heartbeat(context.Background(), HeartbeatDriverSessionInput{
		SessionID:       "ds_1",
		CurrentLocation: domain.Location{Lat: 45.001, Lng: 9.001},
	})
	if err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}
	if len(realtime.events) != 1 {
		t.Fatalf("events = %#v", realtime.events)
	}
	event := realtime.events[0]
	if event.userID != "rider_1" || event.eventType != "driver_tracking.updated" {
		t.Fatalf("event = %#v", event)
	}
	tracking, ok := event.payload.(domain.DriverTracking)
	if !ok {
		t.Fatalf("payload type = %T", event.payload)
	}
	if tracking.BookingID != "booking_active" || tracking.DriverSessionID != "ds_1" {
		t.Fatalf("tracking ids = %#v", tracking)
	}
	if tracking.CurrentLocation != session.CurrentLocation {
		t.Fatalf("tracking.CurrentLocation = %#v session.CurrentLocation = %#v", tracking.CurrentLocation, session.CurrentLocation)
	}
}

type recordedRealtimeEvent struct {
	userID    string
	eventType string
	payload   any
}

type recordingRealtimeHub struct {
	events []recordedRealtimeEvent
}

func (h *recordingRealtimeHub) PublishToUser(_ context.Context, userID string, eventType string, payload any) error {
	h.events = append(h.events, recordedRealtimeEvent{
		userID:    userID,
		eventType: eventType,
		payload:   payload,
	})
	return nil
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
