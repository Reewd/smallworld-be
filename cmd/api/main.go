package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"smallworld/internal/application"
	backgroundoffers "smallworld/internal/background/offers"
	backgroundpresence "smallworld/internal/background/presence"
	"smallworld/internal/foundation"
	applog "smallworld/internal/foundation/logging"
	"smallworld/internal/infrastructure/auth"
	"smallworld/internal/infrastructure/postgres"
	"smallworld/internal/infrastructure/pricing"
	"smallworld/internal/infrastructure/push"
	"smallworld/internal/infrastructure/realtime"
	"smallworld/internal/infrastructure/redisstate"
	"smallworld/internal/infrastructure/routing"
	httpapi "smallworld/internal/interfaces/http"
	"smallworld/internal/matching"
	"smallworld/internal/ports"
)

func main() {
	ctx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()

	logConfig, err := applog.LoadConfigFromEnv("smallworld-api")
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging configuration error: %v\n", err)
		os.Exit(1)
	}
	logger := applog.NewLogger(logConfig, nil)

	databaseURL := getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/smallworld?sslmode=disable")
	redisURL := getenv("REDIS_URL", "redis://localhost:6379/0")
	port := getenv("PORT", "8080")
	firebaseProjectID := os.Getenv("FIREBASE_PROJECT_ID")
	firebaseCredentialsFile := os.Getenv("FIREBASE_CREDENTIALS_FILE")
	firebaseAuthEmulatorHost := os.Getenv("FIREBASE_AUTH_EMULATOR_HOST")
	emulatorMode := firebaseAuthEmulatorHost != ""
	routingProvider, routingMode, err := newRoutingProvider()
	if err != nil {
		logger.Error("routing provider configuration failed", "error", err)
		os.Exit(1)
	}

	pool, err := postgres.Open(ctx, databaseURL, postgres.OpenOptions{
		Logger:             logger.With("component", "postgres"),
		SlowQueryThreshold: logConfig.DBSlowQueryThreshold,
		LogAllQueries:      logConfig.DBLogAllQueriesEnabled,
	})
	if err != nil {
		logger.Error("postgres initialization failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	redisClient, err := redisstate.Open(ctx, redisURL)
	if err != nil {
		logger.Error("redis initialization failed", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	idg := &foundation.AtomicIDGenerator{}
	realtimeHub := realtime.NewHub()

	engine := matching.NewEngine(routingProvider, matching.Config{
		MaxDriverSessionStalenessSeconds: 15,
		ETASafetyBufferSeconds:           30,
		PickupSearchStepMeters:           250,
	})

	deps := application.Dependencies{
		Users:         postgres.Users{Pool: pool},
		Verifications: postgres.Verifications{Pool: pool},
		Vehicles:      postgres.Vehicles{Pool: pool},
		Sessions:      postgres.DriverSessions{Pool: pool},
		Demands:       postgres.TripDemands{Pool: pool},
		Offers:        postgres.RideOffers{Pool: pool},
		Bookings:      postgres.RideBookings{Pool: pool},
		Reviews:       postgres.Reviews{Pool: pool},
		Idempotency:   postgres.Idempotency{Pool: pool},
		OfferAcceptor: postgres.OfferAcceptor{Pool: pool},
		DriverPresence: redisstate.DriverPresenceStore{
			Client: redisClient,
			TTL:    30 * time.Second,
		},
		EphemeralOffers: redisstate.EphemeralOfferStore{
			Client: redisClient,
			TTL:    2 * time.Minute,
		},
		Routing:  routingProvider,
		Pricing:  pricing.NewFixedFormula(),
		Push:     push.NoopNotifier{},
		Realtime: realtimeHub,
		IDGen:    idg,
		Matching: engine,
	}
	services := application.NewServices(deps)
	authVerifier, err := newAuthVerifier(ctx, firebaseProjectID, firebaseCredentialsFile, firebaseAuthEmulatorHost)
	if err != nil {
		logger.Error("auth verifier initialization failed", "error", err)
		os.Exit(1)
	}
	logAuthMode(logger, firebaseProjectID, firebaseCredentialsFile, firebaseAuthEmulatorHost)
	logger.Info("routing configured", "mode", routingMode)

	sweeper := backgroundoffers.NewSweeper(
		deps.Offers,
		deps.Demands,
		deps.Sessions,
		deps.Realtime,
		deps.EphemeralOffers,
		backgroundoffers.Config{
			PollInterval:              5 * time.Second,
			PendingOfferTTL:           2 * time.Minute,
			MaxDriverSessionStaleness: 30 * time.Second,
		},
		logger.With("component", "offer_sweeper"),
	)
	presenceReconciler := backgroundpresence.NewReconciler(
		services.DriverSession,
		deps.Realtime,
		backgroundpresence.Config{
			PollInterval:              5 * time.Second,
			MaxDriverSessionStaleness: 30 * time.Second,
		},
		logger.With("component", "presence_reconciler"),
	)

	if os.Getenv("SEED_DEMO_DATA") == "true" {
		if err := postgres.SeedDemoData(ctx, postgres.Users{Pool: pool}, postgres.Verifications{Pool: pool}, postgres.Vehicles{Pool: pool}); err != nil {
			logger.Error("demo seed failed", "error", err)
			os.Exit(1)
		}
		logger.Info("demo data seeded")
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.NewServer(services, authVerifier, realtimeHub, emulatorMode, logger.With("component", "http")).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("http server listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server exited unexpectedly", "error", err)
			os.Exit(1)
		}
	}()
	go sweeper.Run(ctx)
	go presenceReconciler.Run(ctx)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancelWorkers()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("http server shutdown failed", "error", err)
		return
	}
	logger.Info("http server shutdown complete")
}

func newAuthVerifier(ctx context.Context, projectID, credentialsFile, authEmulatorHost string) (ports.AuthVerifier, error) {
	switch {
	case projectID == "":
		return nil, errors.New("FIREBASE_PROJECT_ID is required")
	case authEmulatorHost == "" && credentialsFile == "":
		return nil, errors.New("FIREBASE_CREDENTIALS_FILE is required unless FIREBASE_AUTH_EMULATOR_HOST is set")
	}

	return auth.NewFirebaseVerifier(ctx, auth.FirebaseConfig{
		ProjectID:        projectID,
		CredentialsFile:  credentialsFile,
		AuthEmulatorHost: authEmulatorHost,
	})
}

func newRoutingProvider() (ports.RoutingProvider, string, error) {
	apiKey := os.Getenv("GOOGLE_MAPS_API_KEY")
	if apiKey == "" {
		return routing.NewHaversineProvider(), "haversine fallback", nil
	}

	provider, err := routing.NewGoogleRoutesProvider(routing.GoogleRoutesConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, "", err
	}
	return provider, "google routes api", nil
}

func logAuthMode(logger interface {
	Info(string, ...any)
}, projectID, credentialsFile, authEmulatorHost string) {
	switch {
	case authEmulatorHost != "":
		logger.Info("auth configured", "mode", "firebase_emulator", "project_id", projectID, "emulator_host", authEmulatorHost)
	case projectID != "" && credentialsFile != "":
		logger.Info("auth configured", "mode", "firebase_production", "project_id", projectID)
	default:
		logger.Info("auth configured", "mode", "incomplete")
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
