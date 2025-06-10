package checker

import (
	"strings"
	"testing"

	"github.com/Azure/cluster-health-monitor/pkg/checker/dnscheck"
)

func TestRegisterAndBuildExampleChecker(t *testing.T) {
	// Register the ExampleChecker builder
	yamlData := []byte(`
checkers:
- name: dns 
  type: dns 
  interval: 10s
  dnsConfig:
    domain: example.com
`)

	checkers, err := BuildCheckersFromConfig(yamlData)
	if err != nil {
		t.Fatalf("failed to build checkers: %v", err)
	}
	if len(checkers) != 1 {
		t.Fatalf("expected 1 checker, got %d", len(checkers))
	}

	dc, ok := checkers[0].(*dnscheck.DNSChecker)
	if !ok {
		t.Fatalf("checker is not of type *dnscheck.DNSChecker")
	}
	if dc.Name() != "dns" {
		t.Errorf("expected name 'dns', got %q", dc.Name())
	}
}

func TestValidationInBuildCheckersFromConfig(t *testing.T) {
	testCases := []struct {
		name             string
		yaml             string
		expectedError    bool
		expectedErrorMsg string
	}{
		{
			name: "Valid DNS Checker",
			yaml: `
checkers:
- name: valid-checker
  type: dns
  interval: 10s
  dnsConfig:
    domain: example.com
`,
			expectedError: false,
		},
		{
			name: "Duplicate Name",
			yaml: `
checkers:
- name: duplicate-name
  type: dns
  interval: 10s
  dnsConfig:
    domain: example.com
- name: duplicate-name
  type: dns
  interval: 10s
  dnsConfig:
    domain: example.com
`,
			expectedError:    true,
			expectedErrorMsg: "duplicate checker name: \"duplicate-name\"",
		},
		{
			name: "Empty Name",
			yaml: `
checkers:
- name: "" 
  type: dns
  interval: 10s
`,
			expectedError:    true,
			expectedErrorMsg: "missing 'name'",
		},
		{
			name: "Missing Type",
			yaml: `
checkers:
- name: test-checker
  interval: 10s
`,
			expectedError:    true,
			expectedErrorMsg: "missing 'type'",
		},
		{
			name: "Invalid Interval",
			yaml: `
checkers:
- name: test-checker
  type: dns
  interval: -10s
`,
			expectedError:    true,
			expectedErrorMsg: "invalid 'interval'",
		},
		{
			name: "Invalid Timeout",
			yaml: `
checkers:
- name: test-checker
  type: dns
  interval: 10s
  timeout: -5s
`,
			expectedError:    true,
			expectedErrorMsg: "invalid 'timeout'",
		},
		{
			name: "Unknown Type",
			yaml: `
checkers:
- name: unknown-checker
  type: unknown
`,
			expectedError:    true,
			expectedErrorMsg: "unrecognized checker type: \"unknown\"",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BuildCheckersFromConfig([]byte(tc.yaml))

			if tc.expectedError && err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.expectedErrorMsg)
				return
			}

			if tc.expectedError && !strings.Contains(err.Error(), tc.expectedErrorMsg) {
				t.Errorf("expected error containing %q, got: %v", tc.expectedErrorMsg, err)
			}

			if !tc.expectedError && err != nil {
				t.Fatalf("unexpected error: %v", err)
				return
			}
		})
	}
}
