package domain

import "errors"

var (
	ErrRequiredField      = errors.New("required field missing")
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidReference   = errors.New("invalid reference")
	ErrCrossTenant        = errors.New("cross-tenant relationship")
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("conflict")
	ErrInvalidTransition  = errors.New("invalid mission transition")
	ErrMissionNotApproved = errors.New("mission is not approved")
	ErrMissionOutOfWindow = errors.New("mission is outside its validity window")
	ErrMandateNotUsable   = errors.New("mandate does not authorize")
	ErrCannotDelegate     = errors.New("mandate cannot delegate")
	ErrAmplification      = errors.New("privilege amplification")
	ErrMalformedAuthority = errors.New("malformed authority")
	ErrBrokenChain        = errors.New("delegation chain broken")
)
