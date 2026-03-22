package postgres

import (
	"database/sql"

	"smallworld/internal/domain"
)

func scanNullableLocation(lat, lng sql.NullFloat64) *domain.Location {
	if !lat.Valid || !lng.Valid {
		return nil
	}
	return &domain.Location{Lat: lat.Float64, Lng: lng.Float64}
}
