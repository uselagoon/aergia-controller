package unidler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func (h *Unidler) setMetrics(r *http.Request, start time.Time) {
	duration := time.Now().Sub(start).Seconds()

	proto := strconv.Itoa(r.ProtoMajor)
	proto = fmt.Sprintf("%s.%s", proto, strconv.Itoa(r.ProtoMinor))

	h.RequestCount.WithLabelValues(proto).Inc()
	h.RequestDuration.WithLabelValues(proto).Observe(duration)
}
