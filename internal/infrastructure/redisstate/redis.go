package redisstate

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"

	"smallworld/internal/domain"
)

const defaultConnectTimeout = 5 * time.Second

func Open(ctx context.Context, redisURL string) (*redis.Client, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis config: %w", err)
	}

	client := redis.NewClient(options)
	ctx, cancel := context.WithTimeout(ctx, defaultConnectTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return client, nil
}

type DriverPresenceStore struct {
	Client    *redis.Client
	KeyPrefix string
	TTL       time.Duration
}

func (s DriverPresenceStore) SaveSession(ctx context.Context, session domain.DriverSession) error {
	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal driver session presence: %w", err)
	}
	expiration := s.TTL
	if expiration <= 0 {
		expiration = 0
	}
	return s.Client.Set(ctx, s.driverKey(session.ID), payload, expiration).Err()
}

func (s DriverPresenceStore) DeleteSession(ctx context.Context, sessionID string) error {
	return s.Client.Del(ctx, s.driverKey(sessionID)).Err()
}

func (s DriverPresenceStore) ListActiveSessions(ctx context.Context) ([]domain.DriverSession, error) {
	var sessions []domain.DriverSession
	iter := s.Client.Scan(ctx, 0, s.driverKey("*"), 0).Iterator()
	for iter.Next(ctx) {
		value, err := s.Client.Get(ctx, iter.Val()).Bytes()
		if err != nil {
			return nil, fmt.Errorf("get driver session presence: %w", err)
		}
		var session domain.DriverSession
		if err := json.Unmarshal(value, &session); err != nil {
			return nil, fmt.Errorf("unmarshal driver session presence: %w", err)
		}
		if session.State == domain.DriverSessionStateActive || session.State == domain.DriverSessionStateFull {
			sessions = append(sessions, session)
		}
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("scan driver session presence: %w", err)
	}
	return sessions, nil
}

func (s DriverPresenceStore) driverKey(sessionID string) string {
	prefix := s.KeyPrefix
	if prefix == "" {
		prefix = "smallworld:driver_session:"
	}
	return prefix + sessionID
}

type EphemeralOfferStore struct {
	Client    *redis.Client
	KeyPrefix string
	TTL       time.Duration
}

func (s EphemeralOfferStore) SavePendingOffer(ctx context.Context, offer domain.RideOffer) error {
	payload, err := json.Marshal(offer)
	if err != nil {
		return fmt.Errorf("marshal ephemeral offer: %w", err)
	}
	expiration := s.TTL
	if expiration <= 0 {
		expiration = 2 * time.Minute
	}
	return s.Client.Set(ctx, s.offerKey(offer.ID), payload, expiration).Err()
}

func (s EphemeralOfferStore) DeletePendingOffer(ctx context.Context, offerID string) error {
	return s.Client.Del(ctx, s.offerKey(offerID)).Err()
}

func (s EphemeralOfferStore) offerKey(offerID string) string {
	prefix := s.KeyPrefix
	if prefix == "" {
		prefix = "smallworld:offer:"
	}
	return prefix + offerID
}
