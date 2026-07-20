package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var RequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "HTTP requests by method, route and status.",
	}, []string{"method", "route", "status"},
)

var RequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Request latency.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"method", "route"},
)

var OutboxDepth = promauto.NewGauge(
	prometheus.GaugeOpts{
		Name: "outbox_backlog_depth",
		Help: "Pending count currently waiting in the outbox",
	},
)

var DeadLettersDepth = promauto.NewGauge(
	prometheus.GaugeOpts{
		Name: "dead_letters_depth",
		Help: "Dead letters count with the status of dead",
	},
)
