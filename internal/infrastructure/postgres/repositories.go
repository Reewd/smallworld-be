package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"smallworld/internal/domain"
)

type Users struct{ Pool *pgxpool.Pool }
type Verifications struct{ Pool *pgxpool.Pool }
type Vehicles struct{ Pool *pgxpool.Pool }
type DriverSessions struct{ Pool *pgxpool.Pool }
type TripDemands struct{ Pool *pgxpool.Pool }
type RideOffers struct{ Pool *pgxpool.Pool }
type RideBookings struct{ Pool *pgxpool.Pool }
type Reviews struct{ Pool *pgxpool.Pool }
type Idempotency struct{ Pool *pgxpool.Pool }

func (r Users) Save(ctx context.Context, user domain.User) error {
	_, err := r.Pool.Exec(ctx, `
		INSERT INTO users (
			id, auth_subject, display_name, average_rating,
			walk_to_pickup_preference, walk_from_dropoff_preference, driver_pickup_detour_preference, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			auth_subject = EXCLUDED.auth_subject,
			display_name = EXCLUDED.display_name,
			average_rating = EXCLUDED.average_rating,
			walk_to_pickup_preference = EXCLUDED.walk_to_pickup_preference,
			walk_from_dropoff_preference = EXCLUDED.walk_from_dropoff_preference,
			driver_pickup_detour_preference = EXCLUDED.driver_pickup_detour_preference
	`,
		user.ID,
		user.AuthSubject,
		user.DisplayName,
		user.AverageRating,
		string(user.Preferences.WalkToPickup),
		string(user.Preferences.WalkFromDropoff),
		string(user.Preferences.DriverPickupDetour),
		user.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save user: %w", err)
	}
	return nil
}

func (r Users) FindByID(ctx context.Context, id string) (domain.User, error) {
	var user domain.User
	err := r.Pool.QueryRow(ctx, `
		SELECT id, auth_subject, display_name, average_rating,
		       walk_to_pickup_preference, walk_from_dropoff_preference, driver_pickup_detour_preference, created_at
		FROM users
		WHERE id = $1
	`, id).Scan(
		&user.ID,
		&user.AuthSubject,
		&user.DisplayName,
		&user.AverageRating,
		&user.Preferences.WalkToPickup,
		&user.Preferences.WalkFromDropoff,
		&user.Preferences.DriverPickupDetour,
		&user.CreatedAt,
	)
	if isNotFound(err) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("find user by id: %w", err)
	}
	return user, nil
}

func (r Users) FindByAuthSubject(ctx context.Context, authSubject string) (domain.User, error) {
	var user domain.User
	err := r.Pool.QueryRow(ctx, `
		SELECT id, auth_subject, display_name, average_rating,
		       walk_to_pickup_preference, walk_from_dropoff_preference, driver_pickup_detour_preference, created_at
		FROM users
		WHERE auth_subject = $1
	`, authSubject).Scan(
		&user.ID,
		&user.AuthSubject,
		&user.DisplayName,
		&user.AverageRating,
		&user.Preferences.WalkToPickup,
		&user.Preferences.WalkFromDropoff,
		&user.Preferences.DriverPickupDetour,
		&user.CreatedAt,
	)
	if isNotFound(err) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("find user by auth subject: %w", err)
	}
	return user, nil
}

func (r Verifications) Save(ctx context.Context, verification domain.IdentityVerification) error {
	_, err := r.Pool.Exec(ctx, `
		INSERT INTO identity_verifications (
			user_id, status, provider, provider_ref, verified_gender, verified_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id) DO UPDATE SET
			status = EXCLUDED.status,
			provider = EXCLUDED.provider,
			provider_ref = EXCLUDED.provider_ref,
			verified_gender = EXCLUDED.verified_gender,
			verified_at = EXCLUDED.verified_at,
			updated_at = EXCLUDED.updated_at
	`,
		verification.UserID,
		string(verification.Status),
		verification.Provider,
		verification.ProviderRef,
		string(verification.VerifiedGender),
		verification.VerifiedAt,
		verification.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save verification: %w", err)
	}
	return nil
}

func (r Verifications) FindByUserID(ctx context.Context, userID string) (domain.IdentityVerification, error) {
	var verification domain.IdentityVerification
	var verifiedAt pgtype.Timestamptz
	var status string
	var gender string
	err := r.Pool.QueryRow(ctx, `
		SELECT user_id, status, provider, provider_ref, verified_gender, verified_at, updated_at
		FROM identity_verifications
		WHERE user_id = $1
	`, userID).Scan(
		&verification.UserID,
		&status,
		&verification.Provider,
		&verification.ProviderRef,
		&gender,
		&verifiedAt,
		&verification.UpdatedAt,
	)
	if isNotFound(err) {
		return domain.IdentityVerification{}, domain.ErrVerificationRequired
	}
	if err != nil {
		return domain.IdentityVerification{}, fmt.Errorf("find verification by user id: %w", err)
	}
	verification.Status = domain.VerificationStatus(status)
	verification.VerifiedGender = domain.Gender(gender)
	if verifiedAt.Valid {
		t := verifiedAt.Time
		verification.VerifiedAt = &t
	}
	return verification, nil
}

func (r Vehicles) Save(ctx context.Context, vehicle domain.Vehicle) error {
	_, err := r.Pool.Exec(ctx, `
		INSERT INTO vehicles (id, user_id, make, model, color, license_plate, capacity, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			make = EXCLUDED.make,
			model = EXCLUDED.model,
			color = EXCLUDED.color,
			license_plate = EXCLUDED.license_plate,
			capacity = EXCLUDED.capacity,
			is_active = EXCLUDED.is_active
	`,
		vehicle.ID,
		vehicle.UserID,
		vehicle.Make,
		vehicle.Model,
		vehicle.Color,
		vehicle.LicensePlate,
		vehicle.Capacity,
		vehicle.IsActive,
		vehicle.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save vehicle: %w", err)
	}
	return nil
}

func (r Vehicles) ListByUserID(ctx context.Context, userID string) ([]domain.Vehicle, error) {
	rows, err := r.Pool.Query(ctx, `
		SELECT id, user_id, make, model, color, license_plate, capacity, is_active, created_at
		FROM vehicles
		WHERE user_id = $1
		ORDER BY created_at ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list vehicles by user id: %w", err)
	}
	defer rows.Close()

	var vehicles []domain.Vehicle
	for rows.Next() {
		var vehicle domain.Vehicle
		if err := rows.Scan(
			&vehicle.ID,
			&vehicle.UserID,
			&vehicle.Make,
			&vehicle.Model,
			&vehicle.Color,
			&vehicle.LicensePlate,
			&vehicle.Capacity,
			&vehicle.IsActive,
			&vehicle.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan vehicle: %w", err)
		}
		vehicles = append(vehicles, vehicle)
	}
	return vehicles, rows.Err()
}

func (r Vehicles) FindByID(ctx context.Context, id string) (domain.Vehicle, error) {
	var vehicle domain.Vehicle
	err := r.Pool.QueryRow(ctx, `
		SELECT id, user_id, make, model, color, license_plate, capacity, is_active, created_at
		FROM vehicles
		WHERE id = $1
	`, id).Scan(
		&vehicle.ID,
		&vehicle.UserID,
		&vehicle.Make,
		&vehicle.Model,
		&vehicle.Color,
		&vehicle.LicensePlate,
		&vehicle.Capacity,
		&vehicle.IsActive,
		&vehicle.CreatedAt,
	)
	if isNotFound(err) {
		return domain.Vehicle{}, fmt.Errorf("vehicle not found")
	}
	if err != nil {
		return domain.Vehicle{}, fmt.Errorf("find vehicle by id: %w", err)
	}
	return vehicle, nil
}

func (r DriverSessions) Save(ctx context.Context, session domain.DriverSession) error {
	_, err := r.Pool.Exec(ctx, `
		INSERT INTO driver_sessions (
			id, driver_id, vehicle_id, state, origin, destination, current_location,
			remaining_capacity, max_driver_pickup_detour_meters, route_distance_meters, route_duration_seconds,
			route_polyline, last_heartbeat_at, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4,
			ST_SetSRID(ST_MakePoint($6, $5), 4326)::geography,
			ST_SetSRID(ST_MakePoint($8, $7), 4326)::geography,
			ST_SetSRID(ST_MakePoint($10, $9), 4326)::geography,
			$11, $12, $13, $14, $15, $16, $17, $18
		)
		ON CONFLICT (id) DO UPDATE SET
			driver_id = EXCLUDED.driver_id,
			vehicle_id = EXCLUDED.vehicle_id,
			state = EXCLUDED.state,
			origin = EXCLUDED.origin,
			destination = EXCLUDED.destination,
			current_location = EXCLUDED.current_location,
			remaining_capacity = EXCLUDED.remaining_capacity,
			max_driver_pickup_detour_meters = EXCLUDED.max_driver_pickup_detour_meters,
			route_distance_meters = EXCLUDED.route_distance_meters,
			route_duration_seconds = EXCLUDED.route_duration_seconds,
			route_polyline = EXCLUDED.route_polyline,
			last_heartbeat_at = EXCLUDED.last_heartbeat_at,
			updated_at = EXCLUDED.updated_at
	`,
		session.ID,
		session.DriverID,
		session.VehicleID,
		string(session.State),
		session.Origin.Lat,
		session.Origin.Lng,
		session.Destination.Lat,
		session.Destination.Lng,
		session.CurrentLocation.Lat,
		session.CurrentLocation.Lng,
		session.RemainingCapacity,
		session.MaxDriverPickupDetourMeters,
		session.RouteDistanceMeters,
		session.RouteDurationSeconds,
		session.RoutePolyline,
		session.LastHeartbeatAt,
		session.CreatedAt,
		session.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save driver session: %w", err)
	}
	return nil
}

func (r DriverSessions) FindByID(ctx context.Context, id string) (domain.DriverSession, error) {
	row := r.Pool.QueryRow(ctx, `
		SELECT
			id, driver_id, vehicle_id, state,
			ST_Y(origin::geometry), ST_X(origin::geometry),
			ST_Y(destination::geometry), ST_X(destination::geometry),
			ST_Y(current_location::geometry), ST_X(current_location::geometry),
			remaining_capacity, max_driver_pickup_detour_meters, route_distance_meters, route_duration_seconds, route_polyline,
			last_heartbeat_at, created_at, updated_at
		FROM driver_sessions
		WHERE id = $1
	`, id)
	return scanDriverSession(row)
}

func (r DriverSessions) ListActive(ctx context.Context) ([]domain.DriverSession, error) {
	rows, err := r.Pool.Query(ctx, `
		SELECT
			id, driver_id, vehicle_id, state,
			ST_Y(origin::geometry), ST_X(origin::geometry),
			ST_Y(destination::geometry), ST_X(destination::geometry),
			ST_Y(current_location::geometry), ST_X(current_location::geometry),
			remaining_capacity, max_driver_pickup_detour_meters, route_distance_meters, route_duration_seconds, route_polyline,
			last_heartbeat_at, created_at, updated_at
		FROM driver_sessions
		WHERE state IN ('active', 'full')
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list active driver sessions: %w", err)
	}
	defer rows.Close()

	var sessions []domain.DriverSession
	for rows.Next() {
		session, err := scanDriverSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (r DriverSessions) FindCurrentByDriverID(ctx context.Context, driverID string) (domain.DriverSession, error) {
	row := r.Pool.QueryRow(ctx, `
		SELECT
			id, driver_id, vehicle_id, state,
			ST_Y(origin::geometry), ST_X(origin::geometry),
			ST_Y(destination::geometry), ST_X(destination::geometry),
			ST_Y(current_location::geometry), ST_X(current_location::geometry),
			remaining_capacity, max_driver_pickup_detour_meters, route_distance_meters, route_duration_seconds, route_polyline,
			last_heartbeat_at, created_at, updated_at
		FROM driver_sessions
		WHERE driver_id = $1 AND state IN ('active', 'full', 'paused')
		ORDER BY
			CASE state
				WHEN 'active' THEN 0
				WHEN 'full' THEN 1
				WHEN 'paused' THEN 2
				ELSE 3
			END,
			updated_at DESC
		LIMIT 1
	`, driverID)
	return scanDriverSession(row)
}

func (r TripDemands) Save(ctx context.Context, demand domain.TripDemand) error {
	var matchedPickupLat any
	var matchedPickupLng any
	var matchedDropoffLat any
	var matchedDropoffLng any
	if demand.MatchedPickup != nil {
		matchedPickupLat = demand.MatchedPickup.Lat
		matchedPickupLng = demand.MatchedPickup.Lng
	}
	if demand.MatchedDropoff != nil {
		matchedDropoffLat = demand.MatchedDropoff.Lat
		matchedDropoffLng = demand.MatchedDropoff.Lng
	}

	_, err := r.Pool.Exec(ctx, `
		INSERT INTO trip_demands (
			id, rider_id, state, requested_origin, requested_destination,
			matched_pickup, matched_dropoff, women_drivers_only,
			max_walk_to_pickup_meters, max_walk_from_dropoff_meters, idempotency_key, created_at, updated_at
		)
		VALUES (
			$1, $2, $3,
			ST_SetSRID(ST_MakePoint($5, $4), 4326)::geography,
			ST_SetSRID(ST_MakePoint($7, $6), 4326)::geography,
			CASE WHEN $8::double precision IS NULL OR $9::double precision IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($9, $8), 4326)::geography END,
			CASE WHEN $10::double precision IS NULL OR $11::double precision IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($11, $10), 4326)::geography END,
			$12, $13, $14, $15, $16, $17
		)
		ON CONFLICT (id) DO UPDATE SET
			rider_id = EXCLUDED.rider_id,
			state = EXCLUDED.state,
			requested_origin = EXCLUDED.requested_origin,
			requested_destination = EXCLUDED.requested_destination,
			matched_pickup = EXCLUDED.matched_pickup,
			matched_dropoff = EXCLUDED.matched_dropoff,
			women_drivers_only = EXCLUDED.women_drivers_only,
			max_walk_to_pickup_meters = EXCLUDED.max_walk_to_pickup_meters,
			max_walk_from_dropoff_meters = EXCLUDED.max_walk_from_dropoff_meters,
			idempotency_key = EXCLUDED.idempotency_key,
			updated_at = EXCLUDED.updated_at
	`,
		demand.ID,
		demand.RiderID,
		string(demand.State),
		demand.RequestedOrigin.Lat,
		demand.RequestedOrigin.Lng,
		demand.RequestedDestination.Lat,
		demand.RequestedDestination.Lng,
		matchedPickupLat,
		matchedPickupLng,
		matchedDropoffLat,
		matchedDropoffLng,
		demand.WomenDriversOnly,
		demand.MaxWalkToPickupMeters,
		demand.MaxWalkFromDropoffMeters,
		demand.IdempotencyKey,
		demand.CreatedAt,
		demand.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save trip demand: %w", err)
	}
	return nil
}

func (r TripDemands) FindByID(ctx context.Context, id string) (domain.TripDemand, error) {
	row := r.Pool.QueryRow(ctx, `
		SELECT
			id, rider_id, state,
			ST_Y(requested_origin::geometry), ST_X(requested_origin::geometry),
			ST_Y(requested_destination::geometry), ST_X(requested_destination::geometry),
			ST_Y(matched_pickup::geometry), ST_X(matched_pickup::geometry),
			ST_Y(matched_dropoff::geometry), ST_X(matched_dropoff::geometry),
			women_drivers_only, max_walk_to_pickup_meters, max_walk_from_dropoff_meters,
			COALESCE(idempotency_key, ''), created_at, updated_at
		FROM trip_demands
		WHERE id = $1
	`, id)
	return scanTripDemand(row)
}

func (r TripDemands) FindActiveByRiderID(ctx context.Context, riderID string) (domain.TripDemand, error) {
	row := r.Pool.QueryRow(ctx, `
		SELECT
			id, rider_id, state,
			ST_Y(requested_origin::geometry), ST_X(requested_origin::geometry),
			ST_Y(requested_destination::geometry), ST_X(requested_destination::geometry),
			ST_Y(matched_pickup::geometry), ST_X(matched_pickup::geometry),
			ST_Y(matched_dropoff::geometry), ST_X(matched_dropoff::geometry),
			women_drivers_only, max_walk_to_pickup_meters, max_walk_from_dropoff_meters,
			COALESCE(idempotency_key, ''), created_at, updated_at
		FROM trip_demands
		WHERE rider_id = $1 AND state IN ('searching', 'offered')
		ORDER BY updated_at DESC
		LIMIT 1
	`, riderID)
	return scanTripDemand(row)
}

func (r RideOffers) Save(ctx context.Context, offer domain.RideOffer) error {
	_, err := r.Pool.Exec(ctx, `
		INSERT INTO ride_offers (
			id, demand_id, driver_session_id, state, detour_meters, pickup_eta_seconds, fare_cents, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			demand_id = EXCLUDED.demand_id,
			driver_session_id = EXCLUDED.driver_session_id,
			state = EXCLUDED.state,
			detour_meters = EXCLUDED.detour_meters,
			pickup_eta_seconds = EXCLUDED.pickup_eta_seconds,
			fare_cents = EXCLUDED.fare_cents,
			updated_at = EXCLUDED.updated_at
	`,
		offer.ID,
		offer.DemandID,
		offer.DriverSessionID,
		string(offer.State),
		offer.DetourMeters,
		offer.PickupETASeconds,
		offer.FareCents,
		offer.CreatedAt,
		offer.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save ride offer: %w", err)
	}
	return nil
}

func (r RideOffers) FindByID(ctx context.Context, id string) (domain.RideOffer, error) {
	var offer domain.RideOffer
	var state string
	err := r.Pool.QueryRow(ctx, `
		SELECT id, demand_id, driver_session_id, state, detour_meters, pickup_eta_seconds, fare_cents, created_at, updated_at
		FROM ride_offers
		WHERE id = $1
	`, id).Scan(
		&offer.ID,
		&offer.DemandID,
		&offer.DriverSessionID,
		&state,
		&offer.DetourMeters,
		&offer.PickupETASeconds,
		&offer.FareCents,
		&offer.CreatedAt,
		&offer.UpdatedAt,
	)
	if isNotFound(err) {
		return domain.RideOffer{}, domain.ErrOfferNotFound
	}
	if err != nil {
		return domain.RideOffer{}, fmt.Errorf("find ride offer by id: %w", err)
	}
	offer.State = domain.RideOfferState(state)
	return offer, nil
}

func (r RideOffers) FindPendingByDemandID(ctx context.Context, demandID string) (domain.RideOffer, error) {
	return scanRideOffer(r.Pool.QueryRow(ctx, `
		SELECT id, demand_id, driver_session_id, state, detour_meters, pickup_eta_seconds, fare_cents, created_at, updated_at
		FROM ride_offers
		WHERE demand_id = $1 AND state = 'pending'
		ORDER BY created_at DESC
		LIMIT 1
	`, demandID))
}

func (r RideOffers) ListPending(ctx context.Context) ([]domain.RideOffer, error) {
	rows, err := r.Pool.Query(ctx, `
		SELECT id, demand_id, driver_session_id, state, detour_meters, pickup_eta_seconds, fare_cents, created_at, updated_at
		FROM ride_offers
		WHERE state = 'pending'
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list pending ride offers: %w", err)
	}
	defer rows.Close()

	var offers []domain.RideOffer
	for rows.Next() {
		offer, err := scanRideOffer(rows)
		if err != nil {
			return nil, err
		}
		offers = append(offers, offer)
	}
	return offers, rows.Err()
}

func (r RideOffers) ListPendingByDriverID(ctx context.Context, driverID string) ([]domain.RideOffer, error) {
	rows, err := r.Pool.Query(ctx, `
		SELECT ro.id, ro.demand_id, ro.driver_session_id, ro.state, ro.detour_meters, ro.pickup_eta_seconds, ro.fare_cents, ro.created_at, ro.updated_at
		FROM ride_offers ro
		INNER JOIN driver_sessions ds ON ds.id = ro.driver_session_id
		WHERE ds.driver_id = $1 AND ro.state = 'pending'
		ORDER BY ro.created_at DESC
	`, driverID)
	if err != nil {
		return nil, fmt.Errorf("list pending ride offers by driver id: %w", err)
	}
	defer rows.Close()

	var offers []domain.RideOffer
	for rows.Next() {
		offer, err := scanRideOffer(rows)
		if err != nil {
			return nil, err
		}
		offers = append(offers, offer)
	}
	return offers, rows.Err()
}

func (r RideOffers) TransitionPending(ctx context.Context, offerID string, next domain.RideOfferState, updatedAt time.Time) (domain.RideOffer, error) {
	return scanRideOffer(r.Pool.QueryRow(ctx, `
		UPDATE ride_offers
		SET state = $2, updated_at = $3
		WHERE id = $1 AND state = 'pending'
		RETURNING id, demand_id, driver_session_id, state, detour_meters, pickup_eta_seconds, fare_cents, created_at, updated_at
	`, offerID, string(next), updatedAt))
}

func (r RideBookings) Save(ctx context.Context, booking domain.RideBooking) error {
	_, err := r.Pool.Exec(ctx, `
		INSERT INTO ride_bookings (
			id, demand_id, driver_session_id, rider_id, driver_id, state,
			matched_pickup, matched_dropoff, rider_walk_to_pickup_m, rider_walk_from_dropoff_m,
			driver_pickup_detour_m, quoted_fare_cents, vehicle_license_plate, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6,
			ST_SetSRID(ST_MakePoint($8, $7), 4326)::geography,
			ST_SetSRID(ST_MakePoint($10, $9), 4326)::geography,
			$11, $12, $13, $14, $15, $16, $17
		)
		ON CONFLICT (id) DO UPDATE SET
			demand_id = EXCLUDED.demand_id,
			driver_session_id = EXCLUDED.driver_session_id,
			rider_id = EXCLUDED.rider_id,
			driver_id = EXCLUDED.driver_id,
			state = EXCLUDED.state,
			matched_pickup = EXCLUDED.matched_pickup,
			matched_dropoff = EXCLUDED.matched_dropoff,
			rider_walk_to_pickup_m = EXCLUDED.rider_walk_to_pickup_m,
			rider_walk_from_dropoff_m = EXCLUDED.rider_walk_from_dropoff_m,
			driver_pickup_detour_m = EXCLUDED.driver_pickup_detour_m,
			quoted_fare_cents = EXCLUDED.quoted_fare_cents,
			vehicle_license_plate = EXCLUDED.vehicle_license_plate,
			updated_at = EXCLUDED.updated_at
	`,
		booking.ID,
		booking.DemandID,
		booking.DriverSessionID,
		booking.RiderID,
		booking.DriverID,
		string(booking.State),
		booking.MatchedPickup.Lat,
		booking.MatchedPickup.Lng,
		booking.MatchedDropoff.Lat,
		booking.MatchedDropoff.Lng,
		booking.RiderWalkToPickupM,
		booking.RiderWalkFromDropoffM,
		booking.DriverPickupDetourM,
		booking.QuotedFareCents,
		booking.VehicleLicensePlate,
		booking.CreatedAt,
		booking.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save ride booking: %w", err)
	}
	return nil
}

func (r RideBookings) FindByID(ctx context.Context, id string) (domain.RideBooking, error) {
	var booking domain.RideBooking
	var state string
	err := r.Pool.QueryRow(ctx, `
		SELECT
			id, demand_id, driver_session_id, rider_id, driver_id, state,
			ST_Y(matched_pickup::geometry), ST_X(matched_pickup::geometry),
			ST_Y(matched_dropoff::geometry), ST_X(matched_dropoff::geometry),
			rider_walk_to_pickup_m, rider_walk_from_dropoff_m, driver_pickup_detour_m,
			quoted_fare_cents, vehicle_license_plate, created_at, updated_at
		FROM ride_bookings
		WHERE id = $1
	`, id).Scan(
		&booking.ID,
		&booking.DemandID,
		&booking.DriverSessionID,
		&booking.RiderID,
		&booking.DriverID,
		&state,
		&booking.MatchedPickup.Lat,
		&booking.MatchedPickup.Lng,
		&booking.MatchedDropoff.Lat,
		&booking.MatchedDropoff.Lng,
		&booking.RiderWalkToPickupM,
		&booking.RiderWalkFromDropoffM,
		&booking.DriverPickupDetourM,
		&booking.QuotedFareCents,
		&booking.VehicleLicensePlate,
		&booking.CreatedAt,
		&booking.UpdatedAt,
	)
	if isNotFound(err) {
		return domain.RideBooking{}, domain.ErrBookingNotFound
	}
	if err != nil {
		return domain.RideBooking{}, fmt.Errorf("find ride booking by id: %w", err)
	}
	booking.State = domain.RideBookingState(state)
	return booking, nil
}

func (r RideBookings) ListByDriverSessionID(ctx context.Context, sessionID string) ([]domain.RideBooking, error) {
	rows, err := r.Pool.Query(ctx, `
		SELECT
			id, demand_id, driver_session_id, rider_id, driver_id, state,
			ST_Y(matched_pickup::geometry), ST_X(matched_pickup::geometry),
			ST_Y(matched_dropoff::geometry), ST_X(matched_dropoff::geometry),
			rider_walk_to_pickup_m, rider_walk_from_dropoff_m, driver_pickup_detour_m,
			quoted_fare_cents, vehicle_license_plate, created_at, updated_at
		FROM ride_bookings
		WHERE driver_session_id = $1
		ORDER BY created_at ASC
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list ride bookings by driver session id: %w", err)
	}
	defer rows.Close()

	var bookings []domain.RideBooking
	for rows.Next() {
		var booking domain.RideBooking
		var state string
		if err := rows.Scan(
			&booking.ID,
			&booking.DemandID,
			&booking.DriverSessionID,
			&booking.RiderID,
			&booking.DriverID,
			&state,
			&booking.MatchedPickup.Lat,
			&booking.MatchedPickup.Lng,
			&booking.MatchedDropoff.Lat,
			&booking.MatchedDropoff.Lng,
			&booking.RiderWalkToPickupM,
			&booking.RiderWalkFromDropoffM,
			&booking.DriverPickupDetourM,
			&booking.QuotedFareCents,
			&booking.VehicleLicensePlate,
			&booking.CreatedAt,
			&booking.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ride booking: %w", err)
		}
		booking.State = domain.RideBookingState(state)
		bookings = append(bookings, booking)
	}
	return bookings, rows.Err()
}

func (r RideBookings) ListActiveByActorID(ctx context.Context, actorUserID string) ([]domain.RideBooking, error) {
	rows, err := r.Pool.Query(ctx, `
		SELECT
			id, demand_id, driver_session_id, rider_id, driver_id, state,
			ST_Y(matched_pickup::geometry), ST_X(matched_pickup::geometry),
			ST_Y(matched_dropoff::geometry), ST_X(matched_dropoff::geometry),
			rider_walk_to_pickup_m, rider_walk_from_dropoff_m, driver_pickup_detour_m,
			quoted_fare_cents, vehicle_license_plate, created_at, updated_at
		FROM ride_bookings
		WHERE (rider_id = $1 OR driver_id = $1)
		  AND state NOT IN ('completed', 'canceled', 'no_show')
		ORDER BY updated_at DESC
	`, actorUserID)
	if err != nil {
		return nil, fmt.Errorf("list active ride bookings by actor id: %w", err)
	}
	defer rows.Close()

	var bookings []domain.RideBooking
	for rows.Next() {
		var booking domain.RideBooking
		var state string
		if err := rows.Scan(
			&booking.ID,
			&booking.DemandID,
			&booking.DriverSessionID,
			&booking.RiderID,
			&booking.DriverID,
			&state,
			&booking.MatchedPickup.Lat,
			&booking.MatchedPickup.Lng,
			&booking.MatchedDropoff.Lat,
			&booking.MatchedDropoff.Lng,
			&booking.RiderWalkToPickupM,
			&booking.RiderWalkFromDropoffM,
			&booking.DriverPickupDetourM,
			&booking.QuotedFareCents,
			&booking.VehicleLicensePlate,
			&booking.CreatedAt,
			&booking.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan active ride booking: %w", err)
		}
		booking.State = domain.RideBookingState(state)
		bookings = append(bookings, booking)
	}
	return bookings, rows.Err()
}

func (r Reviews) Save(ctx context.Context, review domain.Review) error {
	_, err := r.Pool.Exec(ctx, `
		INSERT INTO reviews (id, booking_id, author_id, subject_id, rating, comment, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			booking_id = EXCLUDED.booking_id,
			author_id = EXCLUDED.author_id,
			subject_id = EXCLUDED.subject_id,
			rating = EXCLUDED.rating,
			comment = EXCLUDED.comment
	`,
		review.ID,
		review.BookingID,
		review.AuthorID,
		review.SubjectID,
		review.Rating,
		review.Comment,
		review.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save review: %w", err)
	}
	return nil
}

func (r Reviews) ListBySubjectID(ctx context.Context, subjectID string) ([]domain.Review, error) {
	rows, err := r.Pool.Query(ctx, `
		SELECT id, booking_id, author_id, subject_id, rating, comment, created_at
		FROM reviews
		WHERE subject_id = $1
		ORDER BY created_at DESC
	`, subjectID)
	if err != nil {
		return nil, fmt.Errorf("list reviews by subject id: %w", err)
	}
	defer rows.Close()

	var reviews []domain.Review
	for rows.Next() {
		var review domain.Review
		if err := rows.Scan(
			&review.ID,
			&review.BookingID,
			&review.AuthorID,
			&review.SubjectID,
			&review.Rating,
			&review.Comment,
			&review.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

func (r Idempotency) SaveResult(ctx context.Context, key string, resourceID string, payloadHash string) error {
	_, err := r.Pool.Exec(ctx, `
		INSERT INTO idempotency_keys (key, resource_id, payload_hash, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (key) DO UPDATE SET
			resource_id = EXCLUDED.resource_id,
			payload_hash = EXCLUDED.payload_hash
	`, key, resourceID, payloadHash)
	if err != nil {
		return fmt.Errorf("save idempotency result: %w", err)
	}
	return nil
}

func (r Idempotency) FindResult(ctx context.Context, key string) (string, string, error) {
	var resourceID string
	var payloadHash string
	err := r.Pool.QueryRow(ctx, `
		SELECT resource_id, payload_hash
		FROM idempotency_keys
		WHERE key = $1
	`, key).Scan(&resourceID, &payloadHash)
	if isNotFound(err) {
		return "", "", domain.ErrDemandNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("find idempotency result: %w", err)
	}
	return resourceID, payloadHash, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanDriverSession(row scannable) (domain.DriverSession, error) {
	var session domain.DriverSession
	var state string
	err := row.Scan(
		&session.ID,
		&session.DriverID,
		&session.VehicleID,
		&state,
		&session.Origin.Lat,
		&session.Origin.Lng,
		&session.Destination.Lat,
		&session.Destination.Lng,
		&session.CurrentLocation.Lat,
		&session.CurrentLocation.Lng,
		&session.RemainingCapacity,
		&session.MaxDriverPickupDetourMeters,
		&session.RouteDistanceMeters,
		&session.RouteDurationSeconds,
		&session.RoutePolyline,
		&session.LastHeartbeatAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if isNotFound(err) {
		return domain.DriverSession{}, domain.ErrDriverSessionNotFound
	}
	if err != nil {
		return domain.DriverSession{}, fmt.Errorf("scan driver session: %w", err)
	}
	session.State = domain.DriverSessionState(state)
	return session, nil
}

func scanTripDemand(row scannable) (domain.TripDemand, error) {
	var demand domain.TripDemand
	var state string
	var pickupLat sql.NullFloat64
	var pickupLng sql.NullFloat64
	var dropoffLat sql.NullFloat64
	var dropoffLng sql.NullFloat64
	err := row.Scan(
		&demand.ID,
		&demand.RiderID,
		&state,
		&demand.RequestedOrigin.Lat,
		&demand.RequestedOrigin.Lng,
		&demand.RequestedDestination.Lat,
		&demand.RequestedDestination.Lng,
		&pickupLat,
		&pickupLng,
		&dropoffLat,
		&dropoffLng,
		&demand.WomenDriversOnly,
		&demand.MaxWalkToPickupMeters,
		&demand.MaxWalkFromDropoffMeters,
		&demand.IdempotencyKey,
		&demand.CreatedAt,
		&demand.UpdatedAt,
	)
	if isNotFound(err) {
		return domain.TripDemand{}, domain.ErrDemandNotFound
	}
	if err != nil {
		return domain.TripDemand{}, fmt.Errorf("scan trip demand: %w", err)
	}
	demand.State = domain.TripDemandState(state)
	demand.MatchedPickup = scanNullableLocation(pickupLat, pickupLng)
	demand.MatchedDropoff = scanNullableLocation(dropoffLat, dropoffLng)
	return demand, nil
}

func scanRideOffer(row scannable) (domain.RideOffer, error) {
	var offer domain.RideOffer
	var state string
	err := row.Scan(
		&offer.ID,
		&offer.DemandID,
		&offer.DriverSessionID,
		&state,
		&offer.DetourMeters,
		&offer.PickupETASeconds,
		&offer.FareCents,
		&offer.CreatedAt,
		&offer.UpdatedAt,
	)
	if isNotFound(err) {
		return domain.RideOffer{}, domain.ErrOfferNotFound
	}
	if err != nil {
		return domain.RideOffer{}, fmt.Errorf("scan ride offer: %w", err)
	}
	offer.State = domain.RideOfferState(state)
	return offer, nil
}
