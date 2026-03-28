CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    auth_subject TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    average_rating NUMERIC(3,2) NOT NULL DEFAULT 0,
    walk_to_pickup_preference TEXT NOT NULL DEFAULT 'medium' CHECK (walk_to_pickup_preference IN ('low', 'medium', 'big')),
    walk_from_dropoff_preference TEXT NOT NULL DEFAULT 'medium' CHECK (walk_from_dropoff_preference IN ('low', 'medium', 'big')),
    driver_pickup_detour_preference TEXT NOT NULL DEFAULT 'medium' CHECK (driver_pickup_detour_preference IN ('low', 'medium', 'big')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE identity_verifications (
    user_id TEXT PRIMARY KEY REFERENCES users(id),
    status TEXT NOT NULL,
    provider TEXT NOT NULL,
    provider_ref TEXT NOT NULL,
    verified_gender TEXT NOT NULL DEFAULT 'unknown',
    verified_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE vehicles (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    make TEXT NOT NULL,
    model TEXT NOT NULL,
    color TEXT NOT NULL,
    license_plate TEXT NOT NULL,
    capacity INTEGER NOT NULL CHECK (capacity > 0),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE driver_sessions (
    id TEXT PRIMARY KEY,
    driver_id TEXT NOT NULL REFERENCES users(id),
    vehicle_id TEXT NOT NULL REFERENCES vehicles(id),
    state TEXT NOT NULL,
    origin GEOGRAPHY(POINT, 4326) NOT NULL,
    destination GEOGRAPHY(POINT, 4326) NOT NULL,
    current_location GEOGRAPHY(POINT, 4326) NOT NULL,
    remaining_capacity INTEGER NOT NULL CHECK (remaining_capacity >= 0),
    max_driver_pickup_detour_meters INTEGER NOT NULL,
    route_distance_meters INTEGER NOT NULL,
    route_duration_seconds INTEGER NOT NULL,
    last_heartbeat_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE trip_demands (
    id TEXT PRIMARY KEY,
    rider_id TEXT NOT NULL REFERENCES users(id),
    state TEXT NOT NULL,
    requested_origin GEOGRAPHY(POINT, 4326) NOT NULL,
    requested_destination GEOGRAPHY(POINT, 4326) NOT NULL,
    matched_pickup GEOGRAPHY(POINT, 4326),
    matched_dropoff GEOGRAPHY(POINT, 4326),
    women_drivers_only BOOLEAN NOT NULL DEFAULT FALSE,
    max_walk_to_pickup_meters INTEGER NOT NULL,
    max_walk_from_dropoff_meters INTEGER NOT NULL,
    idempotency_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ride_offers (
    id TEXT PRIMARY KEY,
    demand_id TEXT NOT NULL REFERENCES trip_demands(id),
    driver_session_id TEXT NOT NULL REFERENCES driver_sessions(id),
    state TEXT NOT NULL,
    detour_meters INTEGER NOT NULL,
    pickup_eta_seconds INTEGER NOT NULL,
    fare_cents INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ride_bookings (
    id TEXT PRIMARY KEY,
    demand_id TEXT NOT NULL REFERENCES trip_demands(id),
    driver_session_id TEXT NOT NULL REFERENCES driver_sessions(id),
    rider_id TEXT NOT NULL REFERENCES users(id),
    driver_id TEXT NOT NULL REFERENCES users(id),
    state TEXT NOT NULL,
    matched_pickup GEOGRAPHY(POINT, 4326) NOT NULL,
    matched_dropoff GEOGRAPHY(POINT, 4326) NOT NULL,
    rider_walk_to_pickup_m INTEGER NOT NULL DEFAULT 0,
    rider_walk_from_dropoff_m INTEGER NOT NULL DEFAULT 0,
    driver_pickup_detour_m INTEGER NOT NULL DEFAULT 0,
    quoted_fare_cents INTEGER NOT NULL,
    vehicle_license_plate TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE reviews (
    id TEXT PRIMARY KEY,
    booking_id TEXT NOT NULL REFERENCES ride_bookings(id),
    author_id TEXT NOT NULL REFERENCES users(id),
    subject_id TEXT NOT NULL REFERENCES users(id),
    rating INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE audit_events (
    id BIGSERIAL PRIMARY KEY,
    actor_user_id TEXT,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    action TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
