// Package dnscheck provides a checker for DNS.
package dnscheck

import (
	"context"
	"fmt"

	"github.com/Azure/cluster-health-monitor/pkg/config"
)

// DNSChecker implements the Checker interface for DNS checks.
type DNSChecker struct {
	name   string
	config *config.DNSConfig
}

// BuildDNSChecker creates a new DNSChecker instance.
func BuildDNSChecker(name string, config *config.DNSConfig) (*DNSChecker, error) {
	if err := config.ValidateDNSConfig(); err != nil {
		return nil, err
	}

	return &DNSChecker{
		name:   name,
		config: config,
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
