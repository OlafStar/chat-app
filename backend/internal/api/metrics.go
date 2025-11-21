package api

import (
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"chat-app-backend/internal/queue"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metrics bundles Prometheus collectors that are shared across the HTTP servers.
type metrics struct {
	requests   *prometheus.CounterVec
	duration   *prometheus.HistogramVec
	inFlight   prometheus.Gauge
	queueDepth prometheus.GaugeFunc
}

func newMetrics(reg prometheus.Registerer, listenAddr string, q *queue.RequestQueueManager) *metrics {
	labels := prometheus.Labels{"listen_addr": listenAddr}

	m := &metrics{
		requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "chat_app_http_requests_total",
				Help:        "Total count of HTTP requests received.",
				ConstLabels: labels,
			},
			[]string{"method", "path", "status"},
		),
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "chat_app_http_request_duration_seconds",
				Help:        "Histogram of request durations.",
				Buckets:     prometheus.DefBuckets,
				ConstLabels: labels,
			},
			[]string{"method", "path", "status"},
		),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "chat_app_http_inflight_requests",
			Help:        "Number of requests currently being handled.",
			ConstLabels: labels,
		}),
	}

	reg.MustRegister(m.requests, m.duration, m.inFlight)

	if q != nil {
		m.queueDepth = prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name:        "chat_app_request_queue_depth",
				Help:        "Jobs waiting in the request queue channel.",
				ConstLabels: labels,
			},
			func() float64 {
				return float64(len(q.JobQueue))
			},
		)
		reg.MustRegister(m.queueDepth)
	}

	return m
}

// metricsHandler exposes /metrics using the shared registry.
func (m *metrics) metricsHandler() http.Handler {
	return promhttp.Handler()
}

// instrument wraps the provided handler with Prometheus counters and histograms.
func (m *metrics) instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.inFlight.Inc()
		defer m.inFlight.Dec()

		normalizedPath := sanitizePath(r.URL.Path)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		start := time.Now()
		next.ServeHTTP(rec, r)
		elapsed := time.Since(start).Seconds()

		statusLabel := strconv.Itoa(rec.status)
		labels := []string{r.Method, normalizedPath, statusLabel}

		m.requests.WithLabelValues(labels...).Inc()
		m.duration.WithLabelValues(labels...).Observe(elapsed)
	})
}

// sanitizePath reduces cardinality by collapsing long or parameterised paths.
func sanitizePath(p string) string {
	clean := path.Clean(p)
	if clean == "" || clean == "." {
		return "/"
	}

	segments := strings.Split(clean, "/")
	// The first element is empty for absolute paths; keep up to three actual segments.
	out := segments
	if len(segments) > 4 {
		out = append(segments[:4], "...")
	}

	res := strings.Join(out, "/")
	if !strings.HasPrefix(res, "/") {
		res = "/" + res
	}

	return res
}

// statusRecorder captures the final status code for metrics purposes.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(status int) {
	sr.status = status
	sr.ResponseWriter.WriteHeader(status)
}
