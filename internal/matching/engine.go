package matching

import (
	"context"
	"math"
	"sort"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/ports"
)

type Config struct {
	MaxDriverSessionStalenessSeconds int
	ETASafetyBufferSeconds           int
	PickupSearchStepMeters           int
}

type Engine struct {
	routing ports.RoutingProvider
	config  Config
}

func NewEngine(routing ports.RoutingProvider, config Config) *Engine {
	return &Engine{routing: routing, config: config}
}

func (e *Engine) FindCandidates(ctx context.Context, demand domain.TripDemand, rider domain.IdentityVerification, sessionVerifications map[string]domain.IdentityVerification, sessions []domain.DriverSession) ([]domain.MatchCandidate, error) {
	var phaseOne []domain.MatchCandidate
	var phaseTwo []domain.MatchCandidate

	for _, session := range sessions {
		if e.isStale(session) {
			continue
		}
		verification, ok := sessionVerifications[session.DriverID]
		if !ok || !domain.CanDriverServeDemand(session, verification, demand) {
			continue
		}

		exactCandidate, ok, err := e.evaluateCandidate(ctx, demand, session, demand.RequestedOrigin, demand.RequestedDestination, 1)
		if err != nil {
			return nil, err
		}
		if ok {
			phaseOne = append(phaseOne, exactCandidate)
			continue
		}

		walkCandidate, ok, err := e.evaluateWalkingCandidate(ctx, demand, session)
		if err != nil {
			return nil, err
		}
		if ok {
			phaseTwo = append(phaseTwo, walkCandidate)
		}
	}

	sortCandidates(phaseOne)
	if len(phaseOne) > 0 {
		return phaseOne, nil
	}

	sortCandidates(phaseTwo)
	return phaseTwo, nil
}

func (e *Engine) isStale(session domain.DriverSession) bool {
	if e.config.MaxDriverSessionStalenessSeconds <= 0 {
		return false
	}
	cutoff := time.Now().UTC().Add(-time.Duration(e.config.MaxDriverSessionStalenessSeconds) * time.Second)
	return session.LastHeartbeatAt.Before(cutoff)
}

func (e *Engine) evaluateWalkingCandidate(ctx context.Context, demand domain.TripDemand, session domain.DriverSession) (domain.MatchCandidate, bool, error) {
	step := float64(e.config.PickupSearchStepMeters)
	if step <= 0 {
		step = 250
	}

	best := domain.MatchCandidate{}
	found := false
	for _, pickup := range radialCandidates(demand.RequestedOrigin, step, float64(demand.MaxWalkToPickupMeters)) {
		for _, dropoff := range radialCandidates(demand.RequestedDestination, step, float64(demand.MaxWalkFromDropoffMeters)) {
			candidate, ok, err := e.evaluateCandidate(ctx, demand, session, pickup, dropoff, 2)
			if err != nil {
				return domain.MatchCandidate{}, false, err
			}
			if !ok {
				continue
			}
			if !found || compareCandidates(candidate, best) < 0 {
				best = candidate
				found = true
			}
		}
	}

	return best, found, nil
}

func (e *Engine) evaluateCandidate(ctx context.Context, demand domain.TripDemand, session domain.DriverSession, pickup domain.Location, dropoff domain.Location, phase int) (domain.MatchCandidate, bool, error) {
	driverPickupETA, driverPickupDistance, err := e.routing.DrivingETASeconds(ctx, session.CurrentLocation, pickup)
	if err != nil {
		return domain.MatchCandidate{}, false, err
	}

	riderWalkETA, riderWalkDistance, err := e.routing.WalkingETASeconds(ctx, demand.RequestedOrigin, pickup)
	if err != nil {
		return domain.MatchCandidate{}, false, err
	}

	_, riderDropoffWalkDistance, err := e.routing.WalkingETASeconds(ctx, dropoff, demand.RequestedDestination)
	if err != nil {
		return domain.MatchCandidate{}, false, err
	}

	if riderWalkDistance > demand.MaxWalkToPickupMeters || riderDropoffWalkDistance > demand.MaxWalkFromDropoffMeters {
		return domain.MatchCandidate{}, false, nil
	}

	// Implemented from the product intent: the rider may arrive first and wait,
	// but the driver should not have to materially wait at pickup.
	if riderWalkETA > driverPickupETA+e.config.ETASafetyBufferSeconds {
		return domain.MatchCandidate{}, false, nil
	}

	driverDetourMeters := driverPickupDistance
	if driverDetourMeters > session.MaxDriverPickupDetourMeters {
		return domain.MatchCandidate{}, false, nil
	}

	inCarDuration, inCarDistance, err := e.routing.DrivingETASeconds(ctx, pickup, dropoff)
	if err != nil {
		return domain.MatchCandidate{}, false, err
	}

	candidate := domain.MatchCandidate{
		Session:                    session,
		Pickup:                     pickup,
		Dropoff:                    dropoff,
		Phase:                      phase,
		DriverPickupETASeconds:     driverPickupETA,
		RiderWalkETASeconds:        riderWalkETA,
		DriverPickupDetourMeters:   driverDetourMeters,
		RiderWalkToPickupMeters:    riderWalkDistance,
		RiderWalkFromDropoffMeters: riderDropoffWalkDistance,
		RouteOverlapScore:          overlapScore(session, demand),
		InCarDistanceMeters:        inCarDistance,
		InCarDurationSeconds:       inCarDuration,
	}
	return candidate, true, nil
}

func radialCandidates(center domain.Location, stepMeters, maxMeters float64) []domain.Location {
	candidates := []domain.Location{center}
	if maxMeters <= 0 {
		return candidates
	}
	for meters := stepMeters; meters <= maxMeters; meters += stepMeters {
		delta := meters / 111_320.0
		candidates = append(candidates,
			domain.Location{Lat: center.Lat + delta, Lng: center.Lng},
			domain.Location{Lat: center.Lat - delta, Lng: center.Lng},
			domain.Location{Lat: center.Lat, Lng: center.Lng + delta},
			domain.Location{Lat: center.Lat, Lng: center.Lng - delta},
		)
	}
	return candidates
}

func overlapScore(session domain.DriverSession, demand domain.TripDemand) float64 {
	pickupDistance := haversineMeters(session.Origin, demand.RequestedOrigin)
	destinationDistance := haversineMeters(session.Destination, demand.RequestedDestination)
	return 1.0 / float64(1+pickupDistance+destinationDistance)
}

func compareCandidates(a, b domain.MatchCandidate) int {
	switch {
	case a.DriverPickupDetourMeters != b.DriverPickupDetourMeters:
		return a.DriverPickupDetourMeters - b.DriverPickupDetourMeters
	case a.DriverPickupETASeconds != b.DriverPickupETASeconds:
		return a.DriverPickupETASeconds - b.DriverPickupETASeconds
	case a.RiderWalkToPickupMeters+a.RiderWalkFromDropoffMeters != b.RiderWalkToPickupMeters+b.RiderWalkFromDropoffMeters:
		return (a.RiderWalkToPickupMeters + a.RiderWalkFromDropoffMeters) - (b.RiderWalkToPickupMeters + b.RiderWalkFromDropoffMeters)
	case a.RouteOverlapScore > b.RouteOverlapScore:
		return -1
	case a.RouteOverlapScore < b.RouteOverlapScore:
		return 1
	default:
		return b.Session.RemainingCapacity - a.Session.RemainingCapacity
	}
}

func sortCandidates(candidates []domain.MatchCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		return compareCandidates(candidates[i], candidates[j]) < 0
	})
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
