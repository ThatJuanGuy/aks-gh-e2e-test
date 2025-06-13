package checker

type UnhealthyError struct {
	Code    string
	Message string
}

func (e *UnhealthyError) Error() string {
	return e.Message
}

func NewUnhealthy(code string) *UnhealthyError {
	return &UnhealthyError{
		Code: code,
	}
}

func NewUnhealthyWithMessage(code, message string) *UnhealthyError {
	return &UnhealthyError{
		Code:    code,
		Message: message,
	}
}
