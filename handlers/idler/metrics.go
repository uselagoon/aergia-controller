package idler

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	serviceIdleEvents = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aergia_idling_events",
		Help: "The total number of service idling events that aergia has processed to idle environments",
	})
	cliIdleEvents = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aergia_cli_idling_events",
		Help: "The total number of cli idling events that aergia has processed to idle environments",
	})
)
