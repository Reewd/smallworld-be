package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"smallworld/internal/application"
	backgroundoffers "smallworld/internal/background/offers"
	backgroundpresence "smallworld/internal/background/presence"
	"smallworld/internal/foundation"
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

	databaseURL := getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/smallworld?sslmode=disable")
	redisURL := getenv("REDIS_URL", "redis://localhost:6379/0")
	port := getenv("PORT", "8080")
	firebaseProjectID := os.Getenv("FIREBASE_PROJECT_ID")
	firebaseCredentialsFile := os.Getenv("FIREBASE_CREDENTIALS_FILE")
	routingProvider, routingMode, err := newRoutingProvider()
	if err != nil {
		log.Fatal(err)
	}

	pool, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	redisClient, err := redisstate.Open(ctx, redisURL)
	if err != nil {
		log.Fatal(err)
	}
	defer redisClient.Close()

	idg := &foundation.AtomicIDGenerator{}

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
		Realtime: realtime.NoopHub{},
		IDGen:    idg,
		Matching: engine,
	}
	services := application.NewServices(deps)
	authVerifier, err := newAuthVerifier(ctx, firebaseProjectID, firebaseCredentialsFile)
	if err != nil {
		log.Fatal(err)
	}
	logAuthMode(firebaseProjectID, firebaseCredentialsFile, os.Getenv("ALLOW_DEV_AUTH") == "true")
	log.Printf("routing mode: %s", routingMode)

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
		log.Default(),
	)
	presenceReconciler := backgroundpresence.NewReconciler(
		services.DriverSession,
		deps.Realtime,
		backgroundpresence.Config{
			PollInterval:              5 * time.Second,
			MaxDriverSessionStaleness: 30 * time.Second,
		},
		log.Default(),
	)

	if os.Getenv("SEED_DEMO_DATA") == "true" {
		if err := postgres.SeedDemoData(ctx, postgres.Users{Pool: pool}, postgres.Verifications{Pool: pool}, postgres.Vehicles{Pool: pool}); err != nil {
			log.Fatal(err)
		}
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.NewServer(services, authVerifier).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("smallworld api listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
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
		log.Printf("shutdown error: %v", err)
	}
}

func newAuthVerifier(ctx context.Context, projectID, credentialsFile string) (ports.AuthVerifier, error) {
	var verifiers []ports.AuthVerifier

	if projectID != "" || credentialsFile != "" {
		firebaseVerifier, err := auth.NewFirebaseVerifier(ctx, auth.FirebaseConfig{
			ProjectID:       projectID,
			CredentialsFile: credentialsFile,
		})
		if err != nil {
			return nil, err
		}
		verifiers = append(verifiers, firebaseVerifier)
	}

	if len(verifiers) == 0 || os.Getenv("ALLOW_DEV_AUTH") == "true" {
		verifiers = append(verifiers, auth.DevVerifier{})
	}

	if len(verifiers) == 1 {
		return verifiers[0], nil
	}
	return auth.CompositeVerifier{Verifiers: verifiers}, nil
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

func logAuthMode(projectID, credentialsFile string, allowDevAuth bool) {
	switch {
	case (projectID != "" || credentialsFile != "") && allowDevAuth:
		log.Printf("auth mode: firebase enabled with dev auth fallback")
	case projectID != "" || credentialsFile != "":
		log.Printf("auth mode: firebase only")
	case allowDevAuth:
		log.Printf("auth mode: dev auth only")
	default:
		log.Printf("auth mode: no explicit provider configured, dev auth fallback enabled")
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
