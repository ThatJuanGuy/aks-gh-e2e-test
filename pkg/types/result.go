// TODO: Move this package into pkg/checker once the circular dependency issue is resolved.
package types

type Status string

const (
	// StatusHealthy indicates the checker passed
	StatusHealthy Status = "healthy"
	// StatusUnhealthy indicates the checker failed
	StatusUnhealthy Status = "unhealthy"
)

type Result struct {
	Status Status
	Detail Detail
}

type Detail struct {
	Code    string
	Message string
}

func Healthy() Result {
	return Result{
		Status: StatusHealthy,
	}
}

func Unhealthy(code, message string) Result {
	return Result{
		Status: StatusUnhealthy,
		Detail: Detail{
			Code:    code,
			Message: message,
		},
	}
}
