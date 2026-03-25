package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	FormatText = "text"
	FormatJSON = "json"

	defaultSlowQueryThreshold = 250 * time.Millisecond
)

type Config struct {
	ServiceName            string
	Environment            string
	Format                 string
	Level                  slog.Level
	DBSlowQueryThreshold   time.Duration
	DBLogAllQueriesEnabled bool
}

func LoadConfigFromEnv(serviceName string) (Config, error) {
	cfg := Config{
		ServiceName:            serviceName,
		Environment:            strings.TrimSpace(os.Getenv("APP_ENV")),
		Level:                  slog.LevelInfo,
		DBSlowQueryThreshold:   defaultSlowQueryThreshold,
		DBLogAllQueriesEnabled: false,
	}

	format, err := parseFormat(os.Getenv("LOG_FORMAT"), cfg.Environment)
	if err != nil {
		return Config{}, err
	}
	cfg.Format = format

	level, err := parseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		return Config{}, err
	}
	cfg.Level = level

	slowThreshold, err := parseSlowQueryThreshold(os.Getenv("DB_LOG_SLOW_QUERY_MS"))
	if err != nil {
		return Config{}, err
	}
	if slowThreshold >= 0 {
		cfg.DBSlowQueryThreshold = slowThreshold
	}

	logAllQueries, err := parseBoolWithDefault(os.Getenv("DB_LOG_ALL_QUERIES"), false)
	if err != nil {
		return Config{}, err
	}
	cfg.DBLogAllQueriesEnabled = logAllQueries

	return cfg, nil
}

func NewLogger(cfg Config, w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}

	options := &slog.HandlerOptions{
		Level:     cfg.Level,
		AddSource: cfg.Level <= slog.LevelDebug,
	}

	var handler slog.Handler
	switch cfg.Format {
	case FormatJSON:
		handler = slog.NewJSONHandler(w, options)
	default:
		handler = slog.NewTextHandler(w, options)
	}

	logger := slog.New(handler)
	if cfg.ServiceName != "" {
		logger = logger.With("service", cfg.ServiceName)
	}
	if cfg.Environment != "" {
		logger = logger.With("environment", cfg.Environment)
	}
	return logger
}

func parseFormat(raw, env string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultFormatForEnv(env), nil
	}

	switch strings.ToLower(strings.TrimSpace(raw)) {
	case FormatText:
		return FormatText, nil
	case FormatJSON:
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("invalid LOG_FORMAT %q", raw)
	}
}

func parseLevel(raw string) (slog.Level, error) {
	if strings.TrimSpace(raw) == "" {
		return slog.LevelInfo, nil
	}

	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid LOG_LEVEL %q", raw)
	}
}

func parseSlowQueryThreshold(raw string) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return -1, nil
	}

	ms, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid DB_LOG_SLOW_QUERY_MS %q", raw)
	}
	if ms < 0 {
		return 0, fmt.Errorf("invalid DB_LOG_SLOW_QUERY_MS %q", raw)
	}
	return time.Duration(ms) * time.Millisecond, nil
}

func parseBoolWithDefault(raw string, fallback bool) (bool, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}

	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("invalid boolean value %q", raw)
	}
	return value, nil
}

func defaultFormatForEnv(env string) string {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "", "local", "dev", "development", "test":
		return FormatText
	default:
		return FormatJSON
	}
}
