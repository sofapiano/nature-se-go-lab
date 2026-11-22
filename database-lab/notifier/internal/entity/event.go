package entity

import "errors"

var (
	ErrInvalid     = errors.New("invalid notification")
	ErrUnavailable = errors.New("telegram unavailable")
	ErrDependency  = errors.New("telegram request failed")
)
