package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	LogsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "log_ingestor_logs_processed_total",
		Help: "The total number of logs processed",
	}, []string{"level", "service", "status"})

	HttpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "log_ingestor_http_requests_total",
		Help: "Total number of HTTP requests processed",
	}, []string{"status", "method"})
)
