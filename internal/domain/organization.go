package domain

import (
	"time"

	"github.com/google/uuid"
)

// Organization is the tenancy boundary. Every other aggregate is scoped to it.
type Organization struct {
	OrganizationID uuid.UUID
	Name           string
	CreatedAt      time.Time
}

type OrganizationParams struct {
	OrganizationID uuid.UUID
	Name           string
	CreatedAt      time.Time
}

func NewOrganization(p OrganizationParams) (Organization, error) {
	id := p.OrganizationID
	if id == uuid.Nil {
		generated, err := newID()
		if err != nil {
			return Organization{}, err
		}
		id = generated
	}
	if err := requireNonEmpty("name", p.Name); err != nil {
		return Organization{}, err
	}
	createdAt := p.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	org := Organization{
		OrganizationID: id,
		Name:           p.Name,
		CreatedAt:      normalizeTime(createdAt),
	}
	if err := org.Validate(); err != nil {
		return Organization{}, err
	}
	return org, nil
}

func (o Organization) Validate() error {
	if err := requireID("organization_id", o.OrganizationID); err != nil {
		return err
	}
	if err := requireNonEmpty("name", o.Name); err != nil {
		return err
	}
	return requireTime("created_at", o.CreatedAt)
}
