package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"smallworld/internal/application"
	"smallworld/internal/application/service"
	"smallworld/internal/domain"
	"smallworld/internal/ports"
)

type Server struct {
	services           application.Services
	authVerifier       ports.AuthVerifier
	webSocketHub       webSocketHub
	enableDevBootstrap bool
	logger             *slog.Logger
}

type webSocketHub interface {
	ServeHTTP(http.ResponseWriter, *http.Request, string)
}

func NewServer(services application.Services, authVerifier ports.AuthVerifier, webSocketHub webSocketHub, enableDevBootstrap bool, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Server{
		services:           services,
		authVerifier:       authVerifier,
		webSocketHub:       webSocketHub,
		enableDevBootstrap: enableDevBootstrap,
		logger:             logger,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /v1/ws", s.handleWebSocket)
	mux.HandleFunc("GET /v1/auth/me", s.handleAuthMe)
	if s.enableDevBootstrap {
		mux.HandleFunc("POST /v1/dev/me/bootstrap", s.handleDevBootstrap)
	}
	mux.HandleFunc("GET /v1/me/driver-session", s.handleCurrentDriverSession)
	mux.HandleFunc("GET /v1/me/trip-demand", s.handleCurrentTripDemand)
	mux.HandleFunc("GET /v1/me/ride-offers", s.handleCurrentRideOffers)
	mux.HandleFunc("GET /v1/me/bookings", s.handleCurrentBookings)
	mux.HandleFunc("GET /v1/profile/me", s.handleProfileMe)
	mux.HandleFunc("POST /v1/profile", s.handleProfileUpsert)
	mux.HandleFunc("GET /v1/vehicles", s.handleVehicleList)
	mux.HandleFunc("POST /v1/vehicles", s.handleVehicleCreate)
	mux.HandleFunc("POST /v1/driver-sessions", s.handleDriverSessionCreate)
	mux.HandleFunc("GET /v1/driver-sessions/", s.handleDriverSessionRead)
	mux.HandleFunc("POST /v1/driver-sessions/", s.handleDriverSessionActions)
	mux.HandleFunc("POST /v1/trip-demands", s.handleTripDemandCreate)
	mux.HandleFunc("GET /v1/trip-demands/", s.handleTripDemandRead)
	mux.HandleFunc("POST /v1/trip-demands/", s.handleTripDemandActions)
	mux.HandleFunc("POST /v1/ride-offers/", s.handleOfferActions)
	mux.HandleFunc("GET /v1/bookings/", s.handleBookingRead)
	mux.HandleFunc("POST /v1/bookings/", s.handleBookingActions)
	mux.HandleFunc("GET /v1/users/", s.handleUserReviewList)
	return s.authMiddleware(s.loggingMiddleware(mux))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	if s.webSocketHub == nil {
		writeErrorMessage(w, http.StatusServiceUnavailable, "realtime unavailable")
		return
	}
	s.webSocketHub.ServeHTTP(w, r, userID)
}

func (s *Server) handleDevBootstrap(w http.ResponseWriter, r *http.Request) {
	identity, err := currentIdentity(r)
	if err != nil {
		writeServiceError(w, err, http.StatusUnauthorized)
		return
	}

	var body struct {
		DisplayName    string                            `json:"display_name"`
		VerifiedGender domain.Gender                     `json:"verified_gender"`
		Vehicle        *service.DevBootstrapVehicleInput `json:"vehicle"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}

	result, err := s.services.DevBootstrap.Bootstrap(r.Context(), service.DevBootstrapInput{
		AuthSubject:    identity.Subject,
		DisplayName:    body.DisplayName,
		VerifiedGender: body.VerifiedGender,
		Vehicle:        body.Vehicle,
	})
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCurrentDriverSession(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	session, err := s.services.DriverSession.GetCurrentForDriver(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleCurrentTripDemand(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	demand, err := s.services.TripDemand.GetCurrentForRider(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, demand)
}

func (s *Server) handleCurrentRideOffers(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	offers, err := s.services.Offer.ListPendingForDriver(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, offers)
}

func (s *Server) handleCurrentBookings(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	bookings, err := s.services.Booking.ListActiveForActor(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, bookings)
}

func (s *Server) handleProfileUpsert(w http.ResponseWriter, r *http.Request) {
	identity, err := currentIdentity(r)
	if err != nil {
		writeServiceError(w, err, http.StatusUnauthorized)
		return
	}

	var input service.UpsertProfileInput
	if !decodeJSONBody(w, r, &input) {
		return
	}
	user, err := s.services.Profile.UpsertAuthenticated(r.Context(), identity.Subject, input)
	if err != nil {
		writeServiceError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleProfileMe(w http.ResponseWriter, r *http.Request) {
	identity, err := currentIdentity(r)
	if err != nil {
		writeServiceError(w, err, http.StatusUnauthorized)
		return
	}
	user, err := s.services.Profile.FindByAuthSubject(r.Context(), identity.Subject)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleVehicleCreate(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	var input service.CreateVehicleInput
	if !decodeJSONBody(w, r, &input) {
		return
	}
	input.UserID = userID
	vehicle, err := s.services.Vehicle.Create(r.Context(), input)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, vehicle)
}

func (s *Server) handleVehicleList(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	vehicles, err := s.services.Vehicle.ListByUserID(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, vehicles)
}

func (s *Server) handleDriverSessionCreate(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	var input service.StartDriverSessionInput
	if !decodeJSONBody(w, r, &input) {
		return
	}
	input.UserID = userID
	session, err := s.services.DriverSession.Start(r.Context(), input)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (s *Server) handleDriverSessionRead(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	sessionID := strings.TrimPrefix(r.URL.Path, "/v1/driver-sessions/")
	if strings.Contains(sessionID, "/") {
		http.NotFound(w, r)
		return
	}
	session, err := s.services.DriverSession.GetOwned(r.Context(), userID, sessionID)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleDriverSessionActions(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/driver-sessions/")
	switch {
	case strings.HasSuffix(path, "/heartbeat"):
		sessionID := strings.TrimSuffix(path, "/heartbeat")
		var body struct {
			CurrentLocation domain.Location `json:"current_location"`
		}
		if !decodeJSONBody(w, r, &body) {
			return
		}
		session, err := s.services.DriverSession.HeartbeatOwned(r.Context(), userID, service.HeartbeatDriverSessionInput{
			SessionID:       sessionID,
			CurrentLocation: body.CurrentLocation,
		})
		if err != nil {
			writeServiceError(w, err, http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, session)
	case strings.HasSuffix(path, "/state"):
		sessionID := strings.TrimSuffix(path, "/state")
		var body struct {
			State domain.DriverSessionState `json:"state"`
		}
		if !decodeJSONBody(w, r, &body) {
			return
		}
		session, err := s.services.DriverSession.TransitionStateOwned(r.Context(), userID, service.TransitionDriverSessionStateInput{
			SessionID: sessionID,
			State:     body.State,
		})
		if err != nil {
			writeServiceError(w, err, http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, session)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleTripDemandCreate(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	var input service.CreateTripDemandInput
	if !decodeJSONBody(w, r, &input) {
		return
	}
	input.RiderID = userID
	demand, offer, err := s.services.TripDemand.Create(r.Context(), input)
	if err != nil && err != domain.ErrNoCandidateFound {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"demand": demand,
		"offer":  offer,
	})
}

func (s *Server) handleTripDemandRead(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	demandID := strings.TrimPrefix(r.URL.Path, "/v1/trip-demands/")
	if strings.Contains(demandID, "/") {
		http.NotFound(w, r)
		return
	}
	demand, err := s.services.TripDemand.GetForRider(r.Context(), userID, demandID)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, demand)
}

func (s *Server) handleTripDemandActions(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, "/cancel") {
		http.NotFound(w, r)
		return
	}
	demandID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/trip-demands/"), "/cancel")
	identity, err := currentUserID(r)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	demand, err := s.services.TripDemand.Cancel(r.Context(), identity, demandID)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, demand)
}

func (s *Server) handleOfferActions(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/ride-offers/")
	switch {
	case strings.HasSuffix(path, "/accept"):
		userID, err := currentUserID(r)
		if err != nil {
			writeServiceError(w, err, http.StatusBadRequest)
			return
		}
		offerID := strings.TrimSuffix(path, "/accept")
		booking, err := s.services.Offer.Accept(r.Context(), userID, offerID)
		if err != nil {
			writeServiceError(w, err, http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, booking)
	case strings.HasSuffix(path, "/decline"):
		userID, err := currentUserID(r)
		if err != nil {
			writeServiceError(w, err, http.StatusBadRequest)
			return
		}
		offerID := strings.TrimSuffix(path, "/decline")
		offer, err := s.services.Offer.Decline(r.Context(), userID, offerID)
		if err != nil {
			writeServiceError(w, err, http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, offer)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleBookingRead(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/bookings/")
	switch {
	case strings.HasSuffix(path, "/driver-tracking"):
		s.handleBookingDriverTracking(w, r, userID, strings.TrimSuffix(path, "/driver-tracking"))
		return
	case strings.Contains(path, "/"):
		http.NotFound(w, r)
		return
	}
	booking, err := s.services.Booking.GetForActor(r.Context(), userID, path)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, booking)
}

func (s *Server) handleBookingDriverTracking(w http.ResponseWriter, r *http.Request, userID string, bookingID string) {
	if bookingID == "" || strings.Contains(bookingID, "/") {
		http.NotFound(w, r)
		return
	}
	tracking, err := s.services.Booking.GetDriverTrackingForRider(r.Context(), userID, bookingID)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, tracking)
}

func (s *Server) handleBookingActions(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/bookings/")
	switch {
	case strings.HasSuffix(path, "/state"):
		s.handleBookingTransition(w, r)
	case strings.HasSuffix(path, "/reviews"):
		s.handleReviewCreate(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleBookingTransition(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/bookings/")
	if !strings.HasSuffix(path, "/state") {
		http.NotFound(w, r)
		return
	}
	bookingID := strings.TrimSuffix(path, "/state")
	var body struct {
		State domain.RideBookingState `json:"state"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}
	booking, err := s.services.Booking.TransitionForActor(r.Context(), userID, bookingID, body.State)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, booking)
}

func (s *Server) handleReviewCreate(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, "/reviews") {
		http.NotFound(w, r)
		return
	}
	userID, err := currentUserID(r)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	bookingID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/bookings/"), "/reviews")
	var body struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}
	review, err := s.services.Review.CreateForActor(r.Context(), bookingID, userID, body.Rating, body.Comment)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, review)
}

func (s *Server) handleUserReviewList(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, "/reviews") {
		http.NotFound(w, r)
		return
	}
	userID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/users/"), "/reviews")
	if strings.Contains(userID, "/") || userID == "" {
		http.NotFound(w, r)
		return
	}
	reviews, err := s.services.Review.ListBySubjectID(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, reviews)
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	identity, err := currentIdentity(r)
	if err != nil {
		writeServiceError(w, err, http.StatusUnauthorized)
		return
	}

	response := authBootstrapResponse{
		Auth: ports.AuthIdentity{
			Subject:  identity.Subject,
			Provider: identity.Provider,
		},
		OnboardingState: onboardingStateNeedsProfile,
	}
	if identity.UserID != "" {
		user, err := s.services.Profile.FindByAuthSubject(r.Context(), identity.Subject)
		if err != nil {
			writeServiceError(w, err, http.StatusInternalServerError)
			return
		}
		response.User = &user

		verification, err := s.services.Verification.FindByUserID(r.Context(), user.ID)
		switch {
		case err == nil:
			response.Verification = &verification
			if verification.Status == domain.VerificationPending {
				response.OnboardingState = onboardingStateVerificationPending
			} else if verification.Status == domain.VerificationVerified {
				response.OnboardingState = onboardingStateReady
			} else {
				response.OnboardingState = onboardingStateNeedsVerification
			}
		case errors.Is(err, domain.ErrVerificationRequired):
			response.OnboardingState = onboardingStateNeedsVerification
		default:
			writeServiceError(w, err, http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, http.StatusOK, response)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		writeRequestError(w, http.StatusBadRequest, "invalid request body", err)
		return false
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values in request body")
		}
		writeRequestError(w, http.StatusBadRequest, "invalid request body", err)
		return false
	}

	return true
}

func writeRequestError(w http.ResponseWriter, status int, message string, err error) {
	recordResponseError(w, message, err)
	writeJSON(w, status, map[string]string{"error": message})
}

func writeServiceError(w http.ResponseWriter, err error, defaultStatus int) {
	resolved := resolveServiceError(err, defaultStatus)
	recordResponseError(w, resolved.message, err)
	writeJSON(w, resolved.status, map[string]string{"error": resolved.message})
}

func writeErrorMessage(w http.ResponseWriter, status int, message string) {
	recordResponseError(w, message, nil)
	writeJSON(w, status, map[string]string{"error": message})
}

type resolvedServiceError struct {
	status  int
	message string
}

func resolveServiceError(err error, defaultStatus int) resolvedServiceError {
	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		return resolvedServiceError{status: http.StatusUnauthorized, message: "unauthorized"}
	case errors.Is(err, domain.ErrUserNotFound),
		errors.Is(err, domain.ErrDriverSessionNotFound),
		errors.Is(err, domain.ErrDemandNotFound),
		errors.Is(err, domain.ErrOfferNotFound),
		errors.Is(err, domain.ErrBookingNotFound),
		errors.Is(err, domain.ErrDriverTrackingUnavailable):
		return resolvedServiceError{
			status:  http.StatusNotFound,
			message: messageForKnownServiceError(err),
		}
	case errors.Is(err, domain.ErrVerificationRequired):
		return resolvedServiceError{status: defaultStatus, message: "verification required"}
	case errors.Is(err, domain.ErrVehicleRequired):
		return resolvedServiceError{status: defaultStatus, message: "active vehicle required"}
	case errors.Is(err, domain.ErrNoCandidateFound):
		return resolvedServiceError{status: defaultStatus, message: "no matching driver candidate found"}
	case errors.Is(err, domain.ErrInvalidUserPreferences):
		return resolvedServiceError{status: http.StatusBadRequest, message: "invalid user preferences"}
	case errors.Is(err, domain.ErrCapacityExceeded):
		return resolvedServiceError{status: defaultStatus, message: "capacity exceeded"}
	case errors.Is(err, domain.ErrIdempotencyConflict):
		return resolvedServiceError{status: defaultStatus, message: "idempotency conflict"}
	case errors.Is(err, domain.ErrWomenOnlyRequiresVerifiedFemaleRider):
		return resolvedServiceError{status: defaultStatus, message: "women-only matching is unavailable for this account"}
	default:
		if defaultStatus >= http.StatusInternalServerError {
			return resolvedServiceError{status: defaultStatus, message: "internal server error"}
		}
		return resolvedServiceError{status: defaultStatus, message: err.Error()}
	}
}

func messageForKnownServiceError(err error) string {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		return "user not found"
	case errors.Is(err, domain.ErrDriverSessionNotFound):
		return "driver session not found"
	case errors.Is(err, domain.ErrDemandNotFound):
		return "trip demand not found"
	case errors.Is(err, domain.ErrOfferNotFound):
		return "ride offer not found"
	case errors.Is(err, domain.ErrBookingNotFound):
		return "ride booking not found"
	case errors.Is(err, domain.ErrDriverTrackingUnavailable):
		return "driver tracking unavailable"
	default:
		return err.Error()
	}
}
