package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/ports"
)

type RequestIdentity struct {
	Subject  string `json:"subject"`
	Provider string `json:"provider"`
	UserID   string `json:"user_id,omitempty"`
}

type identityContextKey struct{}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()

		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			s.logAuthFailure(r.Context(), r, http.StatusUnauthorized, "missing bearer token", time.Since(startedAt))
			writeErrorMessage(w, http.StatusUnauthorized, "missing bearer token")
			return
		}

		authIdentity, err := s.authVerifier.VerifyToken(r.Context(), token)
		if err != nil {
			s.logAuthFailure(r.Context(), r, http.StatusUnauthorized, "invalid bearer token", time.Since(startedAt))
			writeErrorMessage(w, http.StatusUnauthorized, "invalid bearer token")
			return
		}

		identity := RequestIdentity{
			Subject:  authIdentity.Subject,
			Provider: authIdentity.Provider,
		}
		if user, err := s.services.Profile.FindByAuthSubject(r.Context(), authIdentity.Subject); err == nil {
			identity.UserID = user.ID
		} else if !errors.Is(err, domain.ErrUserNotFound) {
			resolved := resolveServiceError(err, http.StatusInternalServerError)
			s.logAuthFailure(r.Context(), r, resolved.status, resolved.message, time.Since(startedAt))
			s.logger.ErrorContext(r.Context(), "auth identity resolution failed", "path", r.URL.Path, "error", err)
			writeServiceError(w, err, http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), requestStartedAtContextKey{}, startedAt)
		ctx = context.WithValue(ctx, identityContextKey{}, identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requireIdentity(ctx context.Context) (RequestIdentity, error) {
	identity, ok := ctx.Value(identityContextKey{}).(RequestIdentity)
	if !ok {
		return RequestIdentity{}, domain.ErrUnauthorized
	}
	return identity, nil
}

func requireUserID(ctx context.Context) (RequestIdentity, error) {
	identity, err := requireIdentity(ctx)
	if err != nil {
		return RequestIdentity{}, err
	}
	if identity.UserID == "" {
		return RequestIdentity{}, domain.ErrUserNotFound
	}
	return identity, nil
}

func bearerToken(header string) (string, bool) {
	if header == "" {
		return "", false
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return token, token != ""
}

func currentUserID(r *http.Request) (string, error) {
	identity, err := requireUserID(r.Context())
	if err != nil {
		return "", err
	}
	return identity.UserID, nil
}

func currentIdentity(r *http.Request) (RequestIdentity, error) {
	return requireIdentity(r.Context())
}

type authBootstrapResponse struct {
	Auth ports.AuthIdentity `json:"auth"`
	User *domain.User       `json:"user,omitempty"`
}
