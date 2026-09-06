package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MandateSchemaVersion is the logical mandate schema version persisted in Phase 1.
// It is not a portable wire format.
const MandateSchemaVersion = "1"

// MandateState is a stored mandate status. Issuance and revocation engines are later phases.
type MandateState string

const (
	MandateActive     MandateState = "active"
	MandateRevoked    MandateState = "revoked"
	MandateExpired    MandateState = "expired"
	MandateSuperseded MandateState = "superseded"
)

// Mandate is the logical portable-agent-mandate model from specification §14.
// Persistence of these fields is not a signed interchange document.
type Mandate struct {
	MandateID              uuid.UUID
	Version                string
	OrganizationID         uuid.UUID
	PrincipalBindingID     uuid.UUID
	AgentBindingID         uuid.UUID
	MissionID              uuid.UUID
	AuthoritySourceID      uuid.UUID
	ParentMandateID        *uuid.UUID
	Scope                  json.RawMessage
	CommunicationAuthority json.RawMessage
	ExecutionAuthority     json.RawMessage
	Constraints            json.RawMessage
	Budget                 json.RawMessage
	NotBefore              time.Time
	Expiry                 time.Time
	IssuedAt               time.Time
	DelegationDepth        int
	ApprovalRequirements   json.RawMessage
	EvidenceRequirements   json.RawMessage
	IssuerBindingID        uuid.UUID
	State                  MandateState
}

type MandateParams struct {
	MandateID              uuid.UUID
	Version                string
	OrganizationID         uuid.UUID
	PrincipalBindingID     uuid.UUID
	AgentBindingID         uuid.UUID
	MissionID              uuid.UUID
	AuthoritySourceID      uuid.UUID
	ParentMandateID        *uuid.UUID
	Scope                  json.RawMessage
	CommunicationAuthority json.RawMessage
	ExecutionAuthority     json.RawMessage
	Constraints            json.RawMessage
	Budget                 json.RawMessage
	NotBefore              time.Time
	Expiry                 time.Time
	IssuedAt               time.Time
	DelegationDepth        int
	ApprovalRequirements   json.RawMessage
	EvidenceRequirements   json.RawMessage
	IssuerBindingID        uuid.UUID
	State                  MandateState
}

type MandateRelations struct {
	Principal IdentityBinding
	Agent     IdentityBinding
	Issuer    IdentityBinding
	Mission   Mission
	Source    AuthoritySource
	Parent    *Mandate
}

func NewMandate(p MandateParams) (Mandate, error) {
	id := p.MandateID
	if id == uuid.Nil {
		generated, err := newID()
		if err != nil {
			return Mandate{}, err
		}
		id = generated
	}
	version := p.Version
	if version == "" {
		version = MandateSchemaVersion
	}
	var parent *uuid.UUID
	if p.ParentMandateID != nil {
		copied := *p.ParentMandateID
		parent = &copied
	}
	m := Mandate{
		MandateID:              id,
		Version:                version,
		OrganizationID:         p.OrganizationID,
		PrincipalBindingID:     p.PrincipalBindingID,
		AgentBindingID:         p.AgentBindingID,
		MissionID:              p.MissionID,
		AuthoritySourceID:      p.AuthoritySourceID,
		ParentMandateID:        parent,
		Scope:                  cloneJSON(p.Scope),
		CommunicationAuthority: cloneJSON(p.CommunicationAuthority),
		ExecutionAuthority:     cloneJSON(p.ExecutionAuthority),
		Constraints:            cloneJSON(p.Constraints),
		Budget:                 cloneJSON(p.Budget),
		NotBefore:              normalizeTime(p.NotBefore),
		Expiry:                 normalizeTime(p.Expiry),
		IssuedAt:               normalizeTime(p.IssuedAt),
		DelegationDepth:        p.DelegationDepth,
		ApprovalRequirements:   cloneJSON(p.ApprovalRequirements),
		EvidenceRequirements:   cloneJSON(p.EvidenceRequirements),
		IssuerBindingID:        p.IssuerBindingID,
		State:                  p.State,
	}
	if err := m.Validate(); err != nil {
		return Mandate{}, err
	}
	return m, nil
}

func NewMandateWithRelations(p MandateParams, rel MandateRelations) (Mandate, error) {
	if err := rel.Validate(p.OrganizationID, p); err != nil {
		return Mandate{}, err
	}
	return NewMandate(p)
}

func (m Mandate) Validate() error {
	if err := requireID("mandate_id", m.MandateID); err != nil {
		return err
	}
	if err := requireNonEmpty("version", m.Version); err != nil {
		return err
	}
	if err := requireID("organization_id", m.OrganizationID); err != nil {
		return err
	}
	if err := requireID("principal", m.PrincipalBindingID); err != nil {
		return err
	}
	if err := requireID("agent", m.AgentBindingID); err != nil {
		return err
	}
	if err := requireID("mission", m.MissionID); err != nil {
		return err
	}
	if err := requireID("authority_source", m.AuthoritySourceID); err != nil {
		return err
	}
	if err := requireID("issuer", m.IssuerBindingID); err != nil {
		return err
	}
	if err := requireJSON("scope", m.Scope); err != nil {
		return err
	}
	if err := requireJSON("communication_authority", m.CommunicationAuthority); err != nil {
		return err
	}
	if err := requireJSON("execution_authority", m.ExecutionAuthority); err != nil {
		return err
	}
	if err := requireJSON("constraints", m.Constraints); err != nil {
		return err
	}
	if err := requireJSON("budget", m.Budget); err != nil {
		return err
	}
	if err := requireJSON("approval_requirements", m.ApprovalRequirements); err != nil {
		return err
	}
	if err := requireJSON("evidence_requirements", m.EvidenceRequirements); err != nil {
		return err
	}
	if err := requireTime("not_before", m.NotBefore); err != nil {
		return err
	}
	if err := requireTime("expiry", m.Expiry); err != nil {
		return err
	}
	if err := requireTime("issued_at", m.IssuedAt); err != nil {
		return err
	}
	if m.Expiry.Before(m.NotBefore) {
		return fmt.Errorf("%w: expiry is before not_before", ErrInvalidInput)
	}
	if m.DelegationDepth < 0 {
		return fmt.Errorf("%w: delegation_depth must be non-negative", ErrInvalidInput)
	}
	if m.ParentMandateID != nil {
		if *m.ParentMandateID == uuid.Nil {
			return fmt.Errorf("%w: parent_mandate_id", ErrInvalidReference)
		}
		if *m.ParentMandateID == m.MandateID {
			return fmt.Errorf("%w: parent_mandate_id cannot be self", ErrInvalidReference)
		}
	}
	if !validMandateState(m.State) {
		return fmt.Errorf("%w: state", ErrInvalidInput)
	}
	return nil
}

func (r MandateRelations) Validate(orgID uuid.UUID, p MandateParams) error {
	if err := SameOrganization(
		orgID,
		r.Principal.OrganizationID,
		r.Agent.OrganizationID,
		r.Issuer.OrganizationID,
		r.Mission.OrganizationID,
		r.Source.OrganizationID,
	); err != nil {
		return err
	}
	if r.Principal.BindingID != p.PrincipalBindingID {
		return fmt.Errorf("%w: principal", ErrInvalidReference)
	}
	if r.Principal.Kind != KindPrincipal {
		return fmt.Errorf("%w: principal must have kind principal", ErrInvalidReference)
	}
	if r.Agent.BindingID != p.AgentBindingID {
		return fmt.Errorf("%w: agent", ErrInvalidReference)
	}
	if r.Agent.Kind != KindAgent {
		return fmt.Errorf("%w: agent must have kind agent", ErrInvalidReference)
	}
	if r.Issuer.BindingID != p.IssuerBindingID {
		return fmt.Errorf("%w: issuer", ErrInvalidReference)
	}
	if r.Issuer.Kind != KindIssuer {
		return fmt.Errorf("%w: issuer must have kind issuer", ErrInvalidReference)
	}
	if r.Mission.MissionID != p.MissionID {
		return fmt.Errorf("%w: mission", ErrInvalidReference)
	}
	if r.Source.SourceID != p.AuthoritySourceID {
		return fmt.Errorf("%w: authority_source", ErrInvalidReference)
	}
	if r.Parent != nil {
		if p.ParentMandateID == nil || r.Parent.MandateID != *p.ParentMandateID {
			return fmt.Errorf("%w: parent_mandate_id", ErrInvalidReference)
		}
		if err := SameOrganization(orgID, r.Parent.OrganizationID); err != nil {
			return err
		}
		if err := r.Parent.Validate(); err != nil {
			return err
		}
	}
	if err := r.Principal.Validate(); err != nil {
		return err
	}
	if err := r.Agent.Validate(); err != nil {
		return err
	}
	if err := r.Issuer.Validate(); err != nil {
		return err
	}
	if err := r.Mission.Validate(); err != nil {
		return err
	}
	return r.Source.Validate()
}

func validMandateState(s MandateState) bool {
	switch s {
	case MandateActive, MandateRevoked, MandateExpired, MandateSuperseded:
		return true
	default:
		return false
	}
}
