package checker

import (
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
		t.Errorf("expected name 'example', got %q", dc.Name())
	}
}

func TestDuplicateCheckerName(t *testing.T) {
	yamlData := []byte(`
checkers:
- name: foo
  interval: 10s
  type: dns 
- name: foo
  interval: 10s
  type: dns
`)

	_, err := BuildCheckersFromConfig(yamlData)
	if err == nil {
		t.Fatal("expected error for duplicate checker name, got nil")
	}
	if got, want := err.Error(), "duplicate checker name: \"foo\""; got != want {
		t.Fatalf("unexpected error: got %q, want %q", got, want)
	}
}
