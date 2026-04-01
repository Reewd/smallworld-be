package service

import (
	"context"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/ports"
)

type BookingService struct {
	bookings ports.RideBookingRepository
	offers   ports.RideOfferRepository
	demands  ports.TripDemandRepository
	sessions ports.DriverSessionRepository
	realtime ports.RealtimeHub
}

func NewBookingService(
	bookings ports.RideBookingRepository,
	offers ports.RideOfferRepository,
	demands ports.TripDemandRepository,
	sessions ports.DriverSessionRepository,
	realtime ports.RealtimeHub,
) *BookingService {
	return &BookingService{
		bookings: bookings,
		offers:   offers,
		demands:  demands,
		sessions: sessions,
		realtime: realtime,
	}
}

func (s *BookingService) GetDriverTrackingForRider(ctx context.Context, riderUserID string, bookingID string) (domain.DriverTracking, error) {
	booking, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return domain.DriverTracking{}, err
	}
	if booking.RiderID != riderUserID {
		return domain.DriverTracking{}, domain.ErrUnauthorized
	}
	if !booking.AllowsDriverTracking() {
		return domain.DriverTracking{}, domain.ErrDriverTrackingUnavailable
	}

	session, err := s.sessions.FindByID(ctx, booking.DriverSessionID)
	if err != nil {
		return domain.DriverTracking{}, err
	}
	if session.State == domain.DriverSessionStateEnded {
		return domain.DriverTracking{}, domain.ErrDriverTrackingUnavailable
	}

	return driverTrackingFromBookingAndSession(booking, session), nil
}

func (s *BookingService) Transition(ctx context.Context, bookingID string, next domain.RideBookingState) (domain.RideBooking, error) {
	booking, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return domain.RideBooking{}, err
	}
	if err := domain.RequireTransition("ride_booking", booking.State, booking.CanTransitionTo(next), next); err != nil {
		return domain.RideBooking{}, err
	}
	booking.State = next
	booking.UpdatedAt = time.Now().UTC()
	if err := s.bookings.Save(ctx, booking); err != nil {
		return domain.RideBooking{}, err
	}
	_ = s.realtime.PublishToUser(ctx, booking.RiderID, "ride_booking."+string(next), booking)
	_ = s.realtime.PublishToUser(ctx, booking.DriverID, "ride_booking."+string(next), booking)
	return booking, nil
}

func (s *BookingService) TransitionForActor(ctx context.Context, actorUserID string, bookingID string, next domain.RideBookingState) (domain.RideBooking, error) {
	booking, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return domain.RideBooking{}, err
	}
	if booking.RiderID != actorUserID && booking.DriverID != actorUserID {
		return domain.RideBooking{}, domain.ErrUnauthorized
	}
	if err := domain.RequireTransition("ride_booking", booking.State, booking.CanTransitionTo(next), next); err != nil {
		return domain.RideBooking{}, err
	}
	booking.State = next
	booking.UpdatedAt = time.Now().UTC()
	if err := s.bookings.Save(ctx, booking); err != nil {
		return domain.RideBooking{}, err
	}
	_ = s.realtime.PublishToUser(ctx, booking.RiderID, "ride_booking."+string(next), booking)
	_ = s.realtime.PublishToUser(ctx, booking.DriverID, "ride_booking."+string(next), booking)
	return booking, nil
}

func (s *BookingService) GetForActor(ctx context.Context, actorUserID string, bookingID string) (domain.RideBooking, error) {
	booking, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return domain.RideBooking{}, err
	}
	if booking.RiderID != actorUserID && booking.DriverID != actorUserID {
		return domain.RideBooking{}, domain.ErrUnauthorized
	}
	return booking, nil
}

func (s *BookingService) ListActiveForActor(ctx context.Context, actorUserID string) ([]domain.RideBooking, error) {
	bookings, err := s.bookings.ListActiveByActorID(ctx, actorUserID)
	if err != nil {
		return nil, err
	}
	if len(bookings) == 0 {
		return nil, domain.ErrBookingNotFound
	}
	return bookings, nil
}

func driverTrackingFromBookingAndSession(booking domain.RideBooking, session domain.DriverSession) domain.DriverTracking {
	return domain.DriverTracking{
		BookingID:       booking.ID,
		DriverSessionID: session.ID,
		BookingState:    booking.State,
		SessionState:    session.State,
		CurrentLocation: session.CurrentLocation,
		RoutePolyline:   session.RoutePolyline,
		LastHeartbeatAt: session.LastHeartbeatAt,
		UpdatedAt:       session.UpdatedAt,
	}
}
