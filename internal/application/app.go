package application

import (
	"smallworld/internal/application/service"
	"smallworld/internal/foundation"
	"smallworld/internal/matching"
	"smallworld/internal/ports"
)

type Services struct {
	Profile       *service.ProfileService
	DevBootstrap  *service.DevBootstrapService
	Vehicle       *service.VehicleService
	DriverSession *service.DriverSessionService
	TripDemand    *service.TripDemandService
	Offer         *service.OfferService
	Booking       *service.BookingService
	Review        *service.ReviewService
}

type Dependencies struct {
	Users           ports.UserRepository
	Verifications   ports.VerificationRepository
	Vehicles        ports.VehicleRepository
	Sessions        ports.DriverSessionRepository
	Demands         ports.TripDemandRepository
	Offers          ports.RideOfferRepository
	Bookings        ports.RideBookingRepository
	Reviews         ports.ReviewRepository
	Idempotency     ports.IdempotencyRepository
	OfferAcceptor   ports.OfferAcceptor
	DriverPresence  ports.DriverPresenceStore
	EphemeralOffers ports.EphemeralOfferStore
	Routing         ports.RoutingProvider
	Pricing         ports.PricingService
	Push            ports.PushNotifier
	Realtime        ports.RealtimeHub
	IDGen           foundation.IDGenerator
	Matching        *matching.Engine
}

func NewServices(deps Dependencies) Services {
	return Services{
		Profile:       service.NewProfileService(deps.Users, deps.IDGen),
		DevBootstrap:  service.NewDevBootstrapService(deps.Users, deps.Verifications, deps.Vehicles, deps.IDGen),
		Vehicle:       service.NewVehicleService(deps.Vehicles, deps.IDGen),
		DriverSession: service.NewDriverSessionService(deps.Sessions, deps.Verifications, deps.Vehicles, deps.Routing, deps.Idempotency, deps.DriverPresence, deps.IDGen),
		TripDemand:    service.NewTripDemandService(deps.Demands, deps.Verifications, deps.Idempotency, deps.Sessions, deps.Offers, deps.Bookings, deps.Vehicles, deps.Routing, deps.Pricing, deps.Push, deps.Realtime, deps.DriverPresence, deps.EphemeralOffers, deps.IDGen, deps.Matching),
		Offer:         service.NewOfferService(deps.Offers, deps.Demands, deps.Sessions, deps.Bookings, deps.Vehicles, deps.Pricing, deps.Push, deps.Realtime, deps.OfferAcceptor, deps.EphemeralOffers, deps.IDGen),
		Booking:       service.NewBookingService(deps.Bookings, deps.Offers, deps.Demands, deps.Sessions, deps.Realtime),
		Review:        service.NewReviewService(deps.Reviews, deps.Bookings, deps.IDGen),
	}
}
