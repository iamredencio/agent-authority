package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamredencio/agent-authority/internal/domain"
)

func (s *Store) IssueOriginatingMandate(ctx context.Context, p domain.MandateParams, now time.Time) (domain.Mandate, error) {
	rel, err := s.mandateRelations(ctx, p)
	if err != nil {
		return domain.Mandate{}, err
	}
	issued, err := domain.IssueOriginatingMandate(p, rel, now)
	if err != nil {
		return domain.Mandate{}, err
	}
	if err := s.CreateMandate(ctx, issued); err != nil {
		return domain.Mandate{}, err
	}
	return issued, nil
}

func (s *Store) IssueChildMandate(ctx context.Context, p domain.MandateParams, now time.Time) (domain.Mandate, error) {
	if p.ParentMandateID == nil {
		return domain.Mandate{}, fmt.Errorf("%w: parent mandate", domain.ErrRequiredField)
	}
	rel, err := s.mandateRelations(ctx, p)
	if err != nil {
		return domain.Mandate{}, err
	}
	if rel.Parent == nil {
		return domain.Mandate{}, fmt.Errorf("%w: parent mandate", domain.ErrBrokenChain)
	}
	parentMission, err := s.GetMission(ctx, p.OrganizationID, rel.Parent.MissionID)
	if err != nil {
		return domain.Mandate{}, fmt.Errorf("%w: parent mission: %v", domain.ErrBrokenChain, err)
	}
	var childMissionParent *domain.Mission
	if rel.Mission.ParentMissionID != nil {
		got, err := s.GetMission(ctx, p.OrganizationID, *rel.Mission.ParentMissionID)
		if err != nil {
			return domain.Mandate{}, fmt.Errorf("%w: child mission parent: %v", domain.ErrBrokenChain, err)
		}
		childMissionParent = &got
	}
	issued, err := domain.IssueChildMandate(p, rel, parentMission, childMissionParent, now)
	if err != nil {
		return domain.Mandate{}, err
	}
	if err := s.CreateMandate(ctx, issued); err != nil {
		return domain.Mandate{}, err
	}
	return issued, nil
}

func (s *Store) ReconstructDelegationChain(ctx context.Context, organizationID, mandateID uuid.UUID) ([]domain.Mandate, error) {
	return domain.ReconstructDelegationChain(organizationID, mandateID, func(orgID, id uuid.UUID) (domain.Mandate, error) {
		return s.GetMandate(ctx, orgID, id)
	})
}

func (s *Store) VerifyDelegationChain(ctx context.Context, organizationID, mandateID uuid.UUID, now time.Time) ([]domain.Mandate, error) {
	return domain.VerifyDelegationChain(organizationID, mandateID,
		func(orgID, id uuid.UUID) (domain.Mandate, error) {
			return s.GetMandate(ctx, orgID, id)
		},
		func(orgID, id uuid.UUID) (domain.Mission, error) {
			return s.GetMission(ctx, orgID, id)
		},
		now,
	)
}

func (s *Store) mandateRelations(ctx context.Context, p domain.MandateParams) (domain.MandateRelations, error) {
	principal, err := s.GetIdentityBinding(ctx, p.OrganizationID, p.PrincipalBindingID)
	if err != nil {
		return domain.MandateRelations{}, err
	}
	agent, err := s.GetIdentityBinding(ctx, p.OrganizationID, p.AgentBindingID)
	if err != nil {
		return domain.MandateRelations{}, err
	}
	issuer, err := s.GetIdentityBinding(ctx, p.OrganizationID, p.IssuerBindingID)
	if err != nil {
		return domain.MandateRelations{}, err
	}
	mission, err := s.GetMission(ctx, p.OrganizationID, p.MissionID)
	if err != nil {
		return domain.MandateRelations{}, err
	}
	source, err := s.GetAuthoritySource(ctx, p.OrganizationID, p.AuthoritySourceID)
	if err != nil {
		return domain.MandateRelations{}, err
	}
	rel := domain.MandateRelations{
		Principal: principal,
		Agent:     agent,
		Issuer:    issuer,
		Mission:   mission,
		Source:    source,
	}
	if p.ParentMandateID != nil {
		parent, err := s.GetMandate(ctx, p.OrganizationID, *p.ParentMandateID)
		if err != nil {
			return domain.MandateRelations{}, fmt.Errorf("%w: parent mandate: %v", domain.ErrBrokenChain, err)
		}
		rel.Parent = &parent
	}
	return rel, nil
}
