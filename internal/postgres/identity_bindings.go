package postgres

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/iamredencio/agent-authority/internal/domain"
)

func (s *Store) CreateIdentityBinding(ctx context.Context, b domain.IdentityBinding) error {
	if err := b.Validate(); err != nil {
		return err
	}
	if err := s.organizationExists(ctx, b.OrganizationID); err != nil {
		return err
	}
	var meta any
	if len(b.AttestationMeta) > 0 {
		meta = []byte(b.AttestationMeta)
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO identity_bindings (
			binding_id, organization_id, kind, provider, subject, display_name, attestation_meta, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, b.BindingID, b.OrganizationID, b.Kind, b.Provider, b.Subject, emptyToNil(b.DisplayName), meta, b.Status)
	return mapError(err)
}

func (s *Store) GetIdentityBinding(ctx context.Context, organizationID, bindingID uuid.UUID) (domain.IdentityBinding, error) {
	var b domain.IdentityBinding
	var display *string
	var meta []byte
	err := s.pool.QueryRow(ctx, `
		SELECT binding_id, organization_id, kind, provider, subject, display_name, attestation_meta, status
		FROM identity_bindings
		WHERE organization_id = $1 AND binding_id = $2
	`, organizationID, bindingID).Scan(
		&b.BindingID,
		&b.OrganizationID,
		&b.Kind,
		&b.Provider,
		&b.Subject,
		&display,
		&meta,
		&b.Status,
	)
	if err != nil {
		return domain.IdentityBinding{}, mapError(err)
	}
	b.DisplayName = derefString(display)
	if meta != nil {
		b.AttestationMeta = json.RawMessage(meta)
	}
	return b, nil
}
