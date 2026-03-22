package pricing

import (
	"context"

	"smallworld/internal/domain"
)

type FixedFormula struct {
	BaseFareCents     int
	PerKilometerCents int
	PerMinuteCents    int
	MinimumFareCents  int
}

func NewFixedFormula() *FixedFormula {
	return &FixedFormula{
		BaseFareCents:     200,
		PerKilometerCents: 80,
		PerMinuteCents:    20,
		MinimumFareCents:  500,
	}
}

func (f *FixedFormula) Quote(_ context.Context, candidate domain.MatchCandidate) (int, error) {
	fare := f.BaseFareCents + (candidate.InCarDistanceMeters/1000)*f.PerKilometerCents + (candidate.InCarDurationSeconds/60)*f.PerMinuteCents
	if fare < f.MinimumFareCents {
		fare = f.MinimumFareCents
	}
	return fare, nil
}
