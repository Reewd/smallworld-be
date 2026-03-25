package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigFromEnvDefaultsToLocalText(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("LOG_FORMAT", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("DB_LOG_SLOW_QUERY_MS", "")
	t.Setenv("DB_LOG_ALL_QUERIES", "")

	cfg, err := LoadConfigFromEnv("smallworld-api")
	if err != nil {
		t.Fatalf("LoadConfigFromEnv returned error: %v", err)
	}

	if cfg.Format != FormatText {
		t.Fatalf("expected text format, got %q", cfg.Format)
	}
	if cfg.Level != slog.LevelInfo {
		t.Fatalf("expected info level, got %v", cfg.Level)
	}
	if cfg.DBSlowQueryThreshold != 250*time.Millisecond {
		t.Fatalf("expected 250ms slow query threshold, got %s", cfg.DBSlowQueryThreshold)
	}
	if cfg.DBLogAllQueriesEnabled {
		t.Fatalf("expected DB_LOG_ALL_QUERIES to default to false")
	}
}

func TestLoadConfigFromEnvHonorsOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("DB_LOG_SLOW_QUERY_MS", "42")
	t.Setenv("DB_LOG_ALL_QUERIES", "true")

	cfg, err := LoadConfigFromEnv("smallworld-api")
	if err != nil {
		t.Fatalf("LoadConfigFromEnv returned error: %v", err)
	}

	if cfg.Format != FormatJSON {
		t.Fatalf("expected json format, got %q", cfg.Format)
	}
	if cfg.Level != slog.LevelDebug {
		t.Fatalf("expected debug level, got %v", cfg.Level)
	}
	if cfg.DBSlowQueryThreshold != 42*time.Millisecond {
		t.Fatalf("expected 42ms slow query threshold, got %s", cfg.DBSlowQueryThreshold)
	}
	if !cfg.DBLogAllQueriesEnabled {
		t.Fatalf("expected DB_LOG_ALL_QUERIES to be true")
	}
}

func TestLoadConfigFromEnvRejectsInvalidLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "verbose")

	if _, err := LoadConfigFromEnv("smallworld-api"); err == nil {
		t.Fatalf("expected invalid LOG_LEVEL error")
	}
}

func TestNewLoggerIncludesServiceField(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(Config{
		ServiceName: "smallworld-api",
		Format:      FormatJSON,
		Level:       slog.LevelInfo,
	}, &buf)

	logger.InfoContext(context.Background(), "logger ready", "component", "test")

	output := buf.String()
	if !strings.Contains(output, `"service":"smallworld-api"`) {
		t.Fatalf("expected service field in log output, got %s", output)
	}
	if !strings.Contains(output, `"component":"test"`) {
		t.Fatalf("expected component field in log output, got %s", output)
	}
}
