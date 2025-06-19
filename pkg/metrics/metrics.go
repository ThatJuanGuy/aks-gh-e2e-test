package metrics

import (
	"github.com/Azure/cluster-health-monitor/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// checkerResultCounter is a Prometheus counter that tracks the results of checker runs.
	checkerResultCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cluster_health_monitor_checker_result_total",
			Help: "Total number of checker runs, labeled by status and code",
		},
		[]string{"checker_type", "checker_name", "status", "error_code"},
	)
)

// RecordCheckerResult increments the result counter for a specific checker run.
// If err is not nil, it records a run error (unknown status).
// If result is not nil, it records the status from the result.
func RecordCheckerResult(checkerType, checkerName string, result *types.Result, err error) {
	// If there's an error, record as unknown.
	if err != nil {
		checkerResultCounter.WithLabelValues(checkerType, checkerName, unknownStatus, unknownCode).Inc()
		return
	}

	// Record based on result status.
	var status string
	var errorCode string
	switch result.Status {
	case types.StatusHealthy:
		status = healthyStatus
		errorCode = healthyCode
	case types.StatusUnhealthy:
		status = unhealthyStatus
		errorCode = result.Detail.Code
	}

	checkerResultCounter.WithLabelValues(checkerType, checkerName, status, errorCode).Inc()
}
