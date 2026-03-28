package domain

import "errors"

var (
	ErrUnauthorized                         = errors.New("unauthorized")
	ErrUserNotFound                         = errors.New("user not found")
	ErrDriverSessionNotFound                = errors.New("driver session not found")
	ErrVerificationRequired                 = errors.New("verification required")
	ErrVehicleRequired                      = errors.New("active vehicle required")
	ErrDemandNotFound                       = errors.New("trip demand not found")
	ErrNoCandidateFound                     = errors.New("no matching driver candidate found")
	ErrOfferNotFound                        = errors.New("ride offer not found")
	ErrBookingNotFound                      = errors.New("ride booking not found")
	ErrInvalidUserPreferences               = errors.New("invalid user preferences")
	ErrCapacityExceeded                     = errors.New("capacity exceeded")
	ErrIdempotencyConflict                  = errors.New("idempotency conflict")
	ErrWomenOnlyRequiresVerifiedFemaleRider = errors.New("women-only matching is unavailable for this account")
)
