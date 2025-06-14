package podstartup

import (
	"context"
	"errors"

	"github.com/Azure/cluster-health-monitor/pkg/checker"
	"github.com/Azure/cluster-health-monitor/pkg/config"
)

func Register() {
	checker.RegisterChecker(config.CheckTypeDNS, BuildPodStartupChecker)
}

type PodStartupChecker struct {
	name      string
	namespace string
	podName   string
}

func BuildPodStartupChecker(config *config.CheckerConfig) (checker.Checker, error) {
	return &PodStartupChecker{
		name: config.Name,
	}, nil
}

func (c *PodStartupChecker) Name() string {
	return c.name
}

func (c *PodStartupChecker) Run(ctx context.Context) error {
	return errors.New("PodStartupChecker not implemented yet")
}
