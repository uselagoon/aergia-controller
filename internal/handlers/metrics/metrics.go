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
		UnidleDuration,
		UnidleResults,
		UnidlesInFlight,
		IdleEvaluations,
		PrometheusQueryDuration,
		PrometheusQueryErrors,
		PrometheusQueryWarnings,
		IdlerLastRun,
	)
}

const (
	UnidleResultSuccess      = "success"
	UnidleResultPartialError = "partial_error"
	UnidleResultWaitTimeout  = "wait_timeout"
	UnidleResultError        = "error"

	IdleTypeService = "service"
	IdleTypeCLI     = "cli"

	OutcomeIdled               = "idled"
	OutcomeDryRun              = "dry_run"
	OutcomeSkippedRunningBuild = "skipped_running_build"
	OutcomeSkippedNotOldEnough = "skipped_not_old_enough"
	OutcomeSkippedHasHits      = "skipped_has_hits"
	OutcomeErrorList           = "error_list"
	OutcomeErrorPrometheus     = "error_prometheus"
	OutcomeErrorIngressPatch   = "error_ingress_patch"
	OutcomeErrorScale          = "error_scale"

	PrometheusQueryNginx   = "nginx"
	PrometheusQueryTraefik = "traefik"
)

var (
	UnidleDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "aergia_unidle_duration_seconds",
		Help:    "Time taken to unidle an environment, including waiting for pods to run",
		Buckets: []float64{.5, 1, 2, 5, 10, 20, 30, 45, 60, 90, 120},
	}, []string{"result"})
	UnidleResults = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "aergia_unidle_results_total",
		Help: "The total number of unidle operations by result",
	}, []string{"result"})
	UnidlesInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "aergia_unidles_in_flight",
		Help: "The number of unidle operations currently running",
	})
	IdleEvaluations = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "aergia_idle_evaluations_total",
		Help: "The total number of environment idle evaluations by outcome",
	}, []string{"type", "outcome"})
	PrometheusQueryDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "aergia_prometheus_query_duration_seconds",
		Help:    "Time taken by hit-check queries against prometheus",
		Buckets: []float64{.05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"query"})
	PrometheusQueryErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "aergia_prometheus_query_errors_total",
		Help: "The total number of failed hit-check queries against prometheus",
	}, []string{"query"})
	PrometheusQueryWarnings = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "aergia_prometheus_query_warnings_total",
		Help: "The total number of warnings returned by hit-check queries against prometheus",
	}, []string{"query"})
	IdlerLastRun = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "aergia_idler_last_run_timestamp_seconds",
		Help: "Unix timestamp of the last completed idler pass",
	}, []string{"type"})

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
