package auth

import (
	"context"
	"errors"
	"strings"

	"smallworld/internal/ports"
)

var ErrInvalidToken = errors.New("invalid token")

type DevVerifier struct {
	Provider string
	Prefix   string
}

func (v DevVerifier) VerifyToken(_ context.Context, rawToken string) (ports.AuthIdentity, error) {
	token := strings.TrimSpace(rawToken)
	if token == "" {
		return ports.AuthIdentity{}, ErrInvalidToken
	}

	prefix := v.Prefix
	if prefix == "" {
		prefix = "dev:"
	}
	if !strings.HasPrefix(token, prefix) {
		return ports.AuthIdentity{}, ErrInvalidToken
	}

	subject := strings.TrimSpace(strings.TrimPrefix(token, prefix))
	if subject == "" {
		return ports.AuthIdentity{}, ErrInvalidToken
	}

	provider := v.Provider
	if provider == "" {
		provider = "dev"
	}
	return ports.AuthIdentity{
		Subject:  subject,
		Provider: provider,
	}, nil
}
