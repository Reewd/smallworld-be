package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"smallworld/internal/domain"
)

type OfferAcceptor struct {
	Pool *pgxpool.Pool
}

func (a OfferAcceptor) AcceptOffer(ctx context.Context, offerID string, bookingID string, now time.Time) (domain.RideBooking, error) {
	tx, err := a.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.RideBooking{}, fmt.Errorf("begin offer acceptance transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	offer, err := scanRideOffer(tx.QueryRow(ctx, `
		SELECT id, demand_id, driver_session_id, state, detour_meters, pickup_eta_seconds, fare_cents, created_at, updated_at
		FROM ride_offers
		WHERE id = $1
		FOR UPDATE
	`, offerID))
	if err != nil {
		return domain.RideBooking{}, err
	}
	if err := domain.RequireTransition("ride_offer", offer.State, offer.CanTransitionTo(domain.RideOfferStateAccepted), domain.RideOfferStateAccepted); err != nil {
		return domain.RideBooking{}, err
	}

	session, err := scanDriverSession(tx.QueryRow(ctx, `
		SELECT
			id, driver_id, vehicle_id, state,
			ST_Y(origin::geometry), ST_X(origin::geometry),
			ST_Y(destination::geometry), ST_X(destination::geometry),
			ST_Y(current_location::geometry), ST_X(current_location::geometry),
			remaining_capacity, max_driver_pickup_detour_meters, route_distance_meters, route_duration_seconds, route_polyline,
			last_heartbeat_at, created_at, updated_at
		FROM driver_sessions
		WHERE id = $1
		FOR UPDATE
	`, offer.DriverSessionID))
	if err != nil {
		return domain.RideBooking{}, err
	}
	if session.RemainingCapacity <= 0 {
		return domain.RideBooking{}, domain.ErrCapacityExceeded
	}

	demand, err := scanTripDemand(tx.QueryRow(ctx, `
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
		FOR UPDATE
	`, offer.DemandID))
	if err != nil {
		return domain.RideBooking{}, err
	}

	var vehicle domain.Vehicle
	if err := tx.QueryRow(ctx, `
		SELECT id, user_id, make, model, color, license_plate, capacity, is_active, created_at
		FROM vehicles
		WHERE id = $1
	`, session.VehicleID).Scan(
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
		if isNotFound(err) {
			return domain.RideBooking{}, fmt.Errorf("vehicle not found")
		}
		return domain.RideBooking{}, fmt.Errorf("find vehicle by id: %w", err)
	}

	offer.State = domain.RideOfferStateAccepted
	offer.UpdatedAt = now
	if _, err := tx.Exec(ctx, `
		UPDATE ride_offers
		SET state = $2, updated_at = $3
		WHERE id = $1
	`, offer.ID, string(offer.State), offer.UpdatedAt); err != nil {
		return domain.RideBooking{}, fmt.Errorf("update accepted ride offer: %w", err)
	}

	booking := domain.RideBooking{
		ID:                  bookingID,
		DemandID:            demand.ID,
		DriverSessionID:     session.ID,
		RiderID:             demand.RiderID,
		DriverID:            session.DriverID,
		State:               domain.RideBookingStateAssigned,
		MatchedPickup:       derefLocation(demand.MatchedPickup, demand.RequestedOrigin),
		MatchedDropoff:      derefLocation(demand.MatchedDropoff, demand.RequestedDestination),
		QuotedFareCents:     offer.FareCents,
		VehicleLicensePlate: vehicle.LicensePlate,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if _, err := tx.Exec(ctx, `
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
	); err != nil {
		return domain.RideBooking{}, fmt.Errorf("insert ride booking: %w", err)
	}

	session.RemainingCapacity--
	session.UpdatedAt = now
	if session.RemainingCapacity <= 0 {
		session.State = domain.DriverSessionStateFull
	}
	if _, err := tx.Exec(ctx, `
		UPDATE driver_sessions
		SET state = $2, remaining_capacity = $3, updated_at = $4
		WHERE id = $1
	`, session.ID, string(session.State), session.RemainingCapacity, session.UpdatedAt); err != nil {
		return domain.RideBooking{}, fmt.Errorf("update driver session capacity: %w", err)
	}

	demand.State = domain.TripDemandStateMatched
	demand.UpdatedAt = now
	if _, err := tx.Exec(ctx, `
		UPDATE trip_demands
		SET state = $2, updated_at = $3
		WHERE id = $1
	`, demand.ID, string(demand.State), demand.UpdatedAt); err != nil {
		return domain.RideBooking{}, fmt.Errorf("update trip demand state: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.RideBooking{}, fmt.Errorf("commit offer acceptance transaction: %w", err)
	}
	return booking, nil
}

func derefLocation(location *domain.Location, fallback domain.Location) domain.Location {
	if location == nil {
		return fallback
	}
	return *location
}
