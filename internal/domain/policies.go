package domain

import "fmt"

func (v IdentityVerification) IsVerified() bool {
	return v.Status == VerificationVerified
}

func RiderEligible(v IdentityVerification) bool {
	return v.IsVerified()
}

func DriverEligible(v IdentityVerification, vehicles []Vehicle) bool {
	if !v.IsVerified() {
		return false
	}

	for _, vehicle := range vehicles {
		if vehicle.IsActive && vehicle.Capacity > 0 {
			return true
		}
	}

	return false
}

func CanDriverServeDemand(session DriverSession, verification IdentityVerification, demand TripDemand) bool {
	if session.State != DriverSessionStateActive && session.State != DriverSessionStateFull {
		return false
	}
	if session.RemainingCapacity <= 0 {
		return false
	}
	if demand.WomenDriversOnly && verification.VerifiedGender != GenderFemale {
		return false
	}
	return true
}

func (d TripDemand) CanTransitionTo(next TripDemandState) bool {
	switch d.State {
	case TripDemandStateDraft:
		return next == TripDemandStateSearching || next == TripDemandStateCanceled
	case TripDemandStateSearching:
		return next == TripDemandStateOffered || next == TripDemandStateMatched || next == TripDemandStateCanceled || next == TripDemandStateAborted
	case TripDemandStateOffered:
		return next == TripDemandStateSearching || next == TripDemandStateMatched || next == TripDemandStateCanceled || next == TripDemandStateAborted
	case TripDemandStateMatched, TripDemandStateCanceled, TripDemandStateAborted:
		return false
	default:
		return false
	}
}

func (o RideOffer) CanTransitionTo(next RideOfferState) bool {
	switch o.State {
	case RideOfferStatePending:
		return next == RideOfferStateAccepted || next == RideOfferStateDeclined || next == RideOfferStateExpired || next == RideOfferStateWithdrawn
	case RideOfferStateAccepted, RideOfferStateDeclined, RideOfferStateExpired, RideOfferStateWithdrawn:
		return false
	default:
		return false
	}
}

func (b RideBooking) CanTransitionTo(next RideBookingState) bool {
	switch b.State {
	case RideBookingStateAssigned:
		return next == RideBookingStateRiderWalkingToPickup || next == RideBookingStateDriverEnRouteToPickup || next == RideBookingStateCanceled
	case RideBookingStateRiderWalkingToPickup:
		return next == RideBookingStateDriverEnRouteToPickup || next == RideBookingStatePickupReady || next == RideBookingStateCanceled || next == RideBookingStateNoShow
	case RideBookingStateDriverEnRouteToPickup:
		return next == RideBookingStatePickupReady || next == RideBookingStateCanceled || next == RideBookingStateNoShow
	case RideBookingStatePickupReady:
		return next == RideBookingStateOnboard || next == RideBookingStateCanceled || next == RideBookingStateNoShow
	case RideBookingStateOnboard:
		return next == RideBookingStateCompleted || next == RideBookingStateCanceled
	case RideBookingStateCompleted, RideBookingStateCanceled, RideBookingStateNoShow:
		return false
	default:
		return false
	}
}

func (s DriverSession) CanTransitionTo(next DriverSessionState) bool {
	switch s.State {
	case DriverSessionStateActive:
		return next == DriverSessionStateFull || next == DriverSessionStatePaused || next == DriverSessionStateEnded
	case DriverSessionStateFull:
		return next == DriverSessionStateActive || next == DriverSessionStatePaused || next == DriverSessionStateEnded
	case DriverSessionStatePaused:
		return next == DriverSessionStateActive || next == DriverSessionStateEnded
	case DriverSessionStateEnded:
		return false
	default:
		return false
	}
}

func RequireTransition[T ~string](entity string, from T, allowed bool, to T) error {
	if allowed {
		return nil
	}
	return fmt.Errorf("%s transition %s -> %s is not allowed", entity, from, to)
}
