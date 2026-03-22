package domain

type TripDemandState string

const (
	TripDemandStateDraft     TripDemandState = "draft"
	TripDemandStateSearching TripDemandState = "searching"
	TripDemandStateOffered   TripDemandState = "offered"
	TripDemandStateMatched   TripDemandState = "matched"
	TripDemandStateCanceled  TripDemandState = "canceled"
	TripDemandStateAborted   TripDemandState = "aborted"
)

type RideOfferState string

const (
	RideOfferStatePending   RideOfferState = "pending"
	RideOfferStateAccepted  RideOfferState = "accepted"
	RideOfferStateDeclined  RideOfferState = "declined"
	RideOfferStateExpired   RideOfferState = "expired"
	RideOfferStateWithdrawn RideOfferState = "withdrawn"
)

type RideBookingState string

const (
	RideBookingStateAssigned              RideBookingState = "assigned"
	RideBookingStateRiderWalkingToPickup  RideBookingState = "rider_walking_to_pickup"
	RideBookingStateDriverEnRouteToPickup RideBookingState = "driver_en_route_to_pickup"
	RideBookingStatePickupReady           RideBookingState = "pickup_ready"
	RideBookingStateOnboard               RideBookingState = "onboard"
	RideBookingStateCompleted             RideBookingState = "completed"
	RideBookingStateCanceled              RideBookingState = "canceled"
	RideBookingStateNoShow                RideBookingState = "no_show"
)

type DriverSessionState string

const (
	DriverSessionStateActive DriverSessionState = "active"
	DriverSessionStateFull   DriverSessionState = "full"
	DriverSessionStatePaused DriverSessionState = "paused"
	DriverSessionStateEnded  DriverSessionState = "ended"
)
