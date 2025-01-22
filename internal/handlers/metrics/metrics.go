package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	namespace = "default_http_backend"
	subsystem = "http"
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(RequestCount,
		RequestDuration,
		AllowedRequests,
		VerificationRequests,
		VerificationRequired,
		BlockedRequests,
		NoNamespaceRequests,
		UnidleEvents,
		ServiceIdleEvents,
		CliIdleEvents,
	)
}

var (
	RequestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "request_count_total",
		Help:      "Counter of HTTP requests made.",
	}, []string{"proto"})

	RequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "request_duration_milliseconds",
		Help:      "Histogram of the time (in milliseconds) each request took.",
		Buckets:   append([]float64{.001, .003}, prometheus.DefBuckets...),
	}, []string{"proto"})
	AllowedRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aergia_allowed_requests",
		Help: "The total number of requests that aergia has allowed",
	})
	VerificationRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aergia_verification_requests",
		Help: "The total number of verificiation requests that aergia has recieved",
	})
	VerificationRequired = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aergia_verification_required_requests",
		Help: "The total number of verificiation required requests that aergia has received",
	})
	BlockedRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aergia_blocked_by_block_list",
		Help: "The total number of requests that aergia has blocked by an allow or block list rule",
	})
	NoNamespaceRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aergia_no_namespace",
		Help: "The total number of requests that aergia has received where no namespace was found",
	})
	UnidleEvents = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aergia_unidling_events",
		Help: "The total number of events that aergia has processed to unidle an environments",
	})
	ServiceIdleEvents = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aergia_idling_events",
		Help: "The total number of service idling events that aergia has processed to idle environments",
	})
	CliIdleEvents = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aergia_cli_idling_events",
		Help: "The total number of cli idling events that aergia has processed to idle environments",
	})
)
