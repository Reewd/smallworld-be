package routing

import (
	"context"
	"math"

	"smallworld/internal/domain"
	"smallworld/internal/ports"
)

type HaversineProvider struct {
	WalkingMetersPerSecond float64
	DrivingMetersPerSecond float64
}

func NewHaversineProvider() *HaversineProvider {
	return &HaversineProvider{
		WalkingMetersPerSecond: 1.4,
		DrivingMetersPerSecond: 10.0,
	}
}

func (p *HaversineProvider) Route(_ context.Context, origin, destination domain.Location) (ports.RoutePlan, error) {
	distance := haversineMeters(origin, destination)
	return ports.RoutePlan{
		DistanceMeters:  distance,
		DurationSeconds: int(float64(distance) / p.DrivingMetersPerSecond),
	}, nil
}

func (p *HaversineProvider) WalkingETASeconds(_ context.Context, origin, destination domain.Location) (int, int, error) {
	distance := haversineMeters(origin, destination)
	return int(float64(distance) / p.WalkingMetersPerSecond), distance, nil
}

func (p *HaversineProvider) DrivingETASeconds(_ context.Context, origin, destination domain.Location) (int, int, error) {
	distance := haversineMeters(origin, destination)
	return int(float64(distance) / p.DrivingMetersPerSecond), distance, nil
}

func haversineMeters(a, b domain.Location) int {
	const earthRadius = 6_371_000.0
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLng := (b.Lng - a.Lng) * math.Pi / 180
	sinLat := math.Sin(dLat / 2)
	sinLng := math.Sin(dLng / 2)
	h := sinLat*sinLat + math.Cos(lat1)*math.Cos(lat2)*sinLng*sinLng
	return int(2 * earthRadius * math.Asin(math.Sqrt(h)))
}
