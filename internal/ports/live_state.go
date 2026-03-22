package ports

import (
	"context"
	"time"

	"smallworld/internal/domain"
)

type OfferAcceptor interface {
	AcceptOffer(ctx context.Context, offerID string, bookingID string, now time.Time) (domain.RideBooking, error)
}

type DriverPresenceStore interface {
	SaveSession(ctx context.Context, session domain.DriverSession) error
	DeleteSession(ctx context.Context, sessionID string) error
	ListActiveSessions(ctx context.Context) ([]domain.DriverSession, error)
}

type EphemeralOfferStore interface {
	SavePendingOffer(ctx context.Context, offer domain.RideOffer) error
	DeletePendingOffer(ctx context.Context, offerID string) error
}
