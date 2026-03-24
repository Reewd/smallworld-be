package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"smallworld/internal/application"
	"smallworld/internal/application/service"
	"smallworld/internal/domain"
	"smallworld/internal/foundation"
	"smallworld/internal/infrastructure/memory"
	"smallworld/internal/infrastructure/routing"
	"smallworld/internal/matching"
	"smallworld/internal/ports"
)

func TestDevBootstrapRouteDisabledOutsideEmulatorMode(t *testing.T) {
	server := NewServer(newTestServices(t), staticAuthVerifier{}, &fakeWebSocketHub{}, false)
	req := httptest.NewRequest(http.MethodPost, "/v1/dev/me/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWebSocketRouteRequiresExistingBackendUser(t *testing.T) {
	server := NewServer(newTestServices(t), staticAuthVerifier{}, &fakeWebSocketHub{}, true)
	req := httptest.NewRequest(http.MethodGet, "/v1/ws", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWebSocketRoutePassesResolvedUserIDToHub(t *testing.T) {
	services := newTestServices(t)
	user := domain.User{
		ID:          "user_1",
		AuthSubject: "auth-subject",
		DisplayName: "Andrea",
		Preferences: domain.UserPreferences{MaxWalkToPickupMeters: 300, MaxWalkFromDropoffMeters: 300, MaxDriverPickupDetourMeters: 1000},
	}
	savedUser, err := services.Profile.UpsertAuthenticated(context.Background(), "auth-subject", structToProfileInput(user))
	if err != nil {
		t.Fatalf("upsert profile: %v", err)
	}

	hub := fakeWebSocketHub{}
	server := NewServer(services, staticAuthVerifier{}, &hub, true)
	req := httptest.NewRequest(http.MethodGet, "/v1/ws", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if hub.userID != savedUser.ID {
		t.Fatalf("hub.userID = %q", hub.userID)
	}
}

func TestTripDemandCreateReturnsStreamlinedServiceError(t *testing.T) {
	store := memory.NewStore()
	idg := &foundation.AtomicIDGenerator{}
	services := application.NewServices(application.Dependencies{
		Users:           memory.Users{Store: store},
		Verifications:   memory.Verifications{Store: store},
		Vehicles:        memory.Vehicles{Store: store},
		Sessions:        memory.DriverSessions{Store: store},
		Demands:         memory.TripDemands{Store: store},
		Offers:          memory.RideOffers{Store: store},
		Bookings:        memory.RideBookings{Store: store},
		Reviews:         memory.Reviews{Store: store},
		Idempotency:     memory.Idempotency{Store: store},
		DriverPresence:  nil,
		EphemeralOffers: nil,
		Routing:         routing.NewHaversineProvider(),
		Pricing:         nil,
		Push:            nil,
		Realtime:        nil,
		IDGen:           idg,
		Matching: matching.NewEngine(routing.NewHaversineProvider(), matching.Config{
			MaxDriverSessionStalenessSeconds: 15,
			ETASafetyBufferSeconds:           30,
			PickupSearchStepMeters:           250,
		}),
	})

	user, err := services.Profile.UpsertAuthenticated(context.Background(), "auth-subject", service.UpsertProfileInput{
		DisplayName: "Andrea",
		Preferences: domain.UserPreferences{
			MaxWalkToPickupMeters:       300,
			MaxWalkFromDropoffMeters:    300,
			MaxDriverPickupDetourMeters: 1000,
		},
	})
	if err != nil {
		t.Fatalf("upsert profile: %v", err)
	}
	if err := store.SaveVerification(context.Background(), domain.IdentityVerification{
		UserID:         user.ID,
		Status:         domain.VerificationVerified,
		VerifiedGender: domain.GenderMale,
	}); err != nil {
		t.Fatalf("save verification: %v", err)
	}

	server := NewServer(services, staticAuthVerifier{}, &fakeWebSocketHub{}, true)
	req := httptest.NewRequest(http.MethodPost, "/v1/trip-demands", strings.NewReader(`{
		"requested_origin":{"lat":45.0,"lng":9.0},
		"requested_destination":{"lat":45.01,"lng":9.01},
		"women_drivers_only":true,
		"max_walk_to_pickup_meters":300,
		"max_walk_from_dropoff_meters":300
	}`))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != domain.ErrWomenOnlyRequiresVerifiedFemaleRider.Error() {
		t.Fatalf("unexpected error message: %q", body["error"])
	}
}

func TestResolveServiceErrorHidesUnknownInternalMessage(t *testing.T) {
	resolved := resolveServiceError(errors.New("database connection exploded"), http.StatusInternalServerError)

	if resolved.status != http.StatusInternalServerError {
		t.Fatalf("status = %d", resolved.status)
	}
	if resolved.message != "internal server error" {
		t.Fatalf("message = %q", resolved.message)
	}
}

type staticAuthVerifier struct{}

func (staticAuthVerifier) VerifyToken(context.Context, string) (ports.AuthIdentity, error) {
	return ports.AuthIdentity{Subject: "auth-subject", Provider: "firebase"}, nil
}

type fakeWebSocketHub struct {
	userID string
}

func (h *fakeWebSocketHub) ServeHTTP(w http.ResponseWriter, _ *http.Request, userID string) {
	h.userID = userID
	w.WriteHeader(http.StatusSwitchingProtocols)
}

func newTestServices(t *testing.T) application.Services {
	t.Helper()

	store := memory.NewStore()
	idg := &foundation.AtomicIDGenerator{}
	return application.NewServices(application.Dependencies{
		Users:           memory.Users{Store: store},
		Verifications:   memory.Verifications{Store: store},
		Vehicles:        memory.Vehicles{Store: store},
		Sessions:        memory.DriverSessions{Store: store},
		Demands:         memory.TripDemands{Store: store},
		Offers:          memory.RideOffers{Store: store},
		Bookings:        memory.RideBookings{Store: store},
		Reviews:         memory.Reviews{Store: store},
		Idempotency:     memory.Idempotency{Store: store},
		DriverPresence:  nil,
		EphemeralOffers: nil,
		Routing:         routing.NewHaversineProvider(),
		Pricing:         nil,
		Push:            nil,
		Realtime:        nil,
		IDGen:           idg,
		Matching: matching.NewEngine(routing.NewHaversineProvider(), matching.Config{
			MaxDriverSessionStalenessSeconds: 15,
			ETASafetyBufferSeconds:           30,
			PickupSearchStepMeters:           250,
		}),
	})
}

func structToProfileInput(user domain.User) service.UpsertProfileInput {
	return service.UpsertProfileInput{
		DisplayName: user.DisplayName,
		Preferences: user.Preferences,
	}
}
