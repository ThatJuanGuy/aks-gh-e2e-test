// TODO: Move this package into pkg/checker once the circular dependency issue is resolved.
package types

type Status string

const (
	// StatusHealthy indicates the checker passed
	StatusHealthy Status = "healthy"
	// StatusUnhealthy indicates the checker failed
	StatusUnhealthy Status = "unhealthy"
)

// Result represents the result of a health check.
type Result struct {
	Status Status
	Detail Detail
}

// Detail provides additional information about the health check result if it is not healthy.
type Detail struct {
	Code    string
	Message string
}

// Healthy is a helper function to create a healthy Result.
func Healthy() Result {
	return Result{
		Status: StatusHealthy,
	}
}

// unhealthy is a helper function to create an unhealthy Result with a specific code and message.
func Unhealthy(code, message string) Result {
	return Result{
		Status: StatusUnhealthy,
		Detail: Detail{
			Code:    code,
			Message: message,
		},
	}
}
