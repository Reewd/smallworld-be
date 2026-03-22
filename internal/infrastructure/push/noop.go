package push

import (
	"context"

	"smallworld/internal/domain"
)

type NoopNotifier struct{}

func (NoopNotifier) SendDriverOffer(context.Context, string, domain.RideOffer) error { return nil }
func (NoopNotifier) SendRiderMatched(context.Context, string, domain.RideBooking) error {
	return nil
}
