// Package metrics provides Prometheus metrics for AIWX.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ─── Metrics ──────────────────────────────────────────────────────────────────

var (
	// HTTP metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aiwx_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aiwx_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Job metrics
	JobsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aiwx_jobs_total",
			Help: "Total number of jobs by status",
		},
		[]string{"organization", "status"},
	)

	JobsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "aiwx_jobs_active",
			Help: "Number of currently active (non-terminal) jobs",
		},
	)

	JobDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "aiwx_job_duration_seconds",
			Help:    "Job execution duration in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600},
		},
	)

	// Outbox metrics
	OutboxUnpublished = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "aiwx_outbox_unpublished",
			Help: "Number of unpublished outbox events",
		},
	)

	OutboxPublishedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "aiwx_outbox_published_total",
			Help: "Total number of published outbox events",
		},
	)

	OutboxFailedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "aiwx_outbox_failed_total",
			Help: "Total number of failed outbox publishes",
		},
	)

	// Database metrics
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aiwx_db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	DBConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "aiwx_db_connections_active",
			Help: "Number of active database connections",
		},
	)

	// Project sync metrics
	ProjectSyncsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aiwx_project_syncs_total",
			Help: "Total number of project syncs by status",
		},
		[]string{"status"},
	)

	ProjectSyncDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "aiwx_project_sync_duration_seconds",
			Help:    "Project sync duration in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
		},
	)

	// NATS metrics
	NATSPublishedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "aiwx_nats_published_total",
			Help: "Total number of NATS messages published",
		},
	)

	NATSReceivedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "aiwx_nats_received_total",
			Help: "Total number of NATS messages received",
		},
	)
)

// ─── Middleware ───────────────────────────────────────────────────────────────

// InstrumentHandler wraps an HTTP handler with metrics.
func InstrumentHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		duration := time.Since(start).Seconds()
		path := r.URL.Path

		HTTPRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(sw.status)).Inc()
		HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}

// statusWriter captures the response status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// ─── Handler ──────────────────────────────────────────────────────────────────

// Handler returns the Prometheus metrics handler.
func Handler() http.Handler {
	return promhttp.Handler()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// RecordJobStatus records a job status change.
func RecordJobStatus(organization string, status string) {
	JobsTotal.WithLabelValues(organization, status).Inc()
}

// RecordJobDuration records job execution duration.
func RecordJobDuration(duration time.Duration) {
	JobDuration.Observe(duration.Seconds())
}

// SetActiveJobs sets the number of active jobs.
func SetActiveJobs(count int) {
	JobsActive.Set(float64(count))
}

// RecordOutboxPublished records an outbox publish.
func RecordOutboxPublished() {
	OutboxPublishedTotal.Inc()
}

// RecordOutboxFailed records an outbox publish failure.
func RecordOutboxFailed() {
	OutboxFailedTotal.Inc()
}

// RecordDBQuery records a database query.
func RecordDBQuery(operation string, duration time.Duration) {
	DBQueryDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordProjectSync records a project sync.
func RecordProjectSync(status string, duration time.Duration) {
	ProjectSyncsTotal.WithLabelValues(status).Inc()
	ProjectSyncDuration.Observe(duration.Seconds())
}
