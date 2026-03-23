package memory

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"smallworld/internal/domain"
)

type idempotencyRecord struct {
	resourceID  string
	payloadHash string
}

type Store struct {
	mu             sync.RWMutex
	users          map[string]domain.User
	verifications  map[string]domain.IdentityVerification
	vehicles       map[string]domain.Vehicle
	driverSessions map[string]domain.DriverSession
	tripDemands    map[string]domain.TripDemand
	rideOffers     map[string]domain.RideOffer
	rideBookings   map[string]domain.RideBooking
	reviews        map[string]domain.Review
	idempotency    map[string]idempotencyRecord
}

func NewStore() *Store {
	return &Store{
		users:          map[string]domain.User{},
		verifications:  map[string]domain.IdentityVerification{},
		vehicles:       map[string]domain.Vehicle{},
		driverSessions: map[string]domain.DriverSession{},
		tripDemands:    map[string]domain.TripDemand{},
		rideOffers:     map[string]domain.RideOffer{},
		rideBookings:   map[string]domain.RideBooking{},
		reviews:        map[string]domain.Review{},
		idempotency:    map[string]idempotencyRecord{},
	}
}

func (s *Store) Save(_ context.Context, user domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[user.ID] = user
	return nil
}

func (s *Store) FindByID(_ context.Context, id string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}
	return user, nil
}

func (s *Store) FindByAuthSubject(_ context.Context, authSubject string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, user := range s.users {
		if user.AuthSubject == authSubject {
			return user, nil
		}
	}
	return domain.User{}, domain.ErrUserNotFound
}

func (s *Store) SaveVerification(_ context.Context, verification domain.IdentityVerification) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.verifications[verification.UserID] = verification
	return nil
}

func (s *Store) FindByUserID(_ context.Context, userID string) (domain.IdentityVerification, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	verification, ok := s.verifications[userID]
	if !ok {
		return domain.IdentityVerification{}, domain.ErrVerificationRequired
	}
	return verification, nil
}

func (s *Store) SaveVehicle(_ context.Context, vehicle domain.Vehicle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vehicles[vehicle.ID] = vehicle
	return nil
}

func (s *Store) ListByUserID(_ context.Context, userID string) ([]domain.Vehicle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var vehicles []domain.Vehicle
	for _, vehicle := range s.vehicles {
		if vehicle.UserID == userID {
			vehicles = append(vehicles, vehicle)
		}
	}
	return vehicles, nil
}

func (s *Store) FindVehicleByID(_ context.Context, id string) (domain.Vehicle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	vehicle, ok := s.vehicles[id]
	if !ok {
		return domain.Vehicle{}, errors.New("vehicle not found")
	}
	return vehicle, nil
}

func (s *Store) SaveDriverSession(_ context.Context, session domain.DriverSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.driverSessions[session.ID] = session
	return nil
}

func (s *Store) FindDriverSessionByID(_ context.Context, id string) (domain.DriverSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.driverSessions[id]
	if !ok {
		return domain.DriverSession{}, domain.ErrDriverSessionNotFound
	}
	return session, nil
}

func (s *Store) ListActive(_ context.Context) ([]domain.DriverSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var sessions []domain.DriverSession
	for _, session := range s.driverSessions {
		if session.State == domain.DriverSessionStateActive || session.State == domain.DriverSessionStateFull {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

func (s *Store) FindCurrentDriverSessionByDriverID(_ context.Context, driverID string) (domain.DriverSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var candidates []domain.DriverSession
	for _, session := range s.driverSessions {
		if session.DriverID != driverID || session.State == domain.DriverSessionStateEnded {
			continue
		}
		candidates = append(candidates, session)
	}
	if len(candidates) == 0 {
		return domain.DriverSession{}, domain.ErrDriverSessionNotFound
	}

	sort.Slice(candidates, func(i, j int) bool {
		left := driverSessionStatePriority(candidates[i].State)
		right := driverSessionStatePriority(candidates[j].State)
		if left != right {
			return left < right
		}
		return candidates[i].UpdatedAt.After(candidates[j].UpdatedAt)
	})
	return candidates[0], nil
}

func (s *Store) SaveTripDemand(_ context.Context, demand domain.TripDemand) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tripDemands[demand.ID] = demand
	return nil
}

func (s *Store) FindTripDemandByID(_ context.Context, id string) (domain.TripDemand, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	demand, ok := s.tripDemands[id]
	if !ok {
		return domain.TripDemand{}, domain.ErrDemandNotFound
	}
	return demand, nil
}

func (s *Store) FindActiveByRiderID(_ context.Context, riderID string) (domain.TripDemand, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, demand := range s.tripDemands {
		if demand.RiderID == riderID && (demand.State == domain.TripDemandStateSearching || demand.State == domain.TripDemandStateOffered) {
			return demand, nil
		}
	}
	return domain.TripDemand{}, domain.ErrDemandNotFound
}

func (s *Store) SaveRideOffer(_ context.Context, offer domain.RideOffer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rideOffers[offer.ID] = offer
	return nil
}

func (s *Store) FindRideOfferByID(_ context.Context, id string) (domain.RideOffer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	offer, ok := s.rideOffers[id]
	if !ok {
		return domain.RideOffer{}, domain.ErrOfferNotFound
	}
	return offer, nil
}

func (s *Store) FindPendingByDemandID(_ context.Context, demandID string) (domain.RideOffer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, offer := range s.rideOffers {
		if offer.DemandID == demandID && offer.State == domain.RideOfferStatePending {
			return offer, nil
		}
	}
	return domain.RideOffer{}, domain.ErrOfferNotFound
}

func (s *Store) ListPendingRideOffers(_ context.Context) ([]domain.RideOffer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var offers []domain.RideOffer
	for _, offer := range s.rideOffers {
		if offer.State == domain.RideOfferStatePending {
			offers = append(offers, offer)
		}
	}
	return offers, nil
}

func (s *Store) ListPendingRideOffersByDriverID(_ context.Context, driverID string) ([]domain.RideOffer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionIDs := map[string]struct{}{}
	for _, session := range s.driverSessions {
		if session.DriverID == driverID {
			sessionIDs[session.ID] = struct{}{}
		}
	}

	var offers []domain.RideOffer
	for _, offer := range s.rideOffers {
		if offer.State != domain.RideOfferStatePending {
			continue
		}
		if _, ok := sessionIDs[offer.DriverSessionID]; ok {
			offers = append(offers, offer)
		}
	}
	sort.Slice(offers, func(i, j int) bool {
		return offers[i].CreatedAt.After(offers[j].CreatedAt)
	})
	return offers, nil
}

func (s *Store) TransitionPendingRideOffer(_ context.Context, offerID string, next domain.RideOfferState, updatedAt time.Time) (domain.RideOffer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	offer, ok := s.rideOffers[offerID]
	if !ok {
		return domain.RideOffer{}, domain.ErrOfferNotFound
	}
	if offer.State != domain.RideOfferStatePending {
		return domain.RideOffer{}, domain.RequireTransition("ride_offer", offer.State, false, next)
	}
	offer.State = next
	offer.UpdatedAt = updatedAt
	s.rideOffers[offerID] = offer
	return offer, nil
}

func (s *Store) SaveRideBooking(_ context.Context, booking domain.RideBooking) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rideBookings[booking.ID] = booking
	return nil
}

func (s *Store) FindRideBookingByID(_ context.Context, id string) (domain.RideBooking, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	booking, ok := s.rideBookings[id]
	if !ok {
		return domain.RideBooking{}, domain.ErrBookingNotFound
	}
	return booking, nil
}

func (s *Store) ListByDriverSessionID(_ context.Context, sessionID string) ([]domain.RideBooking, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var bookings []domain.RideBooking
	for _, booking := range s.rideBookings {
		if booking.DriverSessionID == sessionID {
			bookings = append(bookings, booking)
		}
	}
	return bookings, nil
}

func (s *Store) ListActiveBookingsByActorID(_ context.Context, actorUserID string) ([]domain.RideBooking, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var bookings []domain.RideBooking
	for _, booking := range s.rideBookings {
		if booking.RiderID != actorUserID && booking.DriverID != actorUserID {
			continue
		}
		if isTerminalBookingState(booking.State) {
			continue
		}
		bookings = append(bookings, booking)
	}
	sort.Slice(bookings, func(i, j int) bool {
		return bookings[i].UpdatedAt.After(bookings[j].UpdatedAt)
	})
	return bookings, nil
}

func (s *Store) SaveReview(_ context.Context, review domain.Review) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reviews[review.ID] = review
	return nil
}

func (s *Store) ListBySubjectID(_ context.Context, subjectID string) ([]domain.Review, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var reviews []domain.Review
	for _, review := range s.reviews {
		if review.SubjectID == subjectID {
			reviews = append(reviews, review)
		}
	}
	return reviews, nil
}

func (s *Store) SaveResult(_ context.Context, key string, resourceID string, payloadHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.idempotency[key] = idempotencyRecord{resourceID: resourceID, payloadHash: payloadHash}
	return nil
}

func (s *Store) FindResult(_ context.Context, key string) (string, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.idempotency[key]
	if !ok {
		return "", "", errors.New("idempotency key not found")
	}
	return record.resourceID, record.payloadHash, nil
}

func driverSessionStatePriority(state domain.DriverSessionState) int {
	switch state {
	case domain.DriverSessionStateActive:
		return 0
	case domain.DriverSessionStateFull:
		return 1
	case domain.DriverSessionStatePaused:
		return 2
	default:
		return 3
	}
}

func isTerminalBookingState(state domain.RideBookingState) bool {
	switch state {
	case domain.RideBookingStateCompleted, domain.RideBookingStateCanceled, domain.RideBookingStateNoShow:
		return true
	default:
		return false
	}
}
