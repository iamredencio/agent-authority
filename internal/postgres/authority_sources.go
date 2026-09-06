package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/iamredencio/agent-authority/internal/domain"
)

func (s *Store) CreateAuthoritySource(ctx context.Context, src domain.AuthoritySource) error {
	if err := src.Validate(); err != nil {
		return err
	}
	stewardOrg, _, err := s.bindingOrganization(ctx, src.StewardBindingID)
	if err != nil {
		return err
	}
	if err := rejectCrossTenant(src.OrganizationID, stewardOrg, "steward_binding_id"); err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO authority_sources (
			source_id, organization_id, type, steward_binding_id, external_ref, evidence_pointer, summary
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, src.SourceID, src.OrganizationID, src.Type, src.StewardBindingID, src.ExternalRef, emptyToNil(src.EvidencePointer), src.Summary)
	return mapError(err)
}

func (s *Store) GetAuthoritySource(ctx context.Context, organizationID, sourceID uuid.UUID) (domain.AuthoritySource, error) {
	var src domain.AuthoritySource
	var pointer *string
	err := s.pool.QueryRow(ctx, `
		SELECT source_id, organization_id, type, steward_binding_id, external_ref, evidence_pointer, summary
		FROM authority_sources
		WHERE organization_id = $1 AND source_id = $2
	`, organizationID, sourceID).Scan(
		&src.SourceID,
		&src.OrganizationID,
		&src.Type,
		&src.StewardBindingID,
		&src.ExternalRef,
		&pointer,
		&src.Summary,
	)
	if err != nil {
		return domain.AuthoritySource{}, mapError(err)
	}
	src.EvidencePointer = derefString(pointer)
	return src, nil
}
