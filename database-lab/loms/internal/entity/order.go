package entity

import (
	"errors"
)

var (
	ErrMissing      = errors.New("not found")
	ErrInvalid      = errors.New("invalid input")
	ErrConflict     = errors.New("idempotency conflict")
	ErrDuplicateSKU = errors.New("SKU already exists")
	ErrState        = errors.New("operation not allowed")
	ErrStock        = errors.New("insufficient stock")
	ErrDeclined     = errors.New("payment declined")
	ErrUnavailable  = errors.New("dependency unavailable")
	ErrDependency   = errors.New("dependency failed")
)
