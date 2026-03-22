package ports

import "context"

type AuthIdentity struct {
	Subject  string `json:"subject"`
	Provider string `json:"provider"`
}

type AuthVerifier interface {
	VerifyToken(ctx context.Context, rawToken string) (AuthIdentity, error)
}
