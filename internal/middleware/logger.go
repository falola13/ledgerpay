package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

type StatusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *StatusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &StatusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		attrs := []any{
			"Method", r.Method,
			"Status", rec.status,
			"Path", r.URL.Path,
			"duration_ms", time.Since(start).Milliseconds(),
		}

		switch {
		case rec.status >= 500:
			slog.Error("request", attrs...)
		case rec.status >= 400:
			slog.Warn("request", attrs...)
		default:
			slog.Info("request", attrs...)
		}
	})
}
