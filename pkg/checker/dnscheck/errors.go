package dnscheck

import "errors"

const (
	// This is the error code of the DNSChecker's result.
	errCodeServiceNotReady = "serviceNotReady"
	errCodePodsNotReady    = "podsNotReady"
	errCodeServiceTimeout  = "serviceTimeout"
	errCodePodTimeout      = "podTimeout"
	errCodeServiceError    = "serviceError"
	errCodePodError        = "podError"
	errCodeLocalDNSTimeout = "localDnsTimeout"
	errCodeLocalDNSError   = "localDnsError"
)

// This is the error list used by the DNSChecker.
var (
	errServiceNotReady = errors.New("service not ready")
	errPodsNotReady    = errors.New("pods not ready")
)
