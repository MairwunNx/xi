package tracing

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	operationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ximanager_operation_duration_seconds",
			Help:    "Duration of operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
		},
		[]string{"operation"},
	)
)

func init() {
	prometheus.MustRegister(operationDuration)
}

func ProfilePoint(log *Logger, msg, opname string, fields ...any) func() {
	start := time.Now()
	return func() {
		elapsed := time.Since(start)
		elapsedSeconds := elapsed.Seconds()		
		operationDuration.WithLabelValues(opname).Observe(elapsedSeconds)
		log.D(msg, append(fields, "elapsed_ms", elapsed.Milliseconds())...)
	}
}