package memory

import (
	"context"
	"time"

	"smallworld/internal/domain"
)

type Users struct{ Store *Store }
type Verifications struct{ Store *Store }
type Vehicles struct{ Store *Store }
type DriverSessions struct{ Store *Store }
type TripDemands struct{ Store *Store }
type RideOffers struct{ Store *Store }
type RideBookings struct{ Store *Store }
type Reviews struct{ Store *Store }
type Idempotency struct{ Store *Store }

func (r Users) Save(ctx context.Context, user domain.User) error { return r.Store.Save(ctx, user) }
func (r Users) FindByID(ctx context.Context, id string) (domain.User, error) {
	return r.Store.FindByID(ctx, id)
}
func (r Users) FindByAuthSubject(ctx context.Context, authSubject string) (domain.User, error) {
	return r.Store.FindByAuthSubject(ctx, authSubject)
}
func (r Verifications) Save(ctx context.Context, verification domain.IdentityVerification) error {
	return r.Store.SaveVerification(ctx, verification)
}
func (r Verifications) FindByUserID(ctx context.Context, userID string) (domain.IdentityVerification, error) {
	return r.Store.FindByUserID(ctx, userID)
}
func (r Vehicles) Save(ctx context.Context, vehicle domain.Vehicle) error {
	return r.Store.SaveVehicle(ctx, vehicle)
}
func (r Vehicles) ListByUserID(ctx context.Context, userID string) ([]domain.Vehicle, error) {
	return r.Store.ListByUserID(ctx, userID)
}
func (r Vehicles) FindByID(ctx context.Context, id string) (domain.Vehicle, error) {
	return r.Store.FindVehicleByID(ctx, id)
}
func (r DriverSessions) Save(ctx context.Context, session domain.DriverSession) error {
	return r.Store.SaveDriverSession(ctx, session)
}
func (r DriverSessions) FindByID(ctx context.Context, id string) (domain.DriverSession, error) {
	return r.Store.FindDriverSessionByID(ctx, id)
}
func (r DriverSessions) ListActive(ctx context.Context) ([]domain.DriverSession, error) {
	return r.Store.ListActive(ctx)
}
func (r TripDemands) Save(ctx context.Context, demand domain.TripDemand) error {
	return r.Store.SaveTripDemand(ctx, demand)
}
func (r TripDemands) FindByID(ctx context.Context, id string) (domain.TripDemand, error) {
	return r.Store.FindTripDemandByID(ctx, id)
}
func (r TripDemands) FindActiveByRiderID(ctx context.Context, riderID string) (domain.TripDemand, error) {
	return r.Store.FindActiveByRiderID(ctx, riderID)
}
func (r RideOffers) Save(ctx context.Context, offer domain.RideOffer) error {
	return r.Store.SaveRideOffer(ctx, offer)
}
func (r RideOffers) FindByID(ctx context.Context, id string) (domain.RideOffer, error) {
	return r.Store.FindRideOfferByID(ctx, id)
}
func (r RideOffers) FindPendingByDemandID(ctx context.Context, demandID string) (domain.RideOffer, error) {
	return r.Store.FindPendingByDemandID(ctx, demandID)
}
func (r RideOffers) ListPending(ctx context.Context) ([]domain.RideOffer, error) {
	return r.Store.ListPendingRideOffers(ctx)
}
func (r RideOffers) TransitionPending(ctx context.Context, offerID string, next domain.RideOfferState, updatedAt time.Time) (domain.RideOffer, error) {
	return r.Store.TransitionPendingRideOffer(ctx, offerID, next, updatedAt)
}
func (r RideBookings) Save(ctx context.Context, booking domain.RideBooking) error {
	return r.Store.SaveRideBooking(ctx, booking)
}
func (r RideBookings) FindByID(ctx context.Context, id string) (domain.RideBooking, error) {
	return r.Store.FindRideBookingByID(ctx, id)
}
func (r RideBookings) ListByDriverSessionID(ctx context.Context, sessionID string) ([]domain.RideBooking, error) {
	return r.Store.ListByDriverSessionID(ctx, sessionID)
}
func (r Reviews) Save(ctx context.Context, review domain.Review) error {
	return r.Store.SaveReview(ctx, review)
}
func (r Reviews) ListBySubjectID(ctx context.Context, subjectID string) ([]domain.Review, error) {
	return r.Store.ListBySubjectID(ctx, subjectID)
}
func (r Idempotency) SaveResult(ctx context.Context, key string, resourceID string, payloadHash string) error {
	return r.Store.SaveResult(ctx, key, resourceID, payloadHash)
}
func (r Idempotency) FindResult(ctx context.Context, key string) (string, string, error) {
	return r.Store.FindResult(ctx, key)
}
