// Package dnscheck provides a checker for DNS.
package dnscheck

import (
	"context"
	"fmt"

	"time"

	"github.com/Azure/cluster-health-monitor/pkg/config"
)

// DNSChecker implements the Checker interface for DNS checks.
type DNSChecker struct {
	name     string
	interval time.Duration
	timeout  time.Duration
	domain   string
}

// BuildDNSChecker creates a new DNSChecker instance.
func BuildDNSChecker(config config.CheckerConfig) (*DNSChecker, error) {
	if config.DNSConfig == nil {
		return nil, fmt.Errorf("dnsConfig is required for DNSChecker")
	}

	if config.DNSConfig.Domain == "" {
		return nil, fmt.Errorf("domain is required for DNSChecker")
	}

	return &DNSChecker{
		name:     config.Name,
		interval: config.Interval,
		timeout:  config.Timeout,
		domain:   config.DNSConfig.Domain,
	}, nil
}

func (c DNSChecker) Name() string {
	return c.name
}

func (c DNSChecker) Run(ctx context.Context) error {
	// TODO: Get the CoreDNS service IP and pod IPs.

	// TODO: Get LocalDNS IP.

	// TODO: Implement the DNS checking logic here
	return fmt.Errorf("DNSChecker not implemented yet")
}
