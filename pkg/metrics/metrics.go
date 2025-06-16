package metrics

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	healthyStatus   = "healthy"
	unhealthyStatus = "unhealthy"
	unknownStatus   = "unknown"

	healthyCode = "healthy"
	unknownCode = "unknown"
)

// Metrics holds Prometheus collectors and exposes them via HTTP.
type Metrics struct {
	registry      *prometheus.Registry
	resultCounter *prometheus.CounterVec
	addr          string
	server        *http.Server
}

// NewMetrics creates a new Metrics instance with a custom registry and listen address.
func NewMetrics(addr string) (*Metrics, error) {
	reg := prometheus.NewRegistry()
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cluster_health_monitor_checker_result_total",
			Help: "Total number of checker runs, labeled by status and code",
		},
		[]string{"checker_name", "checker_type", "status", "error_code"},
	)
	if err := reg.Register(counter); err != nil {
		log.Printf("Failed to register checker counter: %v", err)
		return nil, err
	}
	return &Metrics{
		registry:      reg,
		addr:          addr,
		resultCounter: counter,
	}, nil
}

// Run starts the HTTP server to expose Prometheus metrics.
func (m *Metrics) Run() error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))
	m.server = &http.Server{
		Addr:    m.addr,
		Handler: mux,
	}
	log.Printf("Starting Prometheus metrics server at %s/metrics", m.addr)
	return m.server.ListenAndServe()
}

// Shutdown gracefully stops the metrics HTTP server.
func (m *Metrics) Shutdown() error {
	if m.server != nil {
		return m.server.Close()
	}
	return nil
}

func (m *Metrics) IncHealth(chkType, chkName string) {
	m.resultCounter.WithLabelValues(chkType, chkName, healthyStatus, healthyCode).Inc()
}

func (m *Metrics) IncUnhealth(chkType, chkName, code string) {
	m.resultCounter.WithLabelValues(chkType, chkName, unhealthyStatus, code).Inc()
}

func (m *Metrics) IncUnknown(chkType, chkName string) {
	m.resultCounter.WithLabelValues(chkType, chkName, unknownStatus, unknownCode).Inc()
}
