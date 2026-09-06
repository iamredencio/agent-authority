package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

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
		parent, err := s.GetMission(ctx, m.OrganizationID, *m.ParentMissionID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return fmt.Errorf("%w: parent_mission_id", domain.ErrInvalidReference)
			}
			return err
		}
		if err := rejectCrossTenant(m.OrganizationID, parent.OrganizationID, "parent_mission_id"); err != nil {
			return err
		}
		if m.NotBefore.Before(parent.NotBefore) || m.Expiry.After(parent.Expiry) {
			return fmt.Errorf("%w: sub-mission window exceeds parent", domain.ErrInvalidInput)
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

func (s *Store) UpdateMission(ctx context.Context, m domain.Mission) error {
	if err := m.Validate(); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE missions
		SET principal_binding_id = $3,
			purpose = $4,
			intended_outcome = $5,
			parent_mission_id = $6,
			not_before = $7,
			expiry = $8,
			state = $9,
			authority_source_id = $10
		WHERE organization_id = $1 AND mission_id = $2
	`, m.OrganizationID, m.MissionID, m.PrincipalBindingID, m.Purpose, m.IntendedOutcome,
		m.ParentMissionID, m.NotBefore, m.Expiry, m.State, m.AuthoritySourceID)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) TransitionMission(ctx context.Context, organizationID, missionID uuid.UUID, to domain.MissionState, now time.Time) (domain.Mission, error) {
	m, err := s.GetMission(ctx, organizationID, missionID)
	if err != nil {
		return domain.Mission{}, err
	}
	var parent *domain.Mission
	if m.ParentMissionID != nil {
		got, err := s.GetMission(ctx, organizationID, *m.ParentMissionID)
		if err != nil {
			return domain.Mission{}, fmt.Errorf("%w: parent mission: %v", domain.ErrBrokenChain, err)
		}
		parent = &got
	}
	next, err := domain.TransitionMission(m, to, now, parent)
	if err != nil {
		return domain.Mission{}, err
	}
	if err := s.UpdateMission(ctx, next); err != nil {
		return domain.Mission{}, err
	}
	return next, nil
}
