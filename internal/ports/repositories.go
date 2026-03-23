package ports

import (
	"context"
	"time"

	"smallworld/internal/domain"
)

type UserRepository interface {
	Save(context.Context, domain.User) error
	FindByID(context.Context, string) (domain.User, error)
	FindByAuthSubject(context.Context, string) (domain.User, error)
}

type VerificationRepository interface {
	Save(context.Context, domain.IdentityVerification) error
	FindByUserID(context.Context, string) (domain.IdentityVerification, error)
}

type VehicleRepository interface {
	Save(context.Context, domain.Vehicle) error
	ListByUserID(context.Context, string) ([]domain.Vehicle, error)
	FindByID(context.Context, string) (domain.Vehicle, error)
}

type DriverSessionRepository interface {
	Save(context.Context, domain.DriverSession) error
	FindByID(context.Context, string) (domain.DriverSession, error)
	ListActive(context.Context) ([]domain.DriverSession, error)
	FindCurrentByDriverID(context.Context, string) (domain.DriverSession, error)
}

type TripDemandRepository interface {
	Save(context.Context, domain.TripDemand) error
	FindByID(context.Context, string) (domain.TripDemand, error)
	FindActiveByRiderID(context.Context, string) (domain.TripDemand, error)
}

type RideOfferRepository interface {
	Save(context.Context, domain.RideOffer) error
	FindByID(context.Context, string) (domain.RideOffer, error)
	FindPendingByDemandID(context.Context, string) (domain.RideOffer, error)
	ListPending(context.Context) ([]domain.RideOffer, error)
	ListPendingByDriverID(context.Context, string) ([]domain.RideOffer, error)
	TransitionPending(context.Context, string, domain.RideOfferState, time.Time) (domain.RideOffer, error)
}

type RideBookingRepository interface {
	Save(context.Context, domain.RideBooking) error
	FindByID(context.Context, string) (domain.RideBooking, error)
	ListByDriverSessionID(context.Context, string) ([]domain.RideBooking, error)
	ListActiveByActorID(context.Context, string) ([]domain.RideBooking, error)
}

type ReviewRepository interface {
	Save(context.Context, domain.Review) error
	ListBySubjectID(context.Context, string) ([]domain.Review, error)
}

type IdempotencyRepository interface {
	SaveResult(context.Context, string, string, string) error
	FindResult(context.Context, string) (resourceID string, payloadHash string, err error)
}
