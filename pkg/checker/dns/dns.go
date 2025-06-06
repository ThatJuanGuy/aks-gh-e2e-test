package dns

import (
	"fmt"

	"github.com/Azure/cluster-health-monitor/pkg/config"
)

type DNSChecker struct {
	name   string
	Domain string
}

func BuildDNSChecker(name string, profile *config.DNSProfile) (*DNSChecker, error) {
	// TODO: Validate the name and profile
	return &DNSChecker{
		name:   name,
		Domain: profile.Domain,
	}, nil
}

func (c DNSChecker) Name() string {
	return c.name
}

func (c DNSChecker) Run() error {
	// TODO: Implement the DNS checking logic here
	return fmt.Errorf("DNSChecker not implemented yet")
}
