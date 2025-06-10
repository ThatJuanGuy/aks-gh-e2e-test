package dnscheck

import (
	"reflect"
	"testing"

	"github.com/Azure/cluster-health-monitor/pkg/config"
)

func TestBuildDNSChecker(t *testing.T) {
	testCases := []struct {
		name            string
		checkerName     string
		dnsConfig       *config.DNSConfig
		expectedError   bool
		expectedChecker *DNSChecker
	}{
		{
			name:        "Valid config",
			checkerName: "test-dns-checker",
			dnsConfig: &config.DNSConfig{
				Domain: "example.com",
			},
			expectedError: false,
			expectedChecker: &DNSChecker{
				name: "test-dns-checker",
				config: &config.DNSConfig{
					Domain: "example.com",
				},
			},
		},
		{
			name:          "Missing DNSConfig",
			checkerName:   "test-dns-checker",
			dnsConfig:     nil,
			expectedError: true,
		},
		{
			name:        "Empty Domain",
			checkerName: "test-dns-checker",
			dnsConfig: &config.DNSConfig{
				Domain: "",
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			checker, err := BuildDNSChecker(tc.checkerName, tc.dnsConfig)

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
