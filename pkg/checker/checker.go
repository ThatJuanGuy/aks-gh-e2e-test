package checker

import (
	"fmt"

	yaml "gopkg.in/yaml.v3"

	"github.com/Azure/cluster-health-monitor/pkg/checker/dns"
	"github.com/Azure/cluster-health-monitor/pkg/checker/podstartup"
	"github.com/Azure/cluster-health-monitor/pkg/config"
)

type Checker interface {
	Name() string
	Run() error
}

type checkerBuilder func(name string, spec map[string]any) (Checker, error)

func BuildCheckersFromConfig(cfg []byte) ([]Checker, error) {
	var root config.Config
	if err := yaml.Unmarshal(cfg, &root); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml: %w", err)
	}

	nameSet := make(map[string]struct{})
	var checkers []Checker
	for _, cfg := range root.Checkers {
		if cfg.Name == "" {
			return nil, fmt.Errorf("checker entry missing 'name'")
		}
		if _, exists := nameSet[cfg.Name]; exists {
			return nil, fmt.Errorf("duplicate checker name: %q", cfg.Name)
		}
		nameSet[cfg.Name] = struct{}{}
		chk, err := buildChecker(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to build checker %q: %w", cfg.Name, err)
		}
		checkers = append(checkers, chk)
	}
	return checkers, nil
}

// buildChecker creates a checker by registry identity (name) and passes the spec.
func buildChecker(cfg config.CheckerConfig) (Checker, error) {
	switch cfg.Type {
	case config.CheckTypeDNS:
		if cfg.DNSProfile == nil {
			return nil, fmt.Errorf("DNSProfile is required for checker type %q", cfg.Type)
		}
		return dns.BuildDNSChecker(cfg.Name, cfg.DNSProfile)
	case config.CheckTypePodStartup:
		if cfg.PodStartupProfile == nil {
			return nil, fmt.Errorf("PodStartupProfile is required for checker type %q", cfg.Type)
		}
		return podstartup.BuildPodStartupChecker(cfg.Name, cfg.PodStartupProfile)
	default:
		return nil, fmt.Errorf("unrecognized checker type: %q", cfg.Type)
	}
}
