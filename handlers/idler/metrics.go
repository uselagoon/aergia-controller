package idler

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	idleEvents = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aergia_idling_events",
		Help: "The total number of events that aergia has processed to idle environments",
	})
)
