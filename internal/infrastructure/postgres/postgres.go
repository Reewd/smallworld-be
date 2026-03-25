package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultConnectTimeout = 5 * time.Second

type OpenOptions struct {
	Logger             *slog.Logger
	SlowQueryThreshold time.Duration
	LogAllQueries      bool
}

func Open(ctx context.Context, databaseURL string, options OpenOptions) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		return nil, errors.New("database url is required")
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	if config.MaxConnIdleTime == 0 {
		config.MaxConnIdleTime = 15 * time.Minute
	}
	if config.MaxConnLifetime == 0 {
		config.MaxConnLifetime = time.Hour
	}
	if config.HealthCheckPeriod == 0 {
		config.HealthCheckPeriod = 30 * time.Second
	}
	if options.Logger != nil {
		config.ConnConfig.Tracer = newQueryTracer(options.Logger, options.SlowQueryThreshold, options.LogAllQueries)
	}

	ctx, cancel := context.WithTimeout(ctx, defaultConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}

func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
