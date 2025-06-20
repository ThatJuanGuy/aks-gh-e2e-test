package podstartup

const (
	// This is the error code of the PodStartupCheckers's result.
	errCodePodCreationError                  = "pod_creation_error"
	errCodePodCreationTimeout                = "pod_creation_timeout"
	errCodeHealthyPodStartupDurationExceeded = "healthy_pod_startup_duration_exceeded"
)
