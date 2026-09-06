package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamredencio/agent-authority/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Store, error) {
	if dsn == "" {
		return nil, fmt.Errorf("%w: postgres dsn", domain.ErrRequiredField)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503":
			return fmt.Errorf("%w: %s", domain.ErrInvalidReference, pgErr.Message)
		case "23505":
			return fmt.Errorf("%w: %s", domain.ErrConflict, pgErr.Message)
		case "23514":
			return fmt.Errorf("%w: %s", domain.ErrInvalidInput, pgErr.Message)
		}
	}
	return err
}

func emptyToNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *Store) organizationExists(ctx context.Context, orgID uuid.UUID) error {
	var found uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT organization_id FROM organizations WHERE organization_id = $1`, orgID).Scan(&found)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: organization_id", domain.ErrInvalidReference)
		}
		return mapError(err)
	}
	return nil
}

func (s *Store) bindingOrganization(ctx context.Context, bindingID uuid.UUID) (uuid.UUID, domain.BindingKind, error) {
	var orgID uuid.UUID
	var kind domain.BindingKind
	err := s.pool.QueryRow(ctx, `SELECT organization_id, kind FROM identity_bindings WHERE binding_id = $1`, bindingID).Scan(&orgID, &kind)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, "", fmt.Errorf("%w: identity binding", domain.ErrInvalidReference)
		}
		return uuid.Nil, "", mapError(err)
	}
	return orgID, kind, nil
}

func (s *Store) sourceOrganization(ctx context.Context, sourceID uuid.UUID) (uuid.UUID, error) {
	var orgID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT organization_id FROM authority_sources WHERE source_id = $1`, sourceID).Scan(&orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, fmt.Errorf("%w: authority source", domain.ErrInvalidReference)
		}
		return uuid.Nil, mapError(err)
	}
	return orgID, nil
}

func (s *Store) missionOrganization(ctx context.Context, missionID uuid.UUID) (uuid.UUID, error) {
	var orgID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT organization_id FROM missions WHERE mission_id = $1`, missionID).Scan(&orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, fmt.Errorf("%w: mission", domain.ErrInvalidReference)
		}
		return uuid.Nil, mapError(err)
	}
	return orgID, nil
}

func (s *Store) mandateOrganization(ctx context.Context, mandateID uuid.UUID) (uuid.UUID, error) {
	var orgID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT organization_id FROM mandates WHERE mandate_id = $1`, mandateID).Scan(&orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, fmt.Errorf("%w: parent mandate", domain.ErrInvalidReference)
		}
		return uuid.Nil, mapError(err)
	}
	return orgID, nil
}

func rejectCrossTenant(orgID, other uuid.UUID, field string) error {
	if err := domain.SameOrganization(orgID, other); err != nil {
		return fmt.Errorf("%w: %s", err, field)
	}
	return nil
}
