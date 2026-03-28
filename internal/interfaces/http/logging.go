package httpapi

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

type responseErrorRecorder interface {
	recordError(message string, err error)
}

type requestStartedAtContextKey struct{}

type loggingResponseWriter struct {
	http.ResponseWriter
	status        int
	bytesWritten  int
	errorMessage  string
	internalError string
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	written, err := w.ResponseWriter.Write(body)
	w.bytesWritten += written
	return written, err
}

func (w *loggingResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("http hijacker is unavailable")
	}
	return hijacker.Hijack()
}

func (w *loggingResponseWriter) Push(target string, options *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, options)
}

func (w *loggingResponseWriter) recordError(message string, err error) {
	w.errorMessage = message
	if err != nil {
		w.internalError = err.Error()
	}
}

func recordResponseError(w http.ResponseWriter, message string, err error) {
	if recorder, ok := w.(responseErrorRecorder); ok {
		recorder.recordError(message, err)
	}
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt, ok := r.Context().Value(requestStartedAtContextKey{}).(time.Time)
		if !ok || startedAt.IsZero() {
			startedAt = time.Now()
		}
		responseWriter := &loggingResponseWriter{ResponseWriter: w}

		s.logger.DebugContext(r.Context(), "http request started",
			"method", r.Method,
			"path", r.URL.Path,
		)

		next.ServeHTTP(responseWriter, r)

		status := responseWriter.status
		if status == 0 {
			status = http.StatusOK
		}

		fields := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"bytes", responseWriter.bytesWritten,
		}

		if identity, err := requireIdentity(r.Context()); err == nil && identity.UserID != "" {
			fields = append(fields, "user_id", identity.UserID)
		}
		if responseWriter.errorMessage != "" {
			fields = append(fields, "error", responseWriter.errorMessage)
		}
		if status >= http.StatusBadRequest && responseWriter.internalError != "" && responseWriter.internalError != responseWriter.errorMessage {
			fields = append(fields, "cause", responseWriter.internalError)
		}

		switch {
		case status >= http.StatusInternalServerError:
			s.logger.ErrorContext(r.Context(), "http request completed", fields...)
		case status >= http.StatusBadRequest:
			s.logger.WarnContext(r.Context(), "http request completed", fields...)
		default:
			s.logger.InfoContext(r.Context(), "http request completed", fields...)
		}
	})
}

func (s *Server) logAuthFailure(ctx context.Context, r *http.Request, status int, message string, duration time.Duration) {
	fields := []any{
		"method", r.Method,
		"path", r.URL.Path,
		"status", status,
		"duration_ms", duration.Milliseconds(),
		"error", message,
	}

	switch {
	case status >= http.StatusInternalServerError:
		s.logger.ErrorContext(ctx, "http request completed", fields...)
	case status >= http.StatusBadRequest:
		s.logger.WarnContext(ctx, "http request completed", fields...)
	default:
		s.logger.InfoContext(ctx, "http request completed", fields...)
	}
}
