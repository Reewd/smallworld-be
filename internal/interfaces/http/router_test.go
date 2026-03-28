package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
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

func TestAuthMeReturnsNeedsProfileForAuthenticatedUserWithoutBackendProfile(t *testing.T) {
	server := NewServer(newTestServices(t), staticAuthVerifier{}, &fakeWebSocketHub{}, true, discardLogger())
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Auth struct {
			Subject  string `json:"subject"`
			Provider string `json:"provider"`
		} `json:"auth"`
		User            *domain.User                 `json:"user"`
		Verification    *domain.IdentityVerification `json:"verification"`
		OnboardingState string                       `json:"onboarding_state"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Auth.Subject != "auth-subject" || body.Auth.Provider != "firebase" {
		t.Fatalf("auth = %#v", body.Auth)
	}
	if body.User != nil {
		t.Fatalf("expected nil user, got %#v", body.User)
	}
	if body.Verification != nil {
		t.Fatalf("expected nil verification, got %#v", body.Verification)
	}
	if body.OnboardingState != string(onboardingStateNeedsProfile) {
		t.Fatalf("onboarding_state = %q", body.OnboardingState)
	}
}

func TestAuthMeReturnsNeedsVerificationWhenUserExistsWithoutVerification(t *testing.T) {
	services, _ := newTestServicesWithStore(t)
	if _, err := services.Profile.UpsertAuthenticated(context.Background(), "auth-subject", service.UpsertProfileInput{
		DisplayName: "Andrea",
		Preferences: domain.UserPreferences{
			WalkToPickup:       domain.PreferenceLevelMedium,
			WalkFromDropoff:    domain.PreferenceLevelMedium,
			DriverPickupDetour: domain.PreferenceLevelMedium,
		},
	}); err != nil {
		t.Fatalf("upsert profile: %v", err)
	}

	server := NewServer(services, staticAuthVerifier{}, &fakeWebSocketHub{}, true, discardLogger())
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		User            *domain.User                 `json:"user"`
		Verification    *domain.IdentityVerification `json:"verification"`
		OnboardingState string                       `json:"onboarding_state"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.User == nil {
		t.Fatalf("expected user to be populated")
	}
	if body.Verification != nil {
		t.Fatalf("expected nil verification, got %#v", body.Verification)
	}
	if body.OnboardingState != string(onboardingStateNeedsVerification) {
		t.Fatalf("onboarding_state = %q", body.OnboardingState)
	}
}

func TestAuthMeReturnsVerificationStateForExistingUser(t *testing.T) {
	tests := []struct {
		name          string
		verification  domain.IdentityVerification
		expectedState string
	}{
		{
			name: "pending",
			verification: domain.IdentityVerification{
				Status:         domain.VerificationPending,
				Provider:       "kyc_vendor",
				ProviderRef:    "ref_1",
				VerifiedGender: domain.GenderUnknown,
			},
			expectedState: string(onboardingStateVerificationPending),
		},
		{
			name: "verified",
			verification: domain.IdentityVerification{
				Status:         domain.VerificationVerified,
				Provider:       "kyc_vendor",
				ProviderRef:    "ref_2",
				VerifiedGender: domain.GenderFemale,
			},
			expectedState: string(onboardingStateReady),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, store := newTestServicesWithStore(t)
			user, err := services.Profile.UpsertAuthenticated(context.Background(), "auth-subject", service.UpsertProfileInput{
				DisplayName: "Andrea",
				Preferences: domain.UserPreferences{
					WalkToPickup:       domain.PreferenceLevelMedium,
					WalkFromDropoff:    domain.PreferenceLevelMedium,
					DriverPickupDetour: domain.PreferenceLevelMedium,
				},
			})
			if err != nil {
				t.Fatalf("upsert profile: %v", err)
			}

			verification := tt.verification
			verification.UserID = user.ID
			if err := store.SaveVerification(context.Background(), verification); err != nil {
				t.Fatalf("save verification: %v", err)
			}

			server := NewServer(services, staticAuthVerifier{}, &fakeWebSocketHub{}, true, discardLogger())
			req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
			req.Header.Set("Authorization", "Bearer test-token")
			rec := httptest.NewRecorder()

			server.Routes().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}

			var body struct {
				Verification    *domain.IdentityVerification `json:"verification"`
				OnboardingState string                       `json:"onboarding_state"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if body.Verification == nil {
				t.Fatalf("expected verification to be populated")
			}
			if body.Verification.Status != verification.Status {
				t.Fatalf("verification.status = %q", body.Verification.Status)
			}
			if body.OnboardingState != tt.expectedState {
				t.Fatalf("onboarding_state = %q", body.OnboardingState)
			}
		})
	}
}

func TestDevBootstrapRouteDisabledOutsideEmulatorMode(t *testing.T) {
	server := NewServer(newTestServices(t), staticAuthVerifier{}, &fakeWebSocketHub{}, false, discardLogger())
	req := httptest.NewRequest(http.MethodPost, "/v1/dev/me/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWebSocketRouteRequiresExistingBackendUser(t *testing.T) {
	server := NewServer(newTestServices(t), staticAuthVerifier{}, &fakeWebSocketHub{}, true, discardLogger())
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
		Preferences: domain.UserPreferences{
			WalkToPickup:       domain.PreferenceLevelMedium,
			WalkFromDropoff:    domain.PreferenceLevelMedium,
			DriverPickupDetour: domain.PreferenceLevelMedium,
		},
	}
	savedUser, err := services.Profile.UpsertAuthenticated(context.Background(), "auth-subject", structToProfileInput(user))
	if err != nil {
		t.Fatalf("upsert profile: %v", err)
	}

	hub := fakeWebSocketHub{}
	server := NewServer(services, staticAuthVerifier{}, &hub, true, discardLogger())
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
			WalkToPickup:       domain.PreferenceLevelMedium,
			WalkFromDropoff:    domain.PreferenceLevelMedium,
			DriverPickupDetour: domain.PreferenceLevelMedium,
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

	server := NewServer(services, staticAuthVerifier{}, &fakeWebSocketHub{}, true, discardLogger())
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

func TestRequestLoggingIncludesDebugStartAndResolvedUserID(t *testing.T) {
	services := newTestServices(t)
	user, err := services.Profile.UpsertAuthenticated(context.Background(), "auth-subject", service.UpsertProfileInput{
		DisplayName: "Andrea",
		Preferences: domain.UserPreferences{
			WalkToPickup:       domain.PreferenceLevelMedium,
			WalkFromDropoff:    domain.PreferenceLevelMedium,
			DriverPickupDetour: domain.PreferenceLevelMedium,
		},
	})
	if err != nil {
		t.Fatalf("upsert profile: %v", err)
	}

	logger, buf := bufferLogger(slog.LevelDebug)
	server := NewServer(services, staticAuthVerifier{}, &fakeWebSocketHub{}, true, logger)
	req := httptest.NewRequest(http.MethodGet, "/v1/profile/me", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	output := buf.String()
	if !strings.Contains(output, `"msg":"http request started"`) {
		t.Fatalf("expected debug start log, got %s", output)
	}
	if !strings.Contains(output, `"msg":"http request completed"`) {
		t.Fatalf("expected completion log, got %s", output)
	}
	if !strings.Contains(output, `"user_id":"`+user.ID+`"`) {
		t.Fatalf("expected user_id in completion log, got %s", output)
	}
}

func TestAuthFailureLoggingDoesNotLeakBearerToken(t *testing.T) {
	logger, buf := bufferLogger(slog.LevelDebug)
	server := NewServer(newTestServices(t), rejectingAuthVerifier{}, &fakeWebSocketHub{}, true, logger)
	req := httptest.NewRequest(http.MethodGet, "/v1/profile/me", nil)
	req.Header.Set("Authorization", "Bearer super-secret-token")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	output := buf.String()
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(output, "invalid bearer token") {
		t.Fatalf("expected auth failure log, got %s", output)
	}
	if strings.Contains(output, "super-secret-token") {
		t.Fatalf("expected bearer token to stay out of logs, got %s", output)
	}
}

func TestRequestLoggingDoesNotLeakRequestBody(t *testing.T) {
	logger, buf := bufferLogger(slog.LevelDebug)
	server := NewServer(newTestServices(t), staticAuthVerifier{}, &fakeWebSocketHub{}, true, logger)
	req := httptest.NewRequest(http.MethodPost, "/v1/profile", strings.NewReader(`{"display_name":"super-secret-body"`))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	output := buf.String()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(output, "super-secret-body") {
		t.Fatalf("expected request body to stay out of logs, got %s", output)
	}
}

func TestProfileUpsertRejectsInvalidPreferenceLevel(t *testing.T) {
	server := NewServer(newTestServices(t), staticAuthVerifier{}, &fakeWebSocketHub{}, true, discardLogger())
	req := httptest.NewRequest(http.MethodPost, "/v1/profile", strings.NewReader(`{
		"display_name":"Andrea",
		"preferences":{
			"walk_to_pickup":"extreme",
			"walk_from_dropoff":"medium",
			"driver_pickup_detour":"medium"
		}
	}`))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestInternalServerErrorsLogAtErrorLevel(t *testing.T) {
	logger, buf := bufferLogger(slog.LevelDebug)
	services := application.Services{
		Profile: service.NewProfileService(failingUsers{}, &foundation.AtomicIDGenerator{}),
	}
	server := NewServer(services, staticAuthVerifier{}, &fakeWebSocketHub{}, true, logger)
	req := httptest.NewRequest(http.MethodGet, "/v1/profile/me", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	output := buf.String()
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(output, `"level":"ERROR"`) {
		t.Fatalf("expected error-level logs, got %s", output)
	}
	if !strings.Contains(output, `"error":"internal server error"`) {
		t.Fatalf("expected generic client-facing 500 message in request log, got %s", output)
	}
}

type staticAuthVerifier struct{}

func (staticAuthVerifier) VerifyToken(context.Context, string) (ports.AuthIdentity, error) {
	return ports.AuthIdentity{Subject: "auth-subject", Provider: "firebase"}, nil
}

type rejectingAuthVerifier struct{}

func (rejectingAuthVerifier) VerifyToken(context.Context, string) (ports.AuthIdentity, error) {
	return ports.AuthIdentity{}, errors.New("invalid token")
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

	services, _ := newTestServicesWithStore(t)
	return services
}

func newTestServicesWithStore(t *testing.T) (application.Services, *memory.Store) {
	t.Helper()

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
	return services, store
}

func structToProfileInput(user domain.User) service.UpsertProfileInput {
	return service.UpsertProfileInput{
		DisplayName: user.DisplayName,
		Preferences: user.Preferences,
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func bufferLogger(level slog.Level) (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: level})), &buf
}

type failingUsers struct{}

func (failingUsers) Save(context.Context, domain.User) error { return errors.New("save exploded") }
func (failingUsers) FindByID(context.Context, string) (domain.User, error) {
	return domain.User{}, errors.New("find by id exploded")
}
func (failingUsers) FindByAuthSubject(context.Context, string) (domain.User, error) {
	return domain.User{}, errors.New("database exploded")
}
