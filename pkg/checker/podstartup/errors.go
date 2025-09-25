package podstartup

const (
	// This is the error code of the PodStartupCheckers's result.
	errCodePodCreationError           = "PodCreationError"
	errCodePodCreationTimeout         = "PodCreationTimeout"
	errCodePodStartupDurationExceeded = "PodStartupDurationExceeded"
	errCodeRequestFailed              = "RequestFailed"
	errCodeRequestTimeout             = "RequestTimeout"
)
