package service

import (
	"context"
	"encoding/json"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/foundation"
	"smallworld/internal/ports"
)

type DriverSessionService struct {
	sessions      ports.DriverSessionRepository
	verifications ports.VerificationRepository
	vehicles      ports.VehicleRepository
	routing       ports.RoutingProvider
	idempotency   ports.IdempotencyRepository
	presence      ports.DriverPresenceStore
	idg           foundation.IDGenerator
}

func NewDriverSessionService(
	sessions ports.DriverSessionRepository,
	verifications ports.VerificationRepository,
	vehicles ports.VehicleRepository,
	routing ports.RoutingProvider,
	idempotency ports.IdempotencyRepository,
	presence ports.DriverPresenceStore,
	idg foundation.IDGenerator,
) *DriverSessionService {
	return &DriverSessionService{
		sessions:      sessions,
		verifications: verifications,
		vehicles:      vehicles,
		routing:       routing,
		idempotency:   idempotency,
		presence:      presence,
		idg:           idg,
	}
}

type StartDriverSessionInput struct {
	UserID                      string          `json:"user_id"`
	VehicleID                   string          `json:"vehicle_id"`
	Origin                      domain.Location `json:"origin"`
	Destination                 domain.Location `json:"destination"`
	CurrentLocation             domain.Location `json:"current_location"`
	MaxDriverPickupDetourMeters int             `json:"max_driver_pickup_detour_meters"`
	IdempotencyKey              string          `json:"idempotency_key"`
}

type HeartbeatDriverSessionInput struct {
	SessionID       string          `json:"session_id"`
	CurrentLocation domain.Location `json:"current_location"`
}

type TransitionDriverSessionStateInput struct {
	SessionID string                    `json:"session_id"`
	State     domain.DriverSessionState `json:"state"`
}

func (s *DriverSessionService) Start(ctx context.Context, input StartDriverSessionInput) (domain.DriverSession, error) {
	payload, _ := json.Marshal(input)
	if input.IdempotencyKey != "" {
		if resourceID, payloadHash, err := s.idempotency.FindResult(ctx, input.IdempotencyKey); err == nil {
			if payloadHash == foundation.HashString(string(payload)) {
				return s.sessions.FindByID(ctx, resourceID)
			}
			return domain.DriverSession{}, domain.ErrIdempotencyConflict
		}
	}

	verification, err := s.verifications.FindByUserID(ctx, input.UserID)
	if err != nil {
		return domain.DriverSession{}, err
	}

	vehicles, err := s.vehicles.ListByUserID(ctx, input.UserID)
	if err != nil {
		return domain.DriverSession{}, err
	}

	if !domain.DriverEligible(verification, vehicles) {
		if !verification.IsVerified() {
			return domain.DriverSession{}, domain.ErrVerificationRequired
		}
		return domain.DriverSession{}, domain.ErrVehicleRequired
	}

	vehicle, err := s.vehicles.FindByID(ctx, input.VehicleID)
	if err != nil {
		return domain.DriverSession{}, err
	}

	route, err := s.routing.Route(ctx, input.Origin, input.Destination)
	if err != nil {
		return domain.DriverSession{}, err
	}

	session := domain.DriverSession{
		ID:                          s.idg.New("ds"),
		DriverID:                    input.UserID,
		VehicleID:                   input.VehicleID,
		State:                       domain.DriverSessionStateActive,
		Origin:                      input.Origin,
		Destination:                 input.Destination,
		CurrentLocation:             input.CurrentLocation,
		RemainingCapacity:           vehicle.Capacity,
		MaxDriverPickupDetourMeters: input.MaxDriverPickupDetourMeters,
		RouteDistanceMeters:         route.DistanceMeters,
		RouteDurationSeconds:        route.DurationSeconds,
		RoutePolyline:               route.Polyline,
		LastHeartbeatAt:             time.Now().UTC(),
		CreatedAt:                   time.Now().UTC(),
		UpdatedAt:                   time.Now().UTC(),
	}

	if err := s.sessions.Save(ctx, session); err != nil {
		return domain.DriverSession{}, err
	}

	if input.IdempotencyKey != "" {
		if err := s.idempotency.SaveResult(ctx, input.IdempotencyKey, session.ID, foundation.HashString(string(payload))); err != nil {
			return domain.DriverSession{}, err
		}
	}

	if s.presence != nil {
		if err := s.presence.SaveSession(ctx, session); err != nil {
			return domain.DriverSession{}, err
		}
	}

	return session, nil
}

func (s *DriverSessionService) Heartbeat(ctx context.Context, input HeartbeatDriverSessionInput) (domain.DriverSession, error) {
	session, err := s.sessions.FindByID(ctx, input.SessionID)
	if err != nil {
		return domain.DriverSession{}, err
	}

	session.CurrentLocation = input.CurrentLocation
	session.LastHeartbeatAt = time.Now().UTC()
	session.UpdatedAt = session.LastHeartbeatAt

	if err := s.sessions.Save(ctx, session); err != nil {
		return domain.DriverSession{}, err
	}
	if s.presence != nil {
		if err := s.presence.SaveSession(ctx, session); err != nil {
			return domain.DriverSession{}, err
		}
	}
	return session, nil
}

func (s *DriverSessionService) HeartbeatOwned(ctx context.Context, actorUserID string, input HeartbeatDriverSessionInput) (domain.DriverSession, error) {
	session, err := s.sessions.FindByID(ctx, input.SessionID)
	if err != nil {
		return domain.DriverSession{}, err
	}
	if session.DriverID != actorUserID {
		return domain.DriverSession{}, domain.ErrUnauthorized
	}
	return s.Heartbeat(ctx, input)
}

func (s *DriverSessionService) TransitionState(ctx context.Context, input TransitionDriverSessionStateInput) (domain.DriverSession, error) {
	session, err := s.sessions.FindByID(ctx, input.SessionID)
	if err != nil {
		return domain.DriverSession{}, err
	}
	if err := domain.RequireTransition("driver_session", session.State, session.CanTransitionTo(input.State), input.State); err != nil {
		return domain.DriverSession{}, err
	}

	session.State = input.State
	session.UpdatedAt = time.Now().UTC()
	if input.State == domain.DriverSessionStateActive && session.RemainingCapacity <= 0 {
		session.State = domain.DriverSessionStateFull
	}

	if err := s.sessions.Save(ctx, session); err != nil {
		return domain.DriverSession{}, err
	}
	if s.presence != nil {
		if session.State == domain.DriverSessionStateEnded {
			if err := s.presence.DeleteSession(ctx, session.ID); err != nil {
				return domain.DriverSession{}, err
			}
		} else {
			if err := s.presence.SaveSession(ctx, session); err != nil {
				return domain.DriverSession{}, err
			}
		}
	}
	return session, nil
}

func (s *DriverSessionService) TransitionStateOwned(ctx context.Context, actorUserID string, input TransitionDriverSessionStateInput) (domain.DriverSession, error) {
	session, err := s.sessions.FindByID(ctx, input.SessionID)
	if err != nil {
		return domain.DriverSession{}, err
	}
	if session.DriverID != actorUserID {
		return domain.DriverSession{}, domain.ErrUnauthorized
	}
	return s.TransitionState(ctx, input)
}

func (s *DriverSessionService) GetOwned(ctx context.Context, actorUserID string, sessionID string) (domain.DriverSession, error) {
	session, err := s.sessions.FindByID(ctx, sessionID)
	if err != nil {
		return domain.DriverSession{}, err
	}
	if session.DriverID != actorUserID {
		return domain.DriverSession{}, domain.ErrUnauthorized
	}
	return session, nil
}

func (s *DriverSessionService) ReconcileStaleSessions(ctx context.Context, maxStaleness time.Duration) ([]domain.DriverSession, error) {
	if maxStaleness <= 0 {
		return nil, nil
	}

	sessions, err := s.sessions.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().UTC().Add(-maxStaleness)
	var paused []domain.DriverSession
	for _, session := range sessions {
		if session.LastHeartbeatAt.Before(cutoff) {
			session.State = domain.DriverSessionStatePaused
			session.UpdatedAt = time.Now().UTC()
			if err := s.sessions.Save(ctx, session); err != nil {
				return nil, err
			}
			if s.presence != nil {
				if err := s.presence.DeleteSession(ctx, session.ID); err != nil {
					return nil, err
				}
			}
			paused = append(paused, session)
		}
	}
	return paused, nil
}
