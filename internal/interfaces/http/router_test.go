package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
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
