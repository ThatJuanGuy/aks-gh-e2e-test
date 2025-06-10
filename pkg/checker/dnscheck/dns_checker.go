// Package dnscheck provides a checker for DNS.
package dnscheck

import (
	"context"
	"fmt"

	"github.com/Azure/cluster-health-monitor/pkg/config"
)

type DNSChecker struct {
	name   string
	Domain string
}

func BuildDNSChecker(name string, config *config.DNSConfig) (*DNSChecker, error) {
	// TODO: Validate the name and config
	return &DNSChecker{
		name: name,
	}, nil
}

func (c DNSChecker) Name() string {
	return c.name
}

func (c DNSChecker) Run(ctx context.Context) error {
	// TODO: Implement the DNS checking logic here
	return fmt.Errorf("DNSChecker not implemented yet")
}
