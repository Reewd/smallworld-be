package service

import (
	"context"
	"sync"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/foundation"
	"smallworld/internal/ports"
)

type OfferService struct {
	offers    ports.RideOfferRepository
	demands   ports.TripDemandRepository
	sessions  ports.DriverSessionRepository
	bookings  ports.RideBookingRepository
	vehicles  ports.VehicleRepository
	pricing   ports.PricingService
	push      ports.PushNotifier
	realtime  ports.RealtimeHub
	acceptor  ports.OfferAcceptor
	ephemeral ports.EphemeralOfferStore
	idg       foundation.IDGenerator
	mu        sync.Mutex
}

func NewOfferService(
	offers ports.RideOfferRepository,
	demands ports.TripDemandRepository,
	sessions ports.DriverSessionRepository,
	bookings ports.RideBookingRepository,
	vehicles ports.VehicleRepository,
	pricing ports.PricingService,
	push ports.PushNotifier,
	realtime ports.RealtimeHub,
	acceptor ports.OfferAcceptor,
	ephemeral ports.EphemeralOfferStore,
	idg foundation.IDGenerator,
) *OfferService {
	return &OfferService{
		offers:    offers,
		demands:   demands,
		sessions:  sessions,
		bookings:  bookings,
		vehicles:  vehicles,
		pricing:   pricing,
		push:      push,
		realtime:  realtime,
		acceptor:  acceptor,
		ephemeral: ephemeral,
		idg:       idg,
	}
}

func (s *OfferService) Accept(ctx context.Context, actorUserID string, offerID string) (domain.RideBooking, error) {
	if s.acceptor != nil {
		offer, err := s.offers.FindByID(ctx, offerID)
		if err != nil {
			return domain.RideBooking{}, err
		}
		session, err := s.sessions.FindByID(ctx, offer.DriverSessionID)
		if err != nil {
			return domain.RideBooking{}, err
		}
		if session.DriverID != actorUserID {
			return domain.RideBooking{}, domain.ErrUnauthorized
		}

		booking, err := s.acceptor.AcceptOffer(ctx, offerID, s.idg.New("booking"), time.Now().UTC())
		if err != nil {
			return domain.RideBooking{}, err
		}
		if s.ephemeral != nil {
			_ = s.ephemeral.DeletePendingOffer(ctx, offerID)
		}
		_ = s.push.SendRiderMatched(ctx, booking.RiderID, booking)
		_ = s.realtime.PublishToUser(ctx, booking.RiderID, "ride_booking.assigned", booking)
		return booking, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	offer, err := s.offers.FindByID(ctx, offerID)
	if err != nil {
		return domain.RideBooking{}, err
	}
	if err := domain.RequireTransition("ride_offer", offer.State, offer.CanTransitionTo(domain.RideOfferStateAccepted), domain.RideOfferStateAccepted); err != nil {
		return domain.RideBooking{}, err
	}

	session, err := s.sessions.FindByID(ctx, offer.DriverSessionID)
	if err != nil {
		return domain.RideBooking{}, err
	}
	if session.RemainingCapacity <= 0 {
		return domain.RideBooking{}, domain.ErrCapacityExceeded
	}
	if session.DriverID != actorUserID {
		return domain.RideBooking{}, domain.ErrUnauthorized
	}

	demand, err := s.demands.FindByID(ctx, offer.DemandID)
	if err != nil {
		return domain.RideBooking{}, err
	}
	vehicle, err := s.vehicles.FindByID(ctx, session.VehicleID)
	if err != nil {
		return domain.RideBooking{}, err
	}

	offer.State = domain.RideOfferStateAccepted
	offer.UpdatedAt = time.Now().UTC()
	if err := s.offers.Save(ctx, offer); err != nil {
		return domain.RideBooking{}, err
	}

	booking := domain.RideBooking{
		ID:                  s.idg.New("booking"),
		DemandID:            demand.ID,
		DriverSessionID:     session.ID,
		RiderID:             demand.RiderID,
		DriverID:            session.DriverID,
		State:               domain.RideBookingStateAssigned,
		MatchedPickup:       derefLocation(demand.MatchedPickup, demand.RequestedOrigin),
		MatchedDropoff:      derefLocation(demand.MatchedDropoff, demand.RequestedDestination),
		QuotedFareCents:     offer.FareCents,
		VehicleLicensePlate: vehicle.LicensePlate,
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}
	if err := s.bookings.Save(ctx, booking); err != nil {
		return domain.RideBooking{}, err
	}

	session.RemainingCapacity--
	session.UpdatedAt = time.Now().UTC()
	if session.RemainingCapacity <= 0 {
		session.State = domain.DriverSessionStateFull
	}
	if err := s.sessions.Save(ctx, session); err != nil {
		return domain.RideBooking{}, err
	}

	demand.State = domain.TripDemandStateMatched
	demand.UpdatedAt = time.Now().UTC()
	if err := s.demands.Save(ctx, demand); err != nil {
		return domain.RideBooking{}, err
	}

	_ = s.push.SendRiderMatched(ctx, demand.RiderID, booking)
	_ = s.realtime.PublishToUser(ctx, demand.RiderID, "ride_booking.assigned", booking)
	if s.ephemeral != nil {
		_ = s.ephemeral.DeletePendingOffer(ctx, offerID)
	}
	return booking, nil
}

func (s *OfferService) Decline(ctx context.Context, actorUserID string, offerID string) (domain.RideOffer, error) {
	offer, err := s.offers.FindByID(ctx, offerID)
	if err != nil {
		return domain.RideOffer{}, err
	}
	session, err := s.sessions.FindByID(ctx, offer.DriverSessionID)
	if err != nil {
		return domain.RideOffer{}, err
	}
	if session.DriverID != actorUserID {
		return domain.RideOffer{}, domain.ErrUnauthorized
	}
	if err := domain.RequireTransition("ride_offer", offer.State, offer.CanTransitionTo(domain.RideOfferStateDeclined), domain.RideOfferStateDeclined); err != nil {
		return domain.RideOffer{}, err
	}
	offer.State = domain.RideOfferStateDeclined
	offer.UpdatedAt = time.Now().UTC()
	if err := s.offers.Save(ctx, offer); err != nil {
		return domain.RideOffer{}, err
	}
	if s.ephemeral != nil {
		_ = s.ephemeral.DeletePendingOffer(ctx, offerID)
	}
	return offer, nil
}

func derefLocation(location *domain.Location, fallback domain.Location) domain.Location {
	if location == nil {
		return fallback
	}
	return *location
}
