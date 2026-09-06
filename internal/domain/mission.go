package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MissionState is a stored mission status. Transition rules are Phase 2.
type MissionState string

const (
	MissionDraft           MissionState = "draft"
	MissionPendingApproval MissionState = "pending_approval"
	MissionApproved        MissionState = "approved"
	MissionSuspended       MissionState = "suspended"
	MissionEnded           MissionState = "ended"
	MissionExpired         MissionState = "expired"
)

// Mission is an approved (or recorded) statement of purpose that justifies authority.
type Mission struct {
	MissionID          uuid.UUID
	OrganizationID     uuid.UUID
	PrincipalBindingID uuid.UUID
	Purpose            string
	IntendedOutcome    string
	ParentMissionID    *uuid.UUID
	NotBefore          time.Time
	Expiry             time.Time
	State              MissionState
	AuthoritySourceID  uuid.UUID
}

type MissionParams struct {
	MissionID          uuid.UUID
	OrganizationID     uuid.UUID
	PrincipalBindingID uuid.UUID
	Purpose            string
	IntendedOutcome    string
	ParentMissionID    *uuid.UUID
	NotBefore          time.Time
	Expiry             time.Time
	State              MissionState
	AuthoritySourceID  uuid.UUID
}

type MissionRelations struct {
	Principal IdentityBinding
	Source    AuthoritySource
	Parent    *Mission
}

func NewMission(p MissionParams) (Mission, error) {
	id := p.MissionID
	if id == uuid.Nil {
		generated, err := newID()
		if err != nil {
			return Mission{}, err
		}
		id = generated
	}
	var parent *uuid.UUID
	if p.ParentMissionID != nil {
		copied := *p.ParentMissionID
		parent = &copied
	}
	m := Mission{
		MissionID:          id,
		OrganizationID:     p.OrganizationID,
		PrincipalBindingID: p.PrincipalBindingID,
		Purpose:            p.Purpose,
		IntendedOutcome:    p.IntendedOutcome,
		ParentMissionID:    parent,
		NotBefore:          normalizeTime(p.NotBefore),
		Expiry:             normalizeTime(p.Expiry),
		State:              p.State,
		AuthoritySourceID:  p.AuthoritySourceID,
	}
	if err := m.Validate(); err != nil {
		return Mission{}, err
	}
	return m, nil
}

func NewMissionWithRelations(p MissionParams, rel MissionRelations) (Mission, error) {
	if err := rel.Validate(p.OrganizationID, p); err != nil {
		return Mission{}, err
	}
	return NewMission(p)
}

func (m Mission) Validate() error {
	if err := requireID("mission_id", m.MissionID); err != nil {
		return err
	}
	if err := requireID("organization_id", m.OrganizationID); err != nil {
		return err
	}
	if err := requireID("principal_binding_id", m.PrincipalBindingID); err != nil {
		return err
	}
	if err := requireID("authority_source_id", m.AuthoritySourceID); err != nil {
		return err
	}
	if err := requireNonEmpty("purpose", m.Purpose); err != nil {
		return err
	}
	if err := requireNonEmpty("intended_outcome", m.IntendedOutcome); err != nil {
		return err
	}
	if err := requireTime("not_before", m.NotBefore); err != nil {
		return err
	}
	if err := requireTime("expiry", m.Expiry); err != nil {
		return err
	}
	if m.Expiry.Before(m.NotBefore) {
		return fmt.Errorf("%w: expiry is before not_before", ErrInvalidInput)
	}
	if m.ParentMissionID != nil {
		if *m.ParentMissionID == uuid.Nil {
			return fmt.Errorf("%w: parent_mission_id", ErrInvalidReference)
		}
		if *m.ParentMissionID == m.MissionID {
			return fmt.Errorf("%w: parent_mission_id cannot be self", ErrInvalidReference)
		}
	}
	if !validMissionState(m.State) {
		return fmt.Errorf("%w: state", ErrInvalidInput)
	}
	return nil
}

func (r MissionRelations) Validate(orgID uuid.UUID, p MissionParams) error {
	if err := SameOrganization(orgID, r.Principal.OrganizationID, r.Source.OrganizationID); err != nil {
		return err
	}
	if r.Principal.BindingID != p.PrincipalBindingID {
		return fmt.Errorf("%w: principal_binding_id", ErrInvalidReference)
	}
	if r.Principal.Kind != KindPrincipal {
		return fmt.Errorf("%w: principal must have kind principal", ErrInvalidReference)
	}
	if r.Source.SourceID != p.AuthoritySourceID {
		return fmt.Errorf("%w: authority_source_id", ErrInvalidReference)
	}
	if r.Parent != nil {
		if p.ParentMissionID == nil || r.Parent.MissionID != *p.ParentMissionID {
			return fmt.Errorf("%w: parent_mission_id", ErrInvalidReference)
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
	return r.Source.Validate()
}

func validMissionState(s MissionState) bool {
	switch s {
	case MissionDraft, MissionPendingApproval, MissionApproved, MissionSuspended, MissionEnded, MissionExpired:
		return true
	default:
		return false
	}
}
