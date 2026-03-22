package ports

import (
	"context"

	"smallworld/internal/domain"
)

type RoutePlan struct {
	DistanceMeters  int    `json:"distance_meters"`
	DurationSeconds int    `json:"duration_seconds"`
	Polyline        string `json:"polyline,omitempty"`
}

type RoutingProvider interface {
	Route(context.Context, domain.Location, domain.Location) (RoutePlan, error)
	WalkingETASeconds(context.Context, domain.Location, domain.Location) (int, int, error)
	DrivingETASeconds(context.Context, domain.Location, domain.Location) (int, int, error)
}

type IdentityVerificationProvider interface {
	SyncStatus(context.Context, string) (domain.IdentityVerification, error)
}

type PricingService interface {
	Quote(context.Context, domain.MatchCandidate) (int, error)
}

type PushNotifier interface {
	SendDriverOffer(context.Context, string, domain.RideOffer) error
	SendRiderMatched(context.Context, string, domain.RideBooking) error
}

type RealtimeHub interface {
	PublishToUser(context.Context, string, string, any) error
}
