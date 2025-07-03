package apiserver

const (
	// This is the error code of the APIServerChecker's result.
	errCodeConfigMapCreateError   = "configmap_create_error"
	errCodeConfigMapCreateTimeout = "configmap_create_timeout"
	errCodeConfigMapGetError      = "configmap_get_error"
	errCodeConfigMapGetTimeout    = "configmap_get_timeout"
	errCodeConfigMapDeleteError   = "configmap_delete_error"
	errCodeConfigMapDeleteTimeout = "configmap_delete_timeout"
)
