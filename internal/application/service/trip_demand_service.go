package service

import (
	"context"
	"encoding/json"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/foundation"
	"smallworld/internal/matching"
	"smallworld/internal/ports"
)

type TripDemandService struct {
	demands       ports.TripDemandRepository
	verifications ports.VerificationRepository
	sessions      ports.DriverSessionRepository
	offers        ports.RideOfferRepository
	bookings      ports.RideBookingRepository
	vehicles      ports.VehicleRepository
	routing       ports.RoutingProvider
	pricing       ports.PricingService
	push          ports.PushNotifier
	realtime      ports.RealtimeHub
	presence      ports.DriverPresenceStore
	ephemeral     ports.EphemeralOfferStore
	idempotency   ports.IdempotencyRepository
	idg           foundation.IDGenerator
	engine        *matching.Engine
}

func NewTripDemandService(
	demands ports.TripDemandRepository,
	verifications ports.VerificationRepository,
	idempotency ports.IdempotencyRepository,
	sessions ports.DriverSessionRepository,
	offers ports.RideOfferRepository,
	bookings ports.RideBookingRepository,
	vehicles ports.VehicleRepository,
	routing ports.RoutingProvider,
	pricing ports.PricingService,
	push ports.PushNotifier,
	realtime ports.RealtimeHub,
	presence ports.DriverPresenceStore,
	ephemeral ports.EphemeralOfferStore,
	idg foundation.IDGenerator,
	engine *matching.Engine,
) *TripDemandService {
	return &TripDemandService{
		demands:       demands,
		verifications: verifications,
		idempotency:   idempotency,
		sessions:      sessions,
		offers:        offers,
		bookings:      bookings,
		vehicles:      vehicles,
		routing:       routing,
		pricing:       pricing,
		push:          push,
		realtime:      realtime,
		presence:      presence,
		ephemeral:     ephemeral,
		idg:           idg,
		engine:        engine,
	}
}

type CreateTripDemandInput struct {
	RiderID                  string          `json:"rider_id"`
	RequestedOrigin          domain.Location `json:"requested_origin"`
	RequestedDestination     domain.Location `json:"requested_destination"`
	WomenDriversOnly         bool            `json:"women_drivers_only"`
	MaxWalkToPickupMeters    int             `json:"max_walk_to_pickup_meters"`
	MaxWalkFromDropoffMeters int             `json:"max_walk_from_dropoff_meters"`
	IdempotencyKey           string          `json:"idempotency_key"`
}

func (s *TripDemandService) Create(ctx context.Context, input CreateTripDemandInput) (domain.TripDemand, *domain.RideOffer, error) {
	payload, _ := json.Marshal(input)
	if input.IdempotencyKey != "" {
		if resourceID, payloadHash, err := s.idempotency.FindResult(ctx, input.IdempotencyKey); err == nil {
			if payloadHash == foundation.HashString(string(payload)) {
				demand, err := s.demands.FindByID(ctx, resourceID)
				return demand, nil, err
			}
			return domain.TripDemand{}, nil, domain.ErrIdempotencyConflict
		}
	}

	verification, err := s.verifications.FindByUserID(ctx, input.RiderID)
	if err != nil {
		return domain.TripDemand{}, nil, err
	}
	if !domain.RiderEligible(verification) {
		return domain.TripDemand{}, nil, domain.ErrVerificationRequired
	}

	now := time.Now().UTC()
	demand := domain.TripDemand{
		ID:                       s.idg.New("td"),
		RiderID:                  input.RiderID,
		State:                    domain.TripDemandStateSearching,
		RequestedOrigin:          input.RequestedOrigin,
		RequestedDestination:     input.RequestedDestination,
		WomenDriversOnly:         input.WomenDriversOnly,
		MaxWalkToPickupMeters:    input.MaxWalkToPickupMeters,
		MaxWalkFromDropoffMeters: input.MaxWalkFromDropoffMeters,
		IdempotencyKey:           input.IdempotencyKey,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	if err := s.demands.Save(ctx, demand); err != nil {
		return domain.TripDemand{}, nil, err
	}

	if input.IdempotencyKey != "" {
		if err := s.idempotency.SaveResult(ctx, input.IdempotencyKey, demand.ID, foundation.HashString(string(payload))); err != nil {
			return domain.TripDemand{}, nil, err
		}
	}

	offer, err := s.runMatch(ctx, demand, verification)
	if err != nil && err != domain.ErrNoCandidateFound {
		return domain.TripDemand{}, nil, err
	}

	return demand, offer, nil
}

func (s *TripDemandService) Cancel(ctx context.Context, riderID string, demandID string) (domain.TripDemand, error) {
	demand, err := s.demands.FindByID(ctx, demandID)
	if err != nil {
		return domain.TripDemand{}, err
	}
	if demand.RiderID != riderID {
		return domain.TripDemand{}, domain.ErrUnauthorized
	}
	if err := domain.RequireTransition("trip_demand", demand.State, demand.CanTransitionTo(domain.TripDemandStateCanceled), domain.TripDemandStateCanceled); err != nil {
		return domain.TripDemand{}, err
	}
	demand.State = domain.TripDemandStateCanceled
	demand.UpdatedAt = time.Now().UTC()
	return demand, s.demands.Save(ctx, demand)
}

func (s *TripDemandService) GetForRider(ctx context.Context, riderID string, demandID string) (domain.TripDemand, error) {
	demand, err := s.demands.FindByID(ctx, demandID)
	if err != nil {
		return domain.TripDemand{}, err
	}
	if demand.RiderID != riderID {
		return domain.TripDemand{}, domain.ErrUnauthorized
	}
	return demand, nil
}

func (s *TripDemandService) GetCurrentForRider(ctx context.Context, riderID string) (domain.TripDemand, error) {
	return s.demands.FindActiveByRiderID(ctx, riderID)
}

func (s *TripDemandService) runMatch(ctx context.Context, demand domain.TripDemand, rider domain.IdentityVerification) (*domain.RideOffer, error) {
	sessions, err := s.listCandidateSessions(ctx)
	if err != nil {
		return nil, err
	}

	sessionVerifications := map[string]domain.IdentityVerification{}
	for _, session := range sessions {
		verification, err := s.verifications.FindByUserID(ctx, session.DriverID)
		if err != nil {
			continue
		}
		sessionVerifications[session.DriverID] = verification
	}

	candidates, err := s.engine.FindCandidates(ctx, demand, rider, sessionVerifications, sessions)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, domain.ErrNoCandidateFound
	}

	best := candidates[0]
	fareCents, err := s.pricing.Quote(ctx, best)
	if err != nil {
		return nil, err
	}

	offer := domain.RideOffer{
		ID:               s.idg.New("offer"),
		DemandID:         demand.ID,
		DriverSessionID:  best.Session.ID,
		State:            domain.RideOfferStatePending,
		DetourMeters:     best.DriverPickupDetourMeters,
		PickupETASeconds: best.DriverPickupETASeconds,
		FareCents:        fareCents,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	if err := s.offers.Save(ctx, offer); err != nil {
		return nil, err
	}

	demand.State = domain.TripDemandStateOffered
	demand.MatchedPickup = &best.Pickup
	demand.MatchedDropoff = &best.Dropoff
	demand.UpdatedAt = time.Now().UTC()
	if err := s.demands.Save(ctx, demand); err != nil {
		return nil, err
	}

	_ = s.push.SendDriverOffer(ctx, best.Session.DriverID, offer)
	_ = s.realtime.PublishToUser(ctx, best.Session.DriverID, "ride_offer.pending", offer)
	if s.ephemeral != nil {
		if err := s.ephemeral.SavePendingOffer(ctx, offer); err != nil {
			return nil, err
		}
	}

	return &offer, nil
}

func (s *TripDemandService) listCandidateSessions(ctx context.Context) ([]domain.DriverSession, error) {
	if s.presence != nil {
		sessions, err := s.presence.ListActiveSessions(ctx)
		if err == nil && len(sessions) > 0 {
			return sessions, nil
		}
	}
	return s.sessions.ListActive(ctx)
}
