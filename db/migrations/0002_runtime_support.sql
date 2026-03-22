CREATE TABLE IF NOT EXISTS idempotency_keys (
    key TEXT PRIMARY KEY,
    resource_id TEXT NOT NULL,
    payload_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_driver_sessions_state_updated_at
    ON driver_sessions (state, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_trip_demands_rider_state_updated_at
    ON trip_demands (rider_id, state, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_ride_offers_demand_state_created_at
    ON ride_offers (demand_id, state, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ride_bookings_driver_session_created_at
    ON ride_bookings (driver_session_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_reviews_subject_created_at
    ON reviews (subject_id, created_at DESC);
