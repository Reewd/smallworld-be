ALTER TABLE driver_sessions
    ADD COLUMN IF NOT EXISTS route_polyline TEXT NOT NULL DEFAULT '';
