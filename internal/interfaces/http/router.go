package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"smallworld/internal/application"
	"smallworld/internal/application/service"
	"smallworld/internal/domain"
	"smallworld/internal/ports"
)

type Server struct {
	services     application.Services
	authVerifier ports.AuthVerifier
}

func NewServer(services application.Services, authVerifier ports.AuthVerifier) *Server {
	return &Server{services: services, authVerifier: authVerifier}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /v1/auth/me", s.handleAuthMe)
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
	return s.authMiddleware(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleProfileUpsert(w http.ResponseWriter, r *http.Request) {
	identity, err := currentIdentity(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	var input service.UpsertProfileInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := s.services.Profile.UpsertAuthenticated(r.Context(), identity.Subject, input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleProfileMe(w http.ResponseWriter, r *http.Request) {
	identity, err := currentIdentity(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	user, err := s.services.Profile.FindByAuthSubject(r.Context(), identity.Subject)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleVehicleCreate(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	var input service.CreateVehicleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	input.UserID = userID
	vehicle, err := s.services.Vehicle.Create(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, vehicle)
}

func (s *Server) handleVehicleList(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	vehicles, err := s.services.Vehicle.ListByUserID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, vehicles)
}

func (s *Server) handleDriverSessionCreate(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	var input service.StartDriverSessionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	input.UserID = userID
	session, err := s.services.DriverSession.Start(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (s *Server) handleDriverSessionRead(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	sessionID := strings.TrimPrefix(r.URL.Path, "/v1/driver-sessions/")
	if strings.Contains(sessionID, "/") {
		http.NotFound(w, r)
		return
	}
	session, err := s.services.DriverSession.GetOwned(r.Context(), userID, sessionID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleDriverSessionActions(w http.ResponseWriter, r *http.Request) {
	userID, err := currentUserID(r)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/driver-sessions/")
	switch {
	case strings.HasSuffix(path, "/heartbeat"):
		sessionID := strings.TrimSuffix(path, "/heartbeat")
		var body struct {
			CurrentLocation domain.Location `json:"current_location"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		session, err := s.services.DriverSession.HeartbeatOwned(r.Context(), userID, service.HeartbeatDriverSessionInput{
			SessionID:       sessionID,
			CurrentLocation: body.CurrentLocation,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, session)
	case strings.HasSuffix(path, "/state"):
		sessionID := strings.TrimSuffix(path, "/state")
		var body struct {
			State domain.DriverSessionState `json:"state"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		session, err := s.services.DriverSession.TransitionStateOwned(r.Context(), userID, service.TransitionDriverSessionStateInput{
			SessionID: sessionID,
			State:     body.State,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
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
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	var input service.CreateTripDemandInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	input.RiderID = userID
	demand, offer, err := s.services.TripDemand.Create(r.Context(), input)
	if err != nil && err != domain.ErrNoCandidateFound {
		writeError(w, http.StatusBadRequest, err)
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
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	demandID := strings.TrimPrefix(r.URL.Path, "/v1/trip-demands/")
	if strings.Contains(demandID, "/") {
		http.NotFound(w, r)
		return
	}
	demand, err := s.services.TripDemand.GetForRider(r.Context(), userID, demandID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
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
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	demand, err := s.services.TripDemand.Cancel(r.Context(), identity, demandID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
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
			status := http.StatusBadRequest
			if errors.Is(err, domain.ErrUnauthorized) {
				status = http.StatusUnauthorized
			}
			writeError(w, status, err)
			return
		}
		offerID := strings.TrimSuffix(path, "/accept")
		booking, err := s.services.Offer.Accept(r.Context(), userID, offerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, booking)
	case strings.HasSuffix(path, "/decline"):
		userID, err := currentUserID(r)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, domain.ErrUnauthorized) {
				status = http.StatusUnauthorized
			}
			writeError(w, status, err)
			return
		}
		offerID := strings.TrimSuffix(path, "/decline")
		offer, err := s.services.Offer.Decline(r.Context(), userID, offerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
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
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/bookings/")
	if strings.Contains(path, "/") {
		http.NotFound(w, r)
		return
	}
	booking, err := s.services.Booking.GetForActor(r.Context(), userID, path)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, booking)
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
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	booking, err := s.services.Booking.TransitionForActor(r.Context(), userID, bookingID, body.State)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
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
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	bookingID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/bookings/"), "/reviews")
	var body struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	review, err := s.services.Review.CreateForActor(r.Context(), bookingID, userID, body.Rating, body.Comment)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
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
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, reviews)
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	identity, err := currentIdentity(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	response := authBootstrapResponse{
		Auth: ports.AuthIdentity{
			Subject:  identity.Subject,
			Provider: identity.Provider,
		},
	}
	if identity.UserID != "" {
		user, err := s.services.Profile.FindByAuthSubject(r.Context(), identity.Subject)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		response.User = &user
	}
	writeJSON(w, http.StatusOK, response)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
