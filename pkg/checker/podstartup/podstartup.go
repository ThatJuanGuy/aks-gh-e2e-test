package podstartup

import (
	"context"
	"errors"

	"github.com/Azure/cluster-health-monitor/pkg/checker"
	"github.com/Azure/cluster-health-monitor/pkg/config"
	"github.com/Azure/cluster-health-monitor/pkg/types"
)

func Register() {
	checker.RegisterChecker(config.CheckTypePodStartup, BuildPodStartupChecker)
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

func (c *PodStartupChecker) Run(ctx context.Context) (*types.Result, error) {
	return nil, errors.New("PodStartupChecker not implemented yet")
}
