package metrics

import (
	"bufio"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "swoops_http_requests_total",
			Help: "Total number of HTTP requests by method, path, and status",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "swoops_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// WAF metrics
	WAFRequestsBlocked = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "swoops_waf_requests_blocked_total",
			Help: "Total number of requests blocked by WAF by reason",
		},
		[]string{"reason"},
	)

	WAFRateLimitHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "swoops_waf_rate_limit_hits_total",
			Help: "Total number of requests blocked due to rate limiting",
		},
	)

	WAFMaliciousPatterns = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "swoops_waf_malicious_patterns_total",
			Help: "Total number of malicious patterns detected by type",
		},
		[]string{"pattern_type"},
	)

	// gRPC agent connection metrics
	AgentConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "swoops_agent_connections_active",
			Help: "Number of currently connected agents",
		},
	)

	AgentConnectionsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "swoops_agent_connections_total",
			Help: "Total number of agent connection attempts",
		},
	)

	AgentConnectionErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "swoops_agent_connection_errors_total",
			Help: "Total number of agent connection errors by type",
		},
		[]string{"error_type"},
	)

	AgentHeartbeatsReceived = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "swoops_agent_heartbeats_received_total",
			Help: "Total number of heartbeats received from agents",
		},
	)

	// Session metrics
	SessionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "swoops_sessions_active",
			Help: "Number of currently active sessions",
		},
	)

	SessionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "swoops_sessions_total",
			Help: "Total number of sessions by agent type",
		},
		[]string{"agent_type"},
	)

	SessionCommandsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "swoops_session_commands_total",
			Help: "Total number of session commands by type and status",
		},
		[]string{"command_type", "status"},
	)

	SessionCommandDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "swoops_session_command_duration_seconds",
			Help:    "Session command execution duration in seconds",
			Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0},
		},
		[]string{"command_type"},
	)

	// Host metrics
	HostsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "swoops_hosts_total",
			Help: "Number of registered hosts by status",
		},
		[]string{"status"},
	)

	// Database metrics
	DatabaseQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "swoops_database_queries_total",
			Help: "Total number of database queries by operation",
		},
		[]string{"operation"},
	)

	DatabaseQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "swoops_database_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
		},
		[]string{"operation"},
	)

	// WebSocket metrics
	WebSocketConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "swoops_websocket_connections_active",
			Help: "Number of currently active WebSocket connections",
		},
	)

	WebSocketMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "swoops_websocket_messages_total",
			Help: "Total number of WebSocket messages by direction",
		},
		[]string{"direction"}, // sent or received
	)
)

// Handler returns the Prometheus metrics HTTP handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// HTTPMiddleware wraps an HTTP handler to collect metrics
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Normalize path to prevent unbounded cardinality
		path := normalizePath(r.URL.Path)

		// Wrap the ResponseWriter to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
		HTTPRequestsTotal.WithLabelValues(r.Method, path, http.StatusText(rw.statusCode)).Inc()
	})
}

// normalizePath replaces resource IDs with placeholders to prevent unbounded cardinality
func normalizePath(path string) string {
	// Replace UUIDs and IDs in common patterns
	// /api/v1/hosts/{id} -> /api/v1/hosts/:id
	// /api/v1/sessions/{id}/output -> /api/v1/sessions/:id/output
	normalized := path

	// Match patterns like /hosts/abc123def456 or /sessions/19ca7780069066bd6acbf50
	patterns := []struct {
		pattern string
		replace string
	}{
		{`/hosts/[a-f0-9-]+`, `/hosts/:id`},
		{`/sessions/[a-f0-9-]+`, `/sessions/:id`},
		{`/reviews/[a-f0-9-]+`, `/reviews/:id`},
		{`/tasks/[a-f0-9-]+`, `/tasks/:id`},
		{`/messages/[a-f0-9-]+`, `/messages/:id`},
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p.pattern)
		normalized = re.ReplaceAllString(normalized, p.replace)
	}

	return normalized
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker interface (required for WebSocket)
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Flush implements http.Flusher interface
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Push implements http.Pusher interface (HTTP/2 server push)
func (rw *responseWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := rw.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}
