package dnscheck

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/Azure/cluster-health-monitor/pkg/config"
)

func TestBuildDNSChecker(t *testing.T) {
	for _, tc := range []struct {
		name        string
		checkerName string
		dnsConfig   *config.DNSConfig
		validateRes func(g *WithT, checker *DNSChecker, err error)
	}{
		{
			name:        "Valid config",
			checkerName: "test-dns-checker",
			dnsConfig: &config.DNSConfig{
				Domain: "example.com",
			},
			validateRes: func(g *WithT, checker *DNSChecker, err error) {
				g.Expect(checker).To(Equal(
					&DNSChecker{
						name: "test-dns-checker",
						config: &config.DNSConfig{
							Domain: "example.com",
						},
					}))
				g.Expect(err).NotTo(HaveOccurred())
			},
		},
		{
			name:        "Empty Checker Name",
			checkerName: "",
			dnsConfig: &config.DNSConfig{
				Domain: "example.com",
			},
			validateRes: func(g *WithT, checker *DNSChecker, err error) {
				g.Expect(checker).To(BeNil())
				g.Expect(err).To(HaveOccurred())
			},
		},
		{
			name:        "Missing DNSConfig",
			checkerName: "test-dns-checker",
			dnsConfig:   nil,
			validateRes: func(g *WithT, checker *DNSChecker, err error) {
				g.Expect(checker).To(BeNil())
				g.Expect(err).To(HaveOccurred())
			},
		},
		{
			name:        "Empty Domain",
			checkerName: "test-dns-checker",
			dnsConfig: &config.DNSConfig{
				Domain: "",
			},
			validateRes: func(g *WithT, checker *DNSChecker, err error) {
				g.Expect(checker).To(BeNil())
				g.Expect(err).To(HaveOccurred())
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			checker, err := BuildDNSChecker(tc.checkerName, tc.dnsConfig)
			tc.validateRes(g, checker, err)
		})
	}
}
