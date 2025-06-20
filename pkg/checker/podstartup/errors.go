package podstartup

import "errors"

const (
	// This is the error code of the PodStartupCheckers's result.
	errCodePodCreationError           = "pod_creation_error"
	errCodePodCreationTimeout         = "pod_creation_timeout"
	errCodePodStartupDurationExceeded = "pod_startup_duration_exceeded"
)

// This is the error list used by the PodStartupChecker.
var (
	errPodHasNoRunningContainer = errors.New("pod has no running container")
)
