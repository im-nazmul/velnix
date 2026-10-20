package domain

import "fmt"

type Code string

const (
	Invalid     Code = "invalid_input"
	Unavailable Code = "unavailable"
	Denied      Code = "denied"
	Conflict    Code = "conflict"
	Failure     Code = "failure"
	Canceled    Code = "canceled"
)

type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }
func E(code Code, format string, args ...any) error {
	return &Error{code, fmt.Sprintf(format, args...)}
}
func ExitCode(code Code) int {
	switch code {
	case Invalid:
		return 2
	case Unavailable:
		return 3
	case Denied:
		return 4
	case Conflict:
		return 5
	case Canceled:
		return 130
	default:
		return 1
	}
}
