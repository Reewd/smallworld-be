package domain

import "testing"

func TestWomenOnlyDemandRequiresVerifiedFemaleDriver(t *testing.T) {
	demand := TripDemand{WomenDriversOnly: true}
	session := DriverSession{State: DriverSessionStateActive, RemainingCapacity: 2}

	if CanDriverServeDemand(session, IdentityVerification{Status: VerificationVerified, VerifiedGender: GenderMale}, demand) {
		t.Fatalf("male driver should not be eligible for women-only demand")
	}

	if !CanDriverServeDemand(session, IdentityVerification{Status: VerificationVerified, VerifiedGender: GenderFemale}, demand) {
		t.Fatalf("female driver should be eligible for women-only demand")
	}
}

func TestRideBookingStateTransitions(t *testing.T) {
	booking := RideBooking{State: RideBookingStateAssigned}
	if !booking.CanTransitionTo(RideBookingStateRiderWalkingToPickup) {
		t.Fatalf("assigned booking should be able to move to rider_walking_to_pickup")
	}
	if booking.CanTransitionTo(RideBookingStateCompleted) {
		t.Fatalf("assigned booking should not be able to move directly to completed")
	}
}
