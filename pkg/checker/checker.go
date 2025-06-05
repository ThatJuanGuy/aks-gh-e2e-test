package checker

import (
	"fmt"

	yaml "gopkg.in/yaml.v3"
)

type Checker interface {
	Name() string
	Run() error
}

type CheckerConfig struct {
	Name string         `yaml:"name"`
	Type string         `yaml:"type"`
	Spec map[string]any `yaml:"spec"`
}

type Config struct {
	Checkers []CheckerConfig `yaml:"checker"`
}

type checkerBuilder func(name string, spec map[string]any) (Checker, error)

var registry = make(map[string]checkerBuilder)

func RegisterChecker(typ string, builder checkerBuilder) {
	registry[typ] = builder
}

func init() {
	RegisterChecker("example", nil)
}

func BuildCheckersFromConfig(config []byte) ([]Checker, error) {
	var root Config
	if err := yaml.Unmarshal(config, &root); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml: %w", err)
	}

	var checkers []Checker
	for _, cfg := range root.Checkers {
		if cfg.Name == "" || cfg.Spec == nil {
			return nil, fmt.Errorf("checker entry missing 'name' or 'spec'")
		}
		chk, err := buildChecker(cfg.Name, cfg.Type, cfg.Spec)
		if err != nil {
			return nil, fmt.Errorf("failed to build checker %q: %w", cfg.Name, err)
		}
		checkers = append(checkers, chk)
	}
	return checkers, nil
}

// buildChecker creates a checker by registry identity (name) and passes the spec.
func buildChecker(name, typ string, spec map[string]any) (Checker, error) {
	builder, exists := registry[typ]
	if !exists {
		return nil, fmt.Errorf("checker type %q not registered", typ)
	}
	return builder(name, spec)
}
