package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestQueryTracerLogsSlowQueries(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tracer := newQueryTracer(logger, time.Millisecond, false)

	ctx := tracer.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{
		SQL: "SELECT    *\nFROM users",
	})
	time.Sleep(2 * time.Millisecond)
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{
		CommandTag: pgconn.NewCommandTag("SELECT 1"),
	})

	entry := decodeSingleLogEntry(t, buf.Bytes())
	if entry["msg"] != "postgres query slow" {
		t.Fatalf("unexpected log message: %#v", entry)
	}
	if entry["sql"] != "SELECT * FROM users" {
		t.Fatalf("unexpected sql field: %#v", entry["sql"])
	}
}

func TestQueryTracerLogsErrors(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tracer := newQueryTracer(logger, time.Hour, false)

	ctx := tracer.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{
		SQL: "INSERT INTO users (id) VALUES ($1)",
	})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{
		Err:        errors.New("boom"),
		CommandTag: pgconn.NewCommandTag("INSERT 0 0"),
	})

	entry := decodeSingleLogEntry(t, buf.Bytes())
	if entry["msg"] != "postgres query failed" {
		t.Fatalf("unexpected log message: %#v", entry)
	}
	if entry["error"] != "boom" {
		t.Fatalf("unexpected error field: %#v", entry["error"])
	}
}

func TestQueryTracerSkipsNormalQueriesByDefault(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tracer := newQueryTracer(logger, time.Hour, false)

	ctx := tracer.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{
		SQL: "SELECT 1",
	})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{
		CommandTag: pgconn.NewCommandTag("SELECT 1"),
	})

	if buf.Len() != 0 {
		t.Fatalf("expected no logs for non-slow successful queries, got %s", buf.String())
	}
}

func TestQueryTracerLogsAllQueriesOnlyWhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tracer := newQueryTracer(logger, time.Hour, true)

	ctx := tracer.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{
		SQL: "UPDATE users SET display_name = $1 WHERE id = $2",
	})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{
		CommandTag: pgconn.NewCommandTag("UPDATE 1"),
	})

	entry := decodeSingleLogEntry(t, buf.Bytes())
	if entry["msg"] != "postgres query completed" {
		t.Fatalf("unexpected log message: %#v", entry)
	}
	if _, ok := entry["args"]; ok {
		t.Fatalf("did not expect args to be logged: %#v", entry)
	}
}

func decodeSingleLogEntry(t *testing.T, data []byte) map[string]any {
	t.Helper()

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &entry); err != nil {
		t.Fatalf("decode log entry: %v", err)
	}
	return entry
}
