package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/iamredencio/agent-authority/internal/domain"
)

func (s *Store) CreateOrganization(ctx context.Context, org domain.Organization) error {
	if err := org.Validate(); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO organizations (organization_id, name, created_at)
		VALUES ($1, $2, $3)
	`, org.OrganizationID, org.Name, org.CreatedAt)
	return mapError(err)
}

func (s *Store) GetOrganization(ctx context.Context, organizationID uuid.UUID) (domain.Organization, error) {
	var org domain.Organization
	err := s.pool.QueryRow(ctx, `
		SELECT organization_id, name, created_at
		FROM organizations
		WHERE organization_id = $1
	`, organizationID).Scan(&org.OrganizationID, &org.Name, &org.CreatedAt)
	if err != nil {
		return domain.Organization{}, mapError(err)
	}
	org.CreatedAt = org.CreatedAt.UTC()
	return org, nil
}
