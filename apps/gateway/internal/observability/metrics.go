// Package observability provides Prometheus metrics and OpenTelemetry tracing.
package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for the gateway.
var Metrics = struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	ActiveRequests  prometheus.Gauge
	AuthAttempts    *prometheus.CounterVec
	AgentExecutions *prometheus.CounterVec
	LLMRequests     *prometheus.CounterVec
	LLMLatency      *prometheus.HistogramVec
	SessionsActive  prometheus.Gauge
	RateLimitHits   *prometheus.CounterVec
}{
	RequestsTotal: promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_requests_total",
			Help: "Total HTTP requests by path, method, and status",
		},
		[]string{"path", "method", "status"},
	),
	RequestDuration: promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_request_duration_seconds",
			Help:    "HTTP request duration by path",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "method"},
	),
	ActiveRequests: promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "gateway_active_requests",
			Help: "Number of active requests",
		},
	),
	AuthAttempts: promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_auth_attempts_total",
			Help: "Authentication attempts by type and success",
		},
		[]string{"type", "success"},
	),
	AgentExecutions: promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_agent_executions_total",
			Help: "Agent executions by agent type and status",
		},
		[]string{"agent", "status"},
	),
	LLMRequests: promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_llm_requests_total",
			Help: "LLM requests by provider and model",
		},
		[]string{"provider", "model"},
	),
	LLMLatency: promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_llm_latency_seconds",
			Help:    "LLM request latency by provider",
			Buckets: []float64{0.5, 1, 2, 5, 10, 30, 60, 120},
		},
		[]string{"provider"},
	),
	SessionsActive: promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "gateway_sessions_active",
			Help: "Number of active user sessions",
		},
	),
	RateLimitHits: promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_rate_limit_hits_total",
			Help: "Rate limit hits by IP",
		},
		[]string{"path"},
	),
}

// MetricsHandler returns the Prometheus metrics handler.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// MetricsMiddleware records request metrics.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		Metrics.ActiveRequests.Inc()

		// Wrap response writer to capture status
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		Metrics.ActiveRequests.Dec()
		duration := time.Since(start).Seconds()

		Metrics.RequestsTotal.WithLabelValues(
			r.URL.Path,
			r.Method,
			strconv.Itoa(wrapped.status),
		).Inc()

		Metrics.RequestDuration.WithLabelValues(
			r.URL.Path,
			r.Method,
		).Observe(duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// RecordAuthAttempt records an authentication attempt.
func RecordAuthAttempt(authType string, success bool) {
	Metrics.AuthAttempts.WithLabelValues(authType, strconv.FormatBool(success)).Inc()
}

// RecordAgentExecution records an agent execution.
func RecordAgentExecution(agent, status string) {
	Metrics.AgentExecutions.WithLabelValues(agent, status).Inc()
}

// RecordLLMRequest records an LLM request.
func RecordLLMRequest(provider, model string, latency time.Duration) {
	Metrics.LLMRequests.WithLabelValues(provider, model).Inc()
	Metrics.LLMLatency.WithLabelValues(provider).Observe(latency.Seconds())
}
