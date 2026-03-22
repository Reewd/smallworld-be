package offers

import (
	"context"
	"testing"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/infrastructure/memory"
)

type noopRealtime struct{}

func (noopRealtime) PublishToUser(context.Context, string, string, any) error { return nil }

type trackingEphemeral struct {
	deleted []string
}

func (t *trackingEphemeral) SavePendingOffer(context.Context, domain.RideOffer) error { return nil }
func (t *trackingEphemeral) DeletePendingOffer(_ context.Context, offerID string) error {
	t.deleted = append(t.deleted, offerID)
	return nil
}

func TestSweeperExpiresTimedOutOffersAndResetsDemand(t *testing.T) {
	store := memory.NewStore()
	now := time.Now().UTC()
	offer := domain.RideOffer{
		ID:              "offer_1",
		DemandID:        "td_1",
		DriverSessionID: "ds_1",
		State:           domain.RideOfferStatePending,
		CreatedAt:       now.Add(-3 * time.Minute),
		UpdatedAt:       now.Add(-3 * time.Minute),
	}
	demand := domain.TripDemand{
		ID:        "td_1",
		RiderID:   "rider_1",
		State:     domain.TripDemandStateOffered,
		UpdatedAt: now.Add(-3 * time.Minute),
	}
	session := domain.DriverSession{
		ID:              "ds_1",
		DriverID:        "driver_1",
		State:           domain.DriverSessionStateActive,
		LastHeartbeatAt: now,
	}

	if err := store.SaveRideOffer(context.Background(), offer); err != nil {
		t.Fatalf("save offer: %v", err)
	}
	if err := store.SaveTripDemand(context.Background(), demand); err != nil {
		t.Fatalf("save demand: %v", err)
	}
	if err := store.SaveDriverSession(context.Background(), session); err != nil {
		t.Fatalf("save session: %v", err)
	}

	ephemeral := &trackingEphemeral{}
	sweeper := NewSweeper(
		memory.RideOffers{Store: store},
		memory.TripDemands{Store: store},
		memory.DriverSessions{Store: store},
		noopRealtime{},
		ephemeral,
		Config{
			PollInterval:              time.Second,
			PendingOfferTTL:           2 * time.Minute,
			MaxDriverSessionStaleness: 30 * time.Second,
		},
		nil,
	)

	if err := sweeper.SweepOnce(context.Background()); err != nil {
		t.Fatalf("sweep once: %v", err)
	}

	updatedOffer, err := store.FindRideOfferByID(context.Background(), "offer_1")
	if err != nil {
		t.Fatalf("find updated offer: %v", err)
	}
	if updatedOffer.State != domain.RideOfferStateExpired {
		t.Fatalf("expected offer to be expired, got %s", updatedOffer.State)
	}

	updatedDemand, err := store.FindTripDemandByID(context.Background(), "td_1")
	if err != nil {
		t.Fatalf("find updated demand: %v", err)
	}
	if updatedDemand.State != domain.TripDemandStateSearching {
		t.Fatalf("expected demand to return to searching, got %s", updatedDemand.State)
	}

	if len(ephemeral.deleted) != 1 || ephemeral.deleted[0] != "offer_1" {
		t.Fatalf("expected ephemeral offer deletion for offer_1, got %#v", ephemeral.deleted)
	}
}
