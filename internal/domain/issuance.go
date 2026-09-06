package domain

import (
	"fmt"
	"time"
)

// IssueOriginatingMandate creates an originating mandate against a mission that
// currently authorizes action. Persistence constructors still accept stored
// records without this check.
func IssueOriginatingMandate(p MandateParams, rel MandateRelations, now time.Time) (Mandate, error) {
	now = normalizeTime(now)
	if err := requireTime("now", now); err != nil {
		return Mandate{}, err
	}
	if p.ParentMandateID != nil {
		return Mandate{}, fmt.Errorf("%w: originating mandate cannot have a parent", ErrInvalidInput)
	}
	if rel.Parent != nil {
		return Mandate{}, fmt.Errorf("%w: originating mandate cannot have a parent", ErrInvalidInput)
	}
	if err := rel.Validate(p.OrganizationID, p); err != nil {
		return Mandate{}, err
	}
	if err := assertBindingsActive(rel); err != nil {
		return Mandate{}, err
	}
	if err := rel.Mission.AssertAuthorizes(now); err != nil {
		return Mandate{}, err
	}
	if p.AuthoritySourceID != rel.Mission.AuthoritySourceID {
		return Mandate{}, fmt.Errorf("%w: authority_source must match the mission source", ErrInvalidReference)
	}
	if err := assertMandateWindowInsideMission(p, rel.Mission); err != nil {
		return Mandate{}, err
	}
	if p.State == "" {
		p.State = MandateActive
	}
	if p.State != MandateActive {
		return Mandate{}, fmt.Errorf("%w: issued mandate must be active", ErrInvalidInput)
	}
	if p.IssuedAt.IsZero() {
		p.IssuedAt = now
	} else if normalizeTime(p.IssuedAt).After(now) {
		return Mandate{}, fmt.Errorf("%w: issued_at is in the future", ErrInvalidInput)
	}
	return NewMandateWithRelations(p, rel)
}

// IssueChildMandate creates a child mandate that is an attenuation of its parent.
func IssueChildMandate(p MandateParams, rel MandateRelations, parentMission Mission, childMissionParent *Mission, now time.Time) (Mandate, error) {
	now = normalizeTime(now)
	if err := requireTime("now", now); err != nil {
		return Mandate{}, err
	}
	if p.ParentMandateID == nil || rel.Parent == nil {
		return Mandate{}, fmt.Errorf("%w: parent mandate", ErrRequiredField)
	}
	if err := rel.Validate(p.OrganizationID, p); err != nil {
		return Mandate{}, err
	}
	if err := assertBindingsActive(rel); err != nil {
		return Mandate{}, err
	}
	if err := AssertMandateUsable(*rel.Parent, parentMission, now); err != nil {
		return Mandate{}, err
	}
	if p.State == "" {
		p.State = MandateActive
	}
	if p.State != MandateActive {
		return Mandate{}, fmt.Errorf("%w: issued mandate must be active", ErrInvalidInput)
	}
	if p.IssuedAt.IsZero() {
		p.IssuedAt = now
	} else if normalizeTime(p.IssuedAt).After(now) {
		return Mandate{}, fmt.Errorf("%w: issued_at is in the future", ErrInvalidInput)
	}
	if err := assertMandateWindowInsideMission(p, rel.Mission); err != nil {
		return Mandate{}, err
	}
	child, err := NewMandateWithRelations(p, rel)
	if err != nil {
		return Mandate{}, err
	}
	if err := AssertAttenuation(child, *rel.Parent, rel.Mission, parentMission, childMissionParent, now); err != nil {
		return Mandate{}, err
	}
	return child, nil
}

// AssertMandateUsable fails closed unless the mandate is active, in window,
// and bound to a mission that currently authorizes action.
func AssertMandateUsable(mandate Mandate, mission Mission, now time.Time) error {
	if err := mandate.Validate(); err != nil {
		return err
	}
	if err := SameOrganization(mandate.OrganizationID, mission.OrganizationID); err != nil {
		return err
	}
	if mandate.MissionID != mission.MissionID {
		return fmt.Errorf("%w: mission", ErrInvalidReference)
	}
	if mandate.State != MandateActive {
		return fmt.Errorf("%w: state=%s", ErrMandateNotUsable, mandate.State)
	}
	now = normalizeTime(now)
	if now.Before(mandate.NotBefore) || now.After(mandate.Expiry) {
		return fmt.Errorf("%w: mandate window", ErrMandateNotUsable)
	}
	return mission.AssertAuthorizes(now)
}

func assertMandateWindowInsideMission(p MandateParams, mission Mission) error {
	nb := normalizeTime(p.NotBefore)
	exp := normalizeTime(p.Expiry)
	if nb.Before(mission.NotBefore) {
		return fmt.Errorf("%w: mandate not_before precedes mission", ErrInvalidInput)
	}
	if exp.After(mission.Expiry) {
		return fmt.Errorf("%w: mandate expiry exceeds mission", ErrInvalidInput)
	}
	return nil
}

func assertBindingsActive(rel MandateRelations) error {
	if rel.Principal.Status != BindingActive {
		return fmt.Errorf("%w: principal binding is not active", ErrInvalidReference)
	}
	if rel.Agent.Status != BindingActive {
		return fmt.Errorf("%w: agent binding is not active", ErrInvalidReference)
	}
	if rel.Issuer.Status != BindingActive {
		return fmt.Errorf("%w: issuer binding is not active", ErrInvalidReference)
	}
	return nil
}
