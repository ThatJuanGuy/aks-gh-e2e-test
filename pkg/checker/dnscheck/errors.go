package dnscheck

import "errors"

const (
	// This is the error code of the DNSChecker's result.
	ServiceNotReady = "service_not_ready"
	PodsNotReady    = "pods_not_ready"
	ServiceTimeout  = "service_timeout"
	PodTimeout      = "pod_timeout"
	ServiceError    = "service_error"
	PodError        = "pod_error"
)

// This is the error list used by the DNSChecker.
var (
	errServiceNotReady = errors.New("service not ready")
	errPodsNotReady    = errors.New("pods not ready")
)
