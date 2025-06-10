package dnscheck

import (
	"reflect"
	"testing"
	"time"

	"github.com/Azure/cluster-health-monitor/pkg/config"
)

func TestBuildDNSChecker(t *testing.T) {
	testCases := []struct {
		name            string
		config          config.CheckerConfig
		expectedError   bool
		expectedChecker *DNSChecker
	}{
		{
			name: "Valid config",
			config: config.CheckerConfig{
				Name:     "test-dns-checker",
				Type:     config.CheckTypeDNS,
				Interval: 10 * time.Second,
				Timeout:  5 * time.Second,
				DNSConfig: &config.DNSConfig{
					Domain: "example.com",
				},
			},
			expectedError: false,
			expectedChecker: &DNSChecker{
				name:     "test-dns-checker",
				interval: 10 * time.Second,
				timeout:  5 * time.Second,
				domain:   "example.com",
			},
		},
		{
			name: "Missing DNSConfig",
			config: config.CheckerConfig{
				Name:     "test-dns-checker",
				Type:     config.CheckTypeDNS,
				Interval: 10 * time.Second,
				Timeout:  5 * time.Second,
			},
			expectedError: true,
		},
		{
			name: "Empty Domain",
			config: config.CheckerConfig{
				Name:     "test-dns-checker",
				Type:     config.CheckTypeDNS,
				Interval: 10 * time.Second,
				Timeout:  5 * time.Second,
				DNSConfig: &config.DNSConfig{
					Domain: "",
				},
			},
			expectedError: true,
		},
		{
			name: "Empty Name",
			config: config.CheckerConfig{
				Name:     "",
				Type:     config.CheckTypeDNS,
				Interval: 10 * time.Second,
				Timeout:  5 * time.Second,
				DNSConfig: &config.DNSConfig{
					Domain: "example.com",
				},
			},
			expectedError: true,
		},
		{
			name: "Invalid Interval",
			config: config.CheckerConfig{
				Name:     "test-dns-checker",
				Type:     config.CheckTypeDNS,
				Interval: -10 * time.Second,
				Timeout:  5 * time.Second,
				DNSConfig: &config.DNSConfig{
					Domain: "example.com",
				},
			},
			expectedError: true,
		},
		{
			name: "Invalid Timeout",
			config: config.CheckerConfig{
				Name:     "test-dns-checker",
				Type:     config.CheckTypeDNS,
				Interval: 10 * time.Second,
				Timeout:  -5 * time.Second,
				DNSConfig: &config.DNSConfig{
					Domain: "example.com",
				},
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			checker, err := BuildDNSChecker(tc.config)

			if tc.expectedError && err == nil {
				t.Errorf("expected error but got nil")
				return
			}

			if !tc.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !tc.expectedError {
				if checker == nil {
					t.Errorf("expected checker but got nil")
					return
				}

				if !reflect.DeepEqual(checker, tc.expectedChecker) {
					t.Errorf("checkers don't match:\nexpected: %+v\ngot: %+v", tc.expectedChecker, checker)
				}
			}
		})
	}
}
