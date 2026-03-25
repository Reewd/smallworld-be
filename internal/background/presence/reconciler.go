package presence

import (
	"context"
	"io"
	"log/slog"
	"time"

	"smallworld/internal/application/service"
	"smallworld/internal/ports"
)

type Config struct {
	PollInterval              time.Duration
	MaxDriverSessionStaleness time.Duration
}

type Reconciler struct {
	driverSessions *service.DriverSessionService
	realtime       ports.RealtimeHub
	config         Config
	logger         *slog.Logger
}

func NewReconciler(driverSessions *service.DriverSessionService, realtime ports.RealtimeHub, config Config, logger *slog.Logger) *Reconciler {
	if config.PollInterval <= 0 {
		config.PollInterval = 5 * time.Second
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Reconciler{
		driverSessions: driverSessions,
		realtime:       realtime,
		config:         config,
		logger:         logger,
	}
}

func (r *Reconciler) Run(ctx context.Context) {
	ticker := time.NewTicker(r.config.PollInterval)
	defer ticker.Stop()

	for {
		if err := r.ReconcileOnce(ctx); err != nil {
			r.logger.ErrorContext(ctx, "driver presence reconciliation failed", "error", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Reconciler) ReconcileOnce(ctx context.Context) error {
	paused, err := r.driverSessions.ReconcileStaleSessions(ctx, r.config.MaxDriverSessionStaleness)
	if err != nil {
		return err
	}
	for _, session := range paused {
		if r.realtime != nil {
			_ = r.realtime.PublishToUser(ctx, session.DriverID, "driver_session.paused", session)
		}
	}
	if len(paused) > 0 {
		r.logger.DebugContext(ctx, "paused stale driver sessions", "count", len(paused))
	}
	return nil
}
