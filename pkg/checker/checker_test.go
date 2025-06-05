package checker

import (
	"testing"

	"github.com/Azure/cluster-health-monitor/pkg/checker/example"
)

func TestRegisterAndBuildExampleChecker(t *testing.T) {
	// Register the ExampleChecker builder
	registerChecker("example", func(name string, spec map[string]any) (Checker, error) {
		return example.BuildExampleChecker(name, spec)
	})

	yamlData := []byte(`
checkers:
- name: example
  type: example
  spec:
    interval: 15
    timeout: 30
`)

	checkers, err := BuildCheckersFromConfig(yamlData)
	if err != nil {
		t.Fatalf("failed to build checkers: %v", err)
	}
	if len(checkers) != 1 {
		t.Fatalf("expected 1 checker, got %d", len(checkers))
	}

	ec, ok := checkers[0].(*example.ExampleChecker)
	if !ok {
		t.Fatalf("checker is not of type *ExampleChecker")
	}
	if ec.Name() != "example" {
		t.Errorf("expected name 'example', got %q", ec.Name())
	}
	if ec.Interval != 15 {
		t.Errorf("expected interval 15, got %d", ec.Interval)
	}
	if ec.Timeout != 30 {
		t.Errorf("expected timeout 30, got %d", ec.Timeout)
	}
}

func TestDuplicateCheckerName(t *testing.T) {
	yamlData := []byte(`
checkers:
- name: foo
  type: example
  spec:
    interval: 1
    timeout: 2
- name: foo
  type: example
  spec:
    interval: 3
    timeout: 4
`)

	_, err := BuildCheckersFromConfig(yamlData)
	if err == nil {
		t.Fatal("expected error for duplicate checker name, got nil")
	}
	if got, want := err.Error(), "duplicate checker name: \"foo\""; got != want {
		t.Fatalf("unexpected error: got %q, want %q", got, want)
	}
}
