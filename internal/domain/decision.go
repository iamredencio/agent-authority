package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ActType is one proposed act evaluated by the Decision API.
type ActType string

const (
	ActCommunicate ActType = "communicate"
	ActExecute     ActType = "execute"
	ActDelegate    ActType = "delegate"
	ActSpend       ActType = "spend"
	ActControl     ActType = "control"
)

// DecisionResult is the independent authorization outcome for one act.
type DecisionResult string

const (
	ResultAllow           DecisionResult = "allow"
	ResultDeny            DecisionResult = "deny"
	ResultPendingApproval DecisionResult = "pending_approval"
)

// Stable reason codes returned on every stored decision.
const (
	ReasonAllowed               = "ALLOWED"
	ReasonDeniedDefault         = "DENIED"
	ReasonMalformedRequest      = "MALFORMED_REQUEST"
	ReasonOrganizationMissing   = "ORGANIZATION_NOT_FOUND"
	ReasonIdentityNotFound      = "IDENTITY_NOT_FOUND"
	ReasonIdentityDisabled      = "IDENTITY_DISABLED"
	ReasonAuthnNotAuthz         = "AUTHN_NOT_AUTHZ"
	ReasonMandateNotFound       = "MANDATE_NOT_FOUND"
	ReasonMandateNotUsable      = "MANDATE_NOT_USABLE"
	ReasonMissionNotApproved    = "MISSION_NOT_APPROVED"
	ReasonMissionOutOfWindow    = "MISSION_OUT_OF_WINDOW"
	ReasonBrokenChain           = "BROKEN_CHAIN"
	ReasonCrossTenant           = "CROSS_TENANT"
	ReasonActorMismatch         = "ACTOR_MISMATCH"
	ReasonUnknownDestination    = "UNKNOWN_DESTINATION"
	ReasonUnknownAction         = "UNKNOWN_ACTION"
	ReasonConstraintViolated    = "CONSTRAINT_VIOLATED"
	ReasonBudgetInsufficient    = "BUDGET_INSUFFICIENT"
	ReasonCannotDelegate        = "CANNOT_DELEGATE"
	ReasonApprovalRequired      = "APPROVAL_REQUIRED"
	ReasonPolicyDeny            = "POLICY_DENY"
	ReasonPolicyError           = "POLICY_ERROR"
	ReasonPersistenceError      = "PERSISTENCE_ERROR"
	ReasonRevocationUnavailable = "REVOCATION_UNAVAILABLE"
)

// Reason is a stable machine code plus human text.
type Reason struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Decision is an evidence-ready authorization record (architecture §6.7).
// EvidenceRecordID is a Phase 4 seam and stays unset in Phase 3.
type Decision struct {
	DecisionID       uuid.UUID
	OrganizationID   uuid.UUID
	MandateID        *uuid.UUID
	MissionID        *uuid.UUID
	ActorBindingID   *uuid.UUID
	ActType          ActType
	Act              json.RawMessage
	Result           DecisionResult
	Reasons          []Reason
	ValidUntil       *time.Time
	DecidedAt        time.Time
	EvidenceRecordID *uuid.UUID
}

type DecisionParams struct {
	DecisionID       uuid.UUID
	OrganizationID   uuid.UUID
	MandateID        *uuid.UUID
	MissionID        *uuid.UUID
	ActorBindingID   *uuid.UUID
	ActType          ActType
	Act              json.RawMessage
	Result           DecisionResult
	Reasons          []Reason
	ValidUntil       *time.Time
	DecidedAt        time.Time
	EvidenceRecordID *uuid.UUID
}

func NewDecision(p DecisionParams) (Decision, error) {
	id := p.DecisionID
	if id == uuid.Nil {
		generated, err := newID()
		if err != nil {
			return Decision{}, err
		}
		id = generated
	}
	d := Decision{
		DecisionID:       id,
		OrganizationID:   p.OrganizationID,
		MandateID:        copyID(p.MandateID),
		MissionID:        copyID(p.MissionID),
		ActorBindingID:   copyID(p.ActorBindingID),
		ActType:          p.ActType,
		Act:              cloneJSON(p.Act),
		Result:           p.Result,
		Reasons:          cloneReasons(p.Reasons),
		ValidUntil:       copyTime(p.ValidUntil),
		DecidedAt:        normalizeTime(p.DecidedAt),
		EvidenceRecordID: copyID(p.EvidenceRecordID),
	}
	if err := d.Validate(); err != nil {
		return Decision{}, err
	}
	return d, nil
}

func (d Decision) Validate() error {
	if err := requireID("decision_id", d.DecisionID); err != nil {
		return err
	}
	if err := requireID("organization_id", d.OrganizationID); err != nil {
		return err
	}
	if !validActType(d.ActType) {
		return fmt.Errorf("%w: act_type", ErrInvalidInput)
	}
	if err := requireJSON("act", d.Act); err != nil {
		return err
	}
	if !validDecisionResult(d.Result) {
		return fmt.Errorf("%w: result", ErrInvalidInput)
	}
	if len(d.Reasons) == 0 {
		return fmt.Errorf("%w: reasons", ErrRequiredField)
	}
	for i, r := range d.Reasons {
		if err := requireNonEmpty("reason.code", r.Code); err != nil {
			return fmt.Errorf("%w: reasons[%d]", err, i)
		}
		if err := requireNonEmpty("reason.message", r.Message); err != nil {
			return fmt.Errorf("%w: reasons[%d]", err, i)
		}
	}
	if err := requireTime("decided_at", d.DecidedAt); err != nil {
		return err
	}
	if d.ValidUntil != nil && d.ValidUntil.Before(d.DecidedAt) {
		return fmt.Errorf("%w: valid_until precedes decided_at", ErrInvalidInput)
	}
	if d.MandateID != nil && *d.MandateID == uuid.Nil {
		return fmt.Errorf("%w: mandate_id", ErrInvalidReference)
	}
	if d.MissionID != nil && *d.MissionID == uuid.Nil {
		return fmt.Errorf("%w: mission_id", ErrInvalidReference)
	}
	if d.ActorBindingID != nil && *d.ActorBindingID == uuid.Nil {
		return fmt.Errorf("%w: actor_binding_id", ErrInvalidReference)
	}
	if d.EvidenceRecordID != nil && *d.EvidenceRecordID == uuid.Nil {
		return fmt.Errorf("%w: evidence_record_id", ErrInvalidReference)
	}
	return nil
}

func validActType(t ActType) bool {
	switch t {
	case ActCommunicate, ActExecute, ActDelegate, ActSpend, ActControl:
		return true
	default:
		return false
	}
}

func validDecisionResult(r DecisionResult) bool {
	switch r {
	case ResultAllow, ResultDeny, ResultPendingApproval:
		return true
	default:
		return false
	}
}

func copyID(id *uuid.UUID) *uuid.UUID {
	if id == nil {
		return nil
	}
	copied := *id
	return &copied
}

func copyTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	copied := normalizeTime(*t)
	return &copied
}

func cloneReasons(in []Reason) []Reason {
	if in == nil {
		return nil
	}
	out := make([]Reason, len(in))
	copy(out, in)
	return out
}
