package dnscheck

import "errors"

const (
	// This is the error code of the DNSChecker's result.
	errCodeServiceNotReady = "ServiceNotReady"
	errCodePodsNotReady    = "PodsNotReady"
	errCodeServiceTimeout  = "ServiceTimeout"
	errCodePodTimeout      = "PodTimeout"
	errCodeServiceError    = "ServiceError"
	errCodePodError        = "PodError"
	errCodeLocalDNSTimeout = "LocalDNSTimeout"
	errCodeLocalDNSError   = "LocalDnsError"
)

// This is the error list used by the DNSChecker.
var (
	errServiceNotReady = errors.New("service not ready")
	errPodsNotReady    = errors.New("pods not ready")
)
