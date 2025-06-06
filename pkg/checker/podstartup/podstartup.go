package podstartup

import (
	"errors"

	"github.com/Azure/cluster-health-monitor/pkg/config"
)

type PodStartupChecker struct {
	name      string
	namespace string
	podName   string
}

func BuildPodStartupChecker(name string, profile *config.PodStartupProfile) (*PodStartupChecker, error) {
	return &PodStartupChecker{
		name:      name,
		namespace: profile.Namespace,
		podName:   profile.PodName,
	}, nil
}

func (c *PodStartupChecker) Name() string {
	return c.name
}

func (c *PodStartupChecker) Run() error {
	return errors.New("PodStartupChecker not implemented yet")
}
