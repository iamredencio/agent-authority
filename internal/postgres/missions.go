package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/iamredencio/agent-authority/internal/domain"
)

func (s *Store) CreateMission(ctx context.Context, m domain.Mission) error {
	if err := m.Validate(); err != nil {
		return err
	}
	principalOrg, kind, err := s.bindingOrganization(ctx, m.PrincipalBindingID)
	if err != nil {
		return err
	}
	if err := rejectCrossTenant(m.OrganizationID, principalOrg, "principal_binding_id"); err != nil {
		return err
	}
	if kind != domain.KindPrincipal {
		return domain.ErrInvalidReference
	}
	sourceOrg, err := s.sourceOrganization(ctx, m.AuthoritySourceID)
	if err != nil {
		return err
	}
	if err := rejectCrossTenant(m.OrganizationID, sourceOrg, "authority_source_id"); err != nil {
		return err
	}
	if m.ParentMissionID != nil {
		parentOrg, err := s.missionOrganization(ctx, *m.ParentMissionID)
		if err != nil {
			return err
		}
		if err := rejectCrossTenant(m.OrganizationID, parentOrg, "parent_mission_id"); err != nil {
			return err
		}
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO missions (
			mission_id, organization_id, principal_binding_id, purpose, intended_outcome,
			parent_mission_id, not_before, expiry, state, authority_source_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, m.MissionID, m.OrganizationID, m.PrincipalBindingID, m.Purpose, m.IntendedOutcome,
		m.ParentMissionID, m.NotBefore, m.Expiry, m.State, m.AuthoritySourceID)
	return mapError(err)
}

func (s *Store) GetMission(ctx context.Context, organizationID, missionID uuid.UUID) (domain.Mission, error) {
	var m domain.Mission
	err := s.pool.QueryRow(ctx, `
		SELECT mission_id, organization_id, principal_binding_id, purpose, intended_outcome,
			parent_mission_id, not_before, expiry, state, authority_source_id
		FROM missions
		WHERE organization_id = $1 AND mission_id = $2
	`, organizationID, missionID).Scan(
		&m.MissionID,
		&m.OrganizationID,
		&m.PrincipalBindingID,
		&m.Purpose,
		&m.IntendedOutcome,
		&m.ParentMissionID,
		&m.NotBefore,
		&m.Expiry,
		&m.State,
		&m.AuthoritySourceID,
	)
	if err != nil {
		return domain.Mission{}, mapError(err)
	}
	m.NotBefore = m.NotBefore.UTC()
	m.Expiry = m.Expiry.UTC()
	return m, nil
}
