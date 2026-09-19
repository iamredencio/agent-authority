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

func (s *Store) GetIdentityBindingBySubject(ctx context.Context, organizationID uuid.UUID, provider domain.IdentityProvider, subject string) (domain.IdentityBinding, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT binding_id, organization_id, kind, provider, subject, display_name, attestation_meta, status
		FROM identity_bindings
		WHERE organization_id = $1 AND provider = $2 AND subject = $3
	`, organizationID, provider, subject)
	if err != nil {
		return domain.IdentityBinding{}, mapError(err)
	}
	defer rows.Close()

	var found []domain.IdentityBinding
	for rows.Next() {
		b, err := scanIdentityBinding(rows)
		if err != nil {
			return domain.IdentityBinding{}, err
		}
		found = append(found, b)
	}
	if err := rows.Err(); err != nil {
		return domain.IdentityBinding{}, mapError(err)
	}
	if len(found) == 0 {
		return domain.IdentityBinding{}, domain.ErrNotFound
	}
	if len(found) > 1 {
		return domain.IdentityBinding{}, domain.ErrConflict
	}
	return found[0], nil
}

func scanIdentityBinding(row interface {
	Scan(dest ...any) error
}) (domain.IdentityBinding, error) {
	var b domain.IdentityBinding
	var display *string
	var meta []byte
	if err := row.Scan(
		&b.BindingID,
		&b.OrganizationID,
		&b.Kind,
		&b.Provider,
		&b.Subject,
		&display,
		&meta,
		&b.Status,
	); err != nil {
		return domain.IdentityBinding{}, mapError(err)
	}
	b.DisplayName = derefString(display)
	if meta != nil {
		b.AttestationMeta = json.RawMessage(meta)
	}
	return b, nil
}

func (s *Store) GetIdentityBinding(ctx context.Context, organizationID, bindingID uuid.UUID) (domain.IdentityBinding, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT binding_id, organization_id, kind, provider, subject, display_name, attestation_meta, status
		FROM identity_bindings
		WHERE organization_id = $1 AND binding_id = $2
	`, organizationID, bindingID)
	return scanIdentityBinding(row)
}
