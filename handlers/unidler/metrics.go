package unidler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func (h *Unidler) setMetrics(r *http.Request, start time.Time) {
	duration := time.Now().Sub(start).Seconds()

	proto := strconv.Itoa(r.ProtoMajor)
	proto = fmt.Sprintf("%s.%s", proto, strconv.Itoa(r.ProtoMinor))

	h.RequestCount.WithLabelValues(proto).Inc()
	h.RequestDuration.WithLabelValues(proto).Observe(duration)
}

var (
	allowedRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aergia_allowed_requests",
		Help: "The total number of requests that aergia has allowed",
	})
	verificationRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aergia_verification_requests",
		Help: "The total number of verificiation requests that aergia has recieved",
	})
	verificationRequired = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aergia_verification_required_requests",
		Help: "The total number of verificiation required requests that aergia has received",
	})
	blockedRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aergia_blocked_by_block_list",
		Help: "The total number of requests that aergia has blocked by an allow or block list rule",
	})
	noNamespaceRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aergia_no_namespace",
		Help: "The total number of requests that aergia has received where no namespace was found",
	})
	unidleEvents = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aergia_unidling_events",
		Help: "The total number of events that aergia has processed to unidle an environments",
	})
)
