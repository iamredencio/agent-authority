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
	ErrUnknownDestination = errors.New("unknown destination")
	ErrUnknownAction      = errors.New("unknown action")
	ErrConstraintViolated = errors.New("constraint violated")
	ErrBudgetInsufficient = errors.New("budget insufficient")
	ErrApprovalRequired   = errors.New("approval required")
	ErrPolicyDenied       = errors.New("policy denied")
	ErrPolicyError        = errors.New("policy evaluation error")
	ErrActorMismatch      = errors.New("actor does not hold the mandate")
	ErrIdentityDisabled   = errors.New("identity binding is disabled")
)
