package domain

import "errors"

var (
	ErrRequiredField    = errors.New("required field missing")
	ErrInvalidInput     = errors.New("invalid input")
	ErrInvalidReference = errors.New("invalid reference")
	ErrCrossTenant      = errors.New("cross-tenant relationship")
	ErrNotFound         = errors.New("not found")
	ErrConflict         = errors.New("conflict")
)
