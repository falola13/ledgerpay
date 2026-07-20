package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/falola13/ledgerpay/internal/metrics"
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

		requestId := r.Context().Value(RequestIdKey).(string)

		next.ServeHTTP(rec, r)

		route := r.Pattern

		if route == "" {
			route = "unmatched"
		}

		metrics.RequestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
		metrics.RequestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())

		attrs := []any{
			"Method", r.Method,
			"Status", rec.status,
			"Path", r.URL.Path,
			"request_id", requestId,
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
