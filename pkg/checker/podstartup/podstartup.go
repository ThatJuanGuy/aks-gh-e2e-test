package podstartup

import "github.com/Azure/cluster-health-monitor/pkg/config"

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
