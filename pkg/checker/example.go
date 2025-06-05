package checker

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
)

type ExampleChecker struct {
	name     string
	Interval int
	Timeout  int
}

func BuildExampleChecker(name string, spec map[string]any) (*ExampleChecker, error) {
	if name == "" {
		return &ExampleChecker{}, nil
	}
	checker := &ExampleChecker{}
	if err := mapstructure.Decode(spec, checker); err != nil {
		return nil, err
	}
	checker.name = name
	return checker, nil
}

func (c ExampleChecker) Name() string {
	return c.name
}

func (c ExampleChecker) Run() error {
	fmt.Println("Running ExampleChecker:", c.name)
	fmt.Printf("Interval: %d, Timeout: %d\n", c.Interval, c.Timeout)
	return nil
}
