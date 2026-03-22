package offers

import (
	"context"
	"errors"
	"log"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/ports"
)

type Config struct {
	PollInterval              time.Duration
	PendingOfferTTL           time.Duration
	MaxDriverSessionStaleness time.Duration
}

type Sweeper struct {
	offers    ports.RideOfferRepository
	demands   ports.TripDemandRepository
	sessions  ports.DriverSessionRepository
	realtime  ports.RealtimeHub
	ephemeral ports.EphemeralOfferStore
	config    Config
	logger    *log.Logger
}

func NewSweeper(
	offers ports.RideOfferRepository,
	demands ports.TripDemandRepository,
	sessions ports.DriverSessionRepository,
	realtime ports.RealtimeHub,
	ephemeral ports.EphemeralOfferStore,
	config Config,
	logger *log.Logger,
) *Sweeper {
	if config.PollInterval <= 0 {
		config.PollInterval = 5 * time.Second
	}
	if config.PendingOfferTTL <= 0 {
		config.PendingOfferTTL = 2 * time.Minute
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Sweeper{
		offers:    offers,
		demands:   demands,
		sessions:  sessions,
		realtime:  realtime,
		ephemeral: ephemeral,
		config:    config,
		logger:    logger,
	}
}

func (s *Sweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	for {
		if err := s.SweepOnce(ctx); err != nil {
			s.logger.Printf("offer sweeper error: %v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Sweeper) SweepOnce(ctx context.Context) error {
	pendingOffers, err := s.offers.ListPending(ctx)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, offer := range pendingOffers {
		nextState, demand, session, ok := s.classifyPendingOffer(ctx, offer, now)
		if !ok {
			continue
		}

		updatedOffer, err := s.offers.TransitionPending(ctx, offer.ID, nextState, now)
		if err != nil {
			// Another process may have accepted or already transitioned the offer.
			continue
		}

		if s.ephemeral != nil {
			_ = s.ephemeral.DeletePendingOffer(ctx, updatedOffer.ID)
		}
		if s.realtime != nil && session.DriverID != "" {
			_ = s.realtime.PublishToUser(ctx, session.DriverID, "ride_offer."+string(updatedOffer.State), updatedOffer)
		}

		if demand.ID != "" && demand.State == domain.TripDemandStateOffered {
			demand.State = domain.TripDemandStateSearching
			demand.UpdatedAt = now
			_ = s.demands.Save(ctx, demand)
			if s.realtime != nil {
				_ = s.realtime.PublishToUser(ctx, demand.RiderID, "trip_demand.searching", demand)
			}
		}
	}

	return nil
}

func (s *Sweeper) classifyPendingOffer(ctx context.Context, offer domain.RideOffer, now time.Time) (domain.RideOfferState, domain.TripDemand, domain.DriverSession, bool) {
	if now.Sub(offer.CreatedAt) >= s.config.PendingOfferTTL {
		demand, _ := s.demands.FindByID(ctx, offer.DemandID)
		session, _ := s.sessions.FindByID(ctx, offer.DriverSessionID)
		return domain.RideOfferStateExpired, demand, session, true
	}

	demand, err := s.demands.FindByID(ctx, offer.DemandID)
	if err != nil {
		session, _ := s.sessions.FindByID(ctx, offer.DriverSessionID)
		return domain.RideOfferStateWithdrawn, domain.TripDemand{}, session, true
	}
	if demand.State == domain.TripDemandStateCanceled || demand.State == domain.TripDemandStateAborted || demand.State == domain.TripDemandStateMatched {
		session, _ := s.sessions.FindByID(ctx, offer.DriverSessionID)
		return domain.RideOfferStateWithdrawn, demand, session, true
	}

	session, err := s.sessions.FindByID(ctx, offer.DriverSessionID)
	if err != nil {
		return domain.RideOfferStateWithdrawn, demand, domain.DriverSession{}, true
	}
	if session.State == domain.DriverSessionStatePaused || session.State == domain.DriverSessionStateEnded {
		return domain.RideOfferStateWithdrawn, demand, session, true
	}
	if s.config.MaxDriverSessionStaleness > 0 && now.Sub(session.LastHeartbeatAt) > s.config.MaxDriverSessionStaleness {
		return domain.RideOfferStateWithdrawn, demand, session, true
	}

	return "", domain.TripDemand{}, domain.DriverSession{}, false
}

func IgnoreNotFound(err error) bool {
	return err == nil || errors.Is(err, domain.ErrOfferNotFound) || errors.Is(err, domain.ErrDemandNotFound)
}
