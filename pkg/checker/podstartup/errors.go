package podstartup

const (
	// This is the error code of the PodStartupCheckers's result.
	errCodePodCreationError           = "podCreationError"
	errCodePodCreationTimeout         = "podCreationTimeout"
	errCodePodStartupDurationExceeded = "podStartupDurationExceeded"
	errCodeRequestFailed              = "requestFailed"
	errCodeRequestTimeout             = "requestTimeout"
)
