package postgres

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type queryTraceContextKey struct{}

type queryTraceContext struct {
	startedAt time.Time
	sql       string
}

type queryTracer struct {
	logger             *slog.Logger
	slowQueryThreshold time.Duration
	logAllQueries      bool
}

func newQueryTracer(logger *slog.Logger, slowQueryThreshold time.Duration, logAllQueries bool) *queryTracer {
	return &queryTracer{
		logger:             logger,
		slowQueryThreshold: slowQueryThreshold,
		logAllQueries:      logAllQueries,
	}
}

func (t *queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if t == nil || t.logger == nil {
		return ctx
	}

	return context.WithValue(ctx, queryTraceContextKey{}, queryTraceContext{
		startedAt: time.Now(),
		sql:       normalizeSQL(data.SQL),
	})
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if t == nil || t.logger == nil {
		return
	}

	traceContext, _ := ctx.Value(queryTraceContextKey{}).(queryTraceContext)
	if traceContext.startedAt.IsZero() {
		traceContext.startedAt = time.Now()
	}

	duration := time.Since(traceContext.startedAt)
	fields := []any{
		"sql", traceContext.sql,
		"duration_ms", duration.Milliseconds(),
	}

	if rowsAffected := data.CommandTag.RowsAffected(); rowsAffected > 0 {
		fields = append(fields, "rows_affected", rowsAffected)
	}

	switch {
	case data.Err != nil:
		t.logger.ErrorContext(ctx, "postgres query failed", append(fields, "error", data.Err)...)
	case t.logAllQueries:
		t.logger.DebugContext(ctx, "postgres query completed", fields...)
	case t.slowQueryThreshold > 0 && duration >= t.slowQueryThreshold:
		t.logger.WarnContext(ctx, "postgres query slow", fields...)
	}
}

func normalizeSQL(sql string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(sql)), " ")
}
