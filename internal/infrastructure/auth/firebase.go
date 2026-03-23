package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	firebase "firebase.google.com/go/v4"
	fbauth "firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"

	"smallworld/internal/ports"
)

var ErrInvalidToken = errors.New("invalid token")

type FirebaseConfig struct {
	ProjectID        string
	CredentialsFile  string
	AuthEmulatorHost string
}

type FirebaseVerifier struct {
	client *fbauth.Client
}

func NewFirebaseVerifier(ctx context.Context, cfg FirebaseConfig) (*FirebaseVerifier, error) {
	fbConfig := &firebase.Config{}
	if cfg.ProjectID != "" {
		fbConfig.ProjectID = cfg.ProjectID
	}

	var opts []option.ClientOption
	if cfg.AuthEmulatorHost != "" {
		if err := os.Setenv("FIREBASE_AUTH_EMULATOR_HOST", strings.TrimSpace(cfg.AuthEmulatorHost)); err != nil {
			return nil, fmt.Errorf("set FIREBASE_AUTH_EMULATOR_HOST: %w", err)
		}
	}
	if cfg.AuthEmulatorHost == "" && cfg.CredentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(cfg.CredentialsFile))
	}

	app, err := firebase.NewApp(ctx, fbConfig, opts...)
	if err != nil {
		return nil, fmt.Errorf("initialize firebase app: %w", err)
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize firebase auth client: %w", err)
	}

	return &FirebaseVerifier{client: client}, nil
}

func (v *FirebaseVerifier) VerifyToken(ctx context.Context, rawToken string) (ports.AuthIdentity, error) {
	token, err := v.client.VerifyIDToken(ctx, rawToken)
	if err != nil {
		return ports.AuthIdentity{}, ErrInvalidToken
	}
	return ports.AuthIdentity{
		Subject:  token.UID,
		Provider: "firebase",
	}, nil
}
