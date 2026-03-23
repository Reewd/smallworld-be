DROP INDEX IF EXISTS idx_reviews_subject_created_at;
DROP INDEX IF EXISTS idx_ride_bookings_driver_session_created_at;
DROP INDEX IF EXISTS idx_ride_offers_demand_state_created_at;
DROP INDEX IF EXISTS idx_trip_demands_rider_state_updated_at;
DROP INDEX IF EXISTS idx_driver_sessions_state_updated_at;
DROP TABLE IF EXISTS idempotency_keys;
