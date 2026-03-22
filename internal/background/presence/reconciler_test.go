package presence

import (
	"context"
	"testing"
	"time"

	"smallworld/internal/application/service"
	"smallworld/internal/domain"
	"smallworld/internal/foundation"
	"smallworld/internal/infrastructure/memory"
	"smallworld/internal/infrastructure/realtime"
	"smallworld/internal/infrastructure/routing"
)

func TestReconcilePausesStaleSessions(t *testing.T) {
	store := memory.NewStore()
	now := time.Now().UTC()

	if err := store.SaveDriverSession(context.Background(), domain.DriverSession{
		ID:                          "ds_1",
		DriverID:                    "driver_1",
		VehicleID:                   "veh_1",
		State:                       domain.DriverSessionStateActive,
		LastHeartbeatAt:             now.Add(-time.Minute),
		UpdatedAt:                   now.Add(-time.Minute),
		MaxDriverPickupDetourMeters: 500,
	}); err != nil {
		t.Fatalf("save driver session: %v", err)
	}

	svc := service.NewDriverSessionService(
		memory.DriverSessions{Store: store},
		memory.Verifications{Store: store},
		memory.Vehicles{Store: store},
		routing.NewHaversineProvider(),
		memory.Idempotency{Store: store},
		nil,
		&foundation.AtomicIDGenerator{},
	)

	reconciler := NewReconciler(svc, realtime.NoopHub{}, Config{
		PollInterval:              time.Second,
		MaxDriverSessionStaleness: 30 * time.Second,
	}, nil)

	if err := reconciler.ReconcileOnce(context.Background()); err != nil {
		t.Fatalf("reconcile once: %v", err)
	}

	session, err := store.FindDriverSessionByID(context.Background(), "ds_1")
	if err != nil {
		t.Fatalf("find driver session: %v", err)
	}
	if session.State != domain.DriverSessionStatePaused {
		t.Fatalf("expected paused session, got %s", session.State)
	}
}
