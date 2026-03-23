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
